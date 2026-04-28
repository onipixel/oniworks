package memory

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

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
	conns    map[string]net.Conn
	mu       sync.RWMutex
	listener net.Listener
	done     chan struct{}
	logger   *slog.Logger
}

func newGossipTransport(store *Store, bindAddr string, peers []string) *gossipTransport {
	g := &gossipTransport{
		store:    store,
		bindAddr: bindAddr,
		peers:    peers,
		conns:    make(map[string]net.Conn),
		done:     make(chan struct{}),
		logger:   slog.Default(),
	}
	go g.listen()
	go g.connectToPeers()
	return g
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
		go g.handleConn(conn)
	}
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

		g.mu.Lock()
		g.conns[addr] = conn
		g.mu.Unlock()

		g.logger.Info("gossip: connected to peer", "addr", addr)
		g.handleConn(conn) // blocks until connection closes

		g.mu.Lock()
		delete(g.conns, addr)
		g.mu.Unlock()

		g.logger.Warn("gossip: peer disconnected, reconnecting", "addr", addr)
		time.Sleep(2 * time.Second)
	}
}

// handleConn reads gossip messages from a connection and applies them.
func (g *gossipTransport) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := gob.NewDecoder(bufio.NewReader(conn))
	for {
		var msg gossipMsg
		if err := dec.Decode(&msg); err != nil {
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
	conns := make([]net.Conn, 0, len(g.conns))
	for _, c := range g.conns {
		conns = append(conns, c)
	}
	g.mu.RUnlock()

	for _, conn := range conns {
		go func(c net.Conn) {
			enc := gob.NewEncoder(c)
			if err := enc.Encode(msg); err != nil {
				g.logger.Warn("gossip: broadcast error", "error", err)
			}
		}(conn)
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
		_ = c.Close()
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
