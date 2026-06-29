package memory

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// maxGossipMsgBytes caps a single decoded gossip message to bound memory use
// from a hostile or buggy peer.
const maxGossipMsgBytes = 8 << 20 // 8 MiB

// handshakeTimeout bounds the auth exchange so a silent peer cannot hold a
// connection (and an accept slot) open indefinitely.
const handshakeTimeout = 10 * time.Second

// gossipMsg is the wire format for peer-to-peer sync messages.
type gossipMsg struct {
	Type      string     // "set", "delete", "publish"
	Key       string     // for set/delete
	Value     any        // for set/publish
	ExpiresAt time.Time  // for set
	Clock     ClockValue // for set
	Topic     string     // for publish
}

// gossipTransport manages TCP connections to peer nodes and broadcasts
// delta messages (set/delete/publish) to all of them.
type gossipTransport struct {
	store    *Store
	bindAddr string
	peers    []string
	secret   string
	conns    map[string]*peerConn
	mu       sync.RWMutex
	listener net.Listener
	done     chan struct{}
	logger   *slog.Logger
}

// peerConn wraps a peer connection and serializes writes with a lock. Each
// gossip message is gob-encoded into its own length-framed blob and written
// atomically under the lock, so concurrent broadcasts can never interleave
// their bytes on the wire (the previous code created a fresh encoder per
// broadcast on a shared conn, corrupting the stream).
type peerConn struct {
	conn net.Conn
	mu   sync.Mutex
}

func (p *peerConn) send(msg gossipMsg) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return err
	}
	if buf.Len() > maxGossipMsgBytes {
		return fmt.Errorf("gossip: message too large: %d bytes", buf.Len())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return writeMsgFrame(p.conn, buf.Bytes())
}

// writeMsgFrame writes a 4-byte big-endian length prefix followed by b.
func writeMsgFrame(w io.Writer, b []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// readMsgFrame reads a 4-byte length-prefixed frame, rejecting oversized ones.
func readMsgFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint32(hdr[:]))
	if n > maxGossipMsgBytes {
		return nil, fmt.Errorf("gossip: frame too large: %d > %d", n, maxGossipMsgBytes)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func newGossipTransport(store *Store, bindAddr string, peers []string, secret string) *gossipTransport {
	g := &gossipTransport{
		store:    store,
		bindAddr: bindAddr,
		peers:    peers,
		secret:   secret,
		conns:    make(map[string]*peerConn),
		done:     make(chan struct{}),
		logger:   slog.Default(),
	}
	if secret == "" {
		g.logger.Warn("gossip: running WITHOUT authentication — any host that can reach " +
			"BindAddr can read and inject data. Set Options.GossipSecret on every node.")
	}
	go g.listen()
	go g.connectToPeers()
	return g
}

// handshake authenticates a freshly established connection by exchanging the
// pre-shared secret in both directions. It is a no-op when no secret is
// configured. Returns an error (and the caller closes the conn) on mismatch.
func (g *gossipTransport) handshake(conn net.Conn) error {
	if g.secret == "" {
		return nil
	}
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	defer conn.SetDeadline(time.Time{})

	// Send our secret length-prefixed.
	if err := writeFrame(conn, []byte(g.secret)); err != nil {
		return fmt.Errorf("gossip: handshake write: %w", err)
	}
	// Read the peer's secret and compare in constant time.
	peer, err := readFrame(conn, len(g.secret)+1)
	if err != nil {
		return fmt.Errorf("gossip: handshake read: %w", err)
	}
	if subtle.ConstantTimeCompare(peer, []byte(g.secret)) != 1 {
		return fmt.Errorf("gossip: handshake rejected: peer secret mismatch")
	}
	return nil
}

// writeFrame writes a 2-byte big-endian length prefix followed by b.
func writeFrame(w io.Writer, b []byte) error {
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// readFrame reads a length-prefixed frame, rejecting anything larger than max.
func readFrame(r io.Reader, max int) ([]byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n > max {
		return nil, fmt.Errorf("frame too large: %d > %d", n, max)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// listen accepts incoming peer connections.
func (g *gossipTransport) listen() {
	ln, err := net.Listen("tcp", g.bindAddr)
	if err != nil {
		g.logger.Error("gossip: listen failed", "addr", g.bindAddr, "error", err)
		return
	}
	g.listener = ln
	g.logger.Info("gossip: listening", "addr", g.bindAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-g.done:
				return
			default:
				g.logger.Warn("gossip: accept error", "error", err)
				continue
			}
		}
		go g.handleInbound(conn)
	}
}

// handleInbound authenticates an accepted peer connection, then reads from it.
func (g *gossipTransport) handleInbound(conn net.Conn) {
	if err := g.handshake(conn); err != nil {
		g.logger.Warn("gossip: rejecting inbound peer", "remote", conn.RemoteAddr(), "error", err)
		_ = conn.Close()
		return
	}
	g.readLoop(conn)
}

// connectToPeers establishes and maintains outbound connections to known peers.
func (g *gossipTransport) connectToPeers() {
	for _, peer := range g.peers {
		go g.maintainConn(peer)
	}
}

func (g *gossipTransport) maintainConn(addr string) {
	for {
		select {
		case <-g.done:
			return
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if err := g.handshake(conn); err != nil {
			g.logger.Warn("gossip: peer auth failed", "addr", addr, "error", err)
			_ = conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		pc := &peerConn{conn: conn}
		g.mu.Lock()
		g.conns[addr] = pc
		g.mu.Unlock()

		g.logger.Info("gossip: connected to peer", "addr", addr)
		g.readLoop(conn) // blocks until connection closes

		g.mu.Lock()
		delete(g.conns, addr)
		g.mu.Unlock()

		g.logger.Warn("gossip: peer disconnected, reconnecting", "addr", addr)
		time.Sleep(2 * time.Second)
	}
}

// readLoop reads length-framed gossip messages from a connection and applies
// them. Each frame is size-bounded and decoded independently.
func (g *gossipTransport) readLoop(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		frame, err := readMsgFrame(r)
		if err != nil {
			return
		}
		var msg gossipMsg
		if err := gob.NewDecoder(bytes.NewReader(frame)).Decode(&msg); err != nil {
			g.logger.Warn("gossip: bad message frame", "error", err)
			return
		}
		g.applyMsg(msg)
	}
}

func (g *gossipTransport) applyMsg(msg gossipMsg) {
	switch msg.Type {
	case "set":
		g.store.applyRemoteSet(msg.Key, msg.Value, msg.ExpiresAt, msg.Clock)
	case "delete":
		g.store.applyRemoteDelete(msg.Key)
	case "publish":
		g.store.applyRemotePublish(msg.Topic, msg.Value)
	}
}

// broadcast sends a gossip message to all connected peers.
func (g *gossipTransport) broadcast(msg gossipMsg) {
	g.mu.RLock()
	conns := make([]*peerConn, 0, len(g.conns))
	for _, c := range g.conns {
		conns = append(conns, c)
	}
	g.mu.RUnlock()

	for _, pc := range conns {
		go func(p *peerConn) {
			if err := p.send(msg); err != nil {
				g.logger.Warn("gossip: broadcast error", "error", err)
			}
		}(pc)
	}
}

func (g *gossipTransport) broadcastSet(key string, e *entry) {
	g.broadcast(gossipMsg{
		Type:      "set",
		Key:       key,
		Value:     e.Value,
		ExpiresAt: e.ExpiresAt,
		Clock:     e.Clock,
	})
}

func (g *gossipTransport) broadcastDelete(key string) {
	g.broadcast(gossipMsg{Type: "delete", Key: key})
}

func (g *gossipTransport) broadcastPublish(topic string, payload any) {
	g.broadcast(gossipMsg{Type: "publish", Topic: topic, Value: payload})
}

func (g *gossipTransport) stop() {
	close(g.done)
	if g.listener != nil {
		_ = g.listener.Close()
	}
	g.mu.Lock()
	for _, c := range g.conns {
		_ = c.conn.Close()
	}
	g.mu.Unlock()
}

// ensure gossipMsg types are gob-registered
func init() {
	gob.Register(gossipMsg{})
	gob.Register(ClockValue{})
}

// stringer for debugging
func (g *gossipTransport) String() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return fmt.Sprintf("gossip{bind=%s peers=%d connected=%d}", g.bindAddr, len(g.peers), len(g.conns))
}
