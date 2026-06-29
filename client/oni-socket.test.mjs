// Tests for the OniSocket client using a mock WebSocket.
// Run: node --test  (from the client/ directory, after `npm run build`)
import { test } from "node:test";
import assert from "node:assert/strict";
import { OniSocket } from "./dist/oni-socket.js";

// A controllable mock matching the browser WebSocket surface the client uses.
class MockWebSocket {
  static OPEN = 1;
  static last = null;
  constructor(url) {
    this.url = url;
    this.readyState = 0;
    this.sent = [];
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    MockWebSocket.last = this;
  }
  // Simulate the connection opening.
  _open() {
    this.readyState = 1;
    this.onopen && this.onopen();
  }
  // Simulate a server message.
  _emit(obj) {
    this.onmessage && this.onmessage({ data: JSON.stringify(obj) });
  }
  _closeRemote() {
    this.readyState = 3;
    this.onclose && this.onclose({ code: 1006 });
  }
  send(data) {
    this.sent.push(JSON.parse(data));
  }
  close() {
    this.readyState = 3;
    this.onclose && this.onclose({ code: 1000 });
  }
}

function newSocket(extra = {}) {
  return new OniSocket("ws://localhost/ws", {
    WebSocket: MockWebSocket,
    reconnect: false,
    heartbeatMs: 0,
    ...extra,
  });
}

test("subscribes on first listener and dispatches matching events", () => {
  const s = newSocket();
  const ws = MockWebSocket.last;
  ws._open();

  const got = [];
  s.channel("chat.general").on("chat.message", (e) => got.push(e.payload));

  // Client should have sent a subscribe frame.
  const sub = ws.sent.find((m) => m.type === "oni:subscribe" && m.channel === "chat.general");
  assert.ok(sub, "expected an oni:subscribe frame for chat.general");

  // Server pushes an event on that channel.
  ws._emit({ type: "chat.message", channel: "chat.general", payload: { text: "hi" }, id: "e1" });
  assert.equal(got.length, 1);
  assert.deepEqual(got[0], { text: "hi" });
});

test("does not deliver events for other channels or types", () => {
  const s = newSocket();
  MockWebSocket.last._open();
  let calls = 0;
  s.channel("a").on("x", () => calls++);
  MockWebSocket.last._emit({ type: "x", channel: "b", payload: {} }); // wrong channel
  MockWebSocket.last._emit({ type: "y", channel: "a", payload: {} }); // wrong type
  assert.equal(calls, 0);
});

test("send emits an event frame to the channel", () => {
  const s = newSocket();
  const ws = MockWebSocket.last;
  ws._open();
  s.channel("room.1").send("ping", { n: 1 });
  const frame = ws.sent.find((m) => m.type === "ping" && m.channel === "room.1");
  assert.ok(frame);
  assert.deepEqual(frame.payload, { n: 1 });
});

test("tracks last event id and resumes after reconnect", () => {
  const s = newSocket({ reconnect: true, reconnectBaseMs: 1, reconnectMaxMs: 1 });
  const ws1 = MockWebSocket.last;
  ws1._open();
  s.channel("chat").on("msg", () => {});
  ws1._emit({ type: "msg", channel: "chat", payload: {}, id: "evt-42" });

  // Drop the connection; client should reconnect (new MockWebSocket).
  ws1._closeRemote();
  return new Promise((resolve) => {
    setTimeout(() => {
      const ws2 = MockWebSocket.last;
      assert.notEqual(ws2, ws1, "expected a new connection after reconnect");
      ws2._open();
      // Re-subscribe + resume frames should be sent on reopen.
      assert.ok(
        ws2.sent.find((m) => m.type === "oni:subscribe" && m.channel === "chat"),
        "expected re-subscribe after reconnect"
      );
      const resume = ws2.sent.find((m) => m.type === "oni:resume");
      assert.ok(resume && resume.id === "evt-42", "expected resume with last event id");
      resolve();
    }, 10);
  });
});

test("deduplicates replayed events by id (at-least-once resume)", () => {
  const s = newSocket();
  const ws = MockWebSocket.last;
  ws._open();
  const got = [];
  s.channel("c").on("e", (ev) => got.push(ev.payload.n));

  // Normal delivery.
  ws._emit({ type: "e", channel: "c", payload: { n: 1 }, id: "001" });
  ws._emit({ type: "e", channel: "c", payload: { n: 2 }, id: "002" });
  // Server replays 1 and 2 (already seen) plus a new 3 — only 3 is new.
  ws._emit({ type: "e", channel: "c", payload: { n: 1 }, id: "001" });
  ws._emit({ type: "e", channel: "c", payload: { n: 2 }, id: "002" });
  ws._emit({ type: "e", channel: "c", payload: { n: 3 }, id: "003" });

  assert.deepEqual(got, [1, 2, 3], "duplicates must be dropped, new events delivered");
});

test("leave unsubscribes and stops delivery", () => {
  const s = newSocket();
  const ws = MockWebSocket.last;
  ws._open();
  const ch = s.channel("c");
  let calls = 0;
  ch.on("e", () => calls++);
  ch.leave();
  assert.ok(ws.sent.find((m) => m.type === "oni:unsubscribe" && m.channel === "c"));
  ws._emit({ type: "e", channel: "c", payload: {} });
  assert.equal(calls, 0, "no delivery after leave");
});
