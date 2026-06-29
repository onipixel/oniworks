package memory

import (
	"bytes"
	"log/slog"
	"net"
	"testing"
)

func newTestTransport(secret string) *gossipTransport {
	return &gossipTransport{secret: secret, logger: slog.Default()}
}

// dialPair sets up a loopback TCP connection and returns the client conn plus a
// channel delivering the accepted server conn.
func dialPair(t *testing.T) (client net.Conn, serverCh chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	serverCh = make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			serverCh <- c
		}
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return client, serverCh
}

// TestGossipHandshakeMatch verifies two nodes with the same secret authenticate.
func TestGossipHandshakeMatch(t *testing.T) {
	client, serverCh := dialPair(t)
	srv := newTestTransport("shared-secret")
	cli := newTestTransport("shared-secret")

	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.handshake(<-serverCh) }()

	if err := cli.handshake(client); err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	if err := <-srvErr; err != nil {
		t.Fatalf("server handshake: %v", err)
	}
}

// TestGossipHandshakeRejectsWrongSecret is the core auth regression: a peer with
// the wrong secret must be rejected.
func TestGossipHandshakeRejectsWrongSecret(t *testing.T) {
	client, serverCh := dialPair(t)
	srv := newTestTransport("right-secret")
	cli := newTestTransport("wrong-secret")

	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.handshake(<-serverCh) }()

	cliErr := cli.handshake(client)
	if cliErr == nil && <-srvErr == nil {
		t.Fatal("expected handshake to reject mismatched secret on at least one side")
	}
}

// TestGossipNoSecretSkipsHandshake verifies handshake is a no-op when unset
// (backwards-compatible single-trusted-network mode).
func TestGossipNoSecretSkipsHandshake(t *testing.T) {
	g := newTestTransport("")
	if err := g.handshake(nil); err != nil {
		t.Fatalf("expected no-op handshake with empty secret, got %v", err)
	}
}

// TestMsgFrameRoundTrip verifies the 4-byte message framing and size guard.
func TestMsgFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("hello gossip frame")
	if err := writeMsgFrame(&buf, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readMsgFrame(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("round trip mismatch: got %q want %q", got, payload)
	}
}

// TestMsgFrameRejectsOversize verifies an over-large declared length is refused
// rather than allocating unbounded memory.
func TestMsgFrameRejectsOversize(t *testing.T) {
	// Hand-build a header claiming a huge length.
	var hdr [4]byte
	hdr[0] = 0xFF
	hdr[1] = 0xFF
	hdr[2] = 0xFF
	hdr[3] = 0xFF
	if _, err := readMsgFrame(bytes.NewReader(hdr[:])); err == nil {
		t.Fatal("expected oversize frame to be rejected")
	}
}
