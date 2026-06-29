/**
 * OniSocket — the official TypeScript client for OniWorks realtime (Oni Socket).
 *
 * Dependency-free. Works in the browser (uses the global WebSocket) and anywhere
 * a WebSocket implementation is provided via options.WebSocket.
 *
 *   const socket = new OniSocket("/ws", { token: jwt })
 *   socket.channel("chat.general")
 *     .on("chat.message", (e) => appendMessage(e.payload))
 *   socket.channel("chat.general").send("chat.message", { text: "hey!" })
 *
 * Features: channel subscribe/unsubscribe, typed event listeners, auto-reconnect
 * with exponential backoff, reconnect/resume via last_event_id (no missed
 * events), heartbeat ping/pong, and presence helpers.
 */

/** The wire envelope exchanged with the server. */
export interface OniEvent<T = any> {
  id?: string;
  type: string;
  channel?: string;
  payload?: T;
  ts?: number;
}

/** System event types (mirror framework/realtime/event.go). */
const SYS = {
  Error: "oni:error",
  Ping: "oni:ping",
  Pong: "oni:pong",
  Resume: "oni:resume",
  Ack: "oni:ack",
  Subscribe: "oni:subscribe",
  Unsubscribe: "oni:unsubscribe",
} as const;

export type ConnectionState = "connecting" | "open" | "closed";

export interface OniSocketOptions {
  /** Auth token sent as ?token=… on the connection URL (JWT or session token). */
  token?: string;
  /** Reconnect automatically on unexpected close (default: true). */
  reconnect?: boolean;
  /** Initial reconnect delay in ms (default: 500). */
  reconnectBaseMs?: number;
  /** Max reconnect delay in ms (default: 10000). */
  reconnectMaxMs?: number;
  /** Heartbeat interval in ms; 0 disables (default: 25000). */
  heartbeatMs?: number;
  /** Inject a WebSocket implementation (for Node/testing). Defaults to global. */
  WebSocket?: any;
  onOpen?: () => void;
  onClose?: (ev?: any) => void;
  onError?: (err: any) => void;
  onStateChange?: (state: ConnectionState) => void;
}

type Listener = (e: OniEvent) => void;

/** A subscribed channel. Obtain one via socket.channel(name). */
export class Channel {
  private listeners = new Map<string, Set<Listener>>();
  private subscribed = false;

  constructor(public readonly name: string, private socket: OniSocket) {}

  /** Register a listener for an event type on this channel. Returns this. */
  on<T = any>(type: string, cb: (e: OniEvent<T>) => void): this {
    let set = this.listeners.get(type);
    if (!set) {
      set = new Set();
      this.listeners.set(type, set);
    }
    set.add(cb as Listener);
    this.ensureSubscribed();
    return this;
  }

  /** Remove a listener (or all listeners for a type if cb omitted). */
  off(type: string, cb?: Listener): this {
    const set = this.listeners.get(type);
    if (!set) return this;
    if (cb) set.delete(cb);
    else set.clear();
    return this;
  }

  /** Send an event to this channel. */
  send<T = any>(type: string, payload?: T): this {
    this.ensureSubscribed();
    this.socket._send({ type, channel: this.name, payload });
    return this;
  }

  /** Presence convenience: fire cb when members join this channel. */
  joining(cb: (e: OniEvent) => void): this {
    return this.on("presence.join", cb);
  }

  /** Presence convenience: fire cb when members leave this channel. */
  leaving(cb: (e: OniEvent) => void): this {
    return this.on("presence.leave", cb);
  }

  /** Unsubscribe and drop all listeners. */
  leave(): void {
    if (this.subscribed) {
      this.socket._send({ type: SYS.Unsubscribe, channel: this.name });
      this.subscribed = false;
    }
    this.listeners.clear();
    this.socket._dropChannel(this.name);
  }

  /** @internal dispatch an incoming event to matching listeners. */
  _dispatch(e: OniEvent): void {
    const set = this.listeners.get(e.type);
    if (set) for (const cb of set) cb(e);
    const all = this.listeners.get("*");
    if (all) for (const cb of all) cb(e);
  }

  /** @internal (re)send the subscribe frame when the socket is open. */
  _resubscribe(): void {
    if (this.listeners.size > 0) {
      this.subscribed = true;
      this.socket._send({ type: SYS.Subscribe, channel: this.name });
    }
  }

  private ensureSubscribed(): void {
    if (!this.subscribed) {
      this.subscribed = true;
      this.socket._send({ type: SYS.Subscribe, channel: this.name });
    }
  }
}

export class OniSocket {
  private ws: any = null;
  private channels = new Map<string, Channel>();
  private opts: Required<Omit<OniSocketOptions, "token" | "WebSocket" | "onOpen" | "onClose" | "onError" | "onStateChange">> &
    OniSocketOptions;
  private lastEventID = "";
  private reconnectAttempts = 0;
  private heartbeat: any = null;
  private closedByUser = false;
  private state: ConnectionState = "closed";

  constructor(private url: string, options: OniSocketOptions = {}) {
    this.opts = {
      reconnect: options.reconnect ?? true,
      reconnectBaseMs: options.reconnectBaseMs ?? 500,
      reconnectMaxMs: options.reconnectMaxMs ?? 10000,
      heartbeatMs: options.heartbeatMs ?? 25000,
      ...options,
    };
    this.connect();
  }

  /** Get (or lazily create) a channel by name. */
  channel(name: string): Channel {
    let ch = this.channels.get(name);
    if (!ch) {
      ch = new Channel(name, this);
      this.channels.set(name, ch);
    }
    return ch;
  }

  /** Current connection state. */
  getState(): ConnectionState {
    return this.state;
  }

  /** Close the connection and stop reconnecting. */
  close(): void {
    this.closedByUser = true;
    this.stopHeartbeat();
    if (this.ws) this.ws.close();
  }

  // ─────────────────────────── internals ───────────────────────────

  private connect(): void {
    this.setState("connecting");
    const WS = this.opts.WebSocket || (globalThis as any).WebSocket;
    if (!WS) throw new Error("OniSocket: no WebSocket implementation available");

    const ws = new WS(this.buildURL());
    this.ws = ws;

    ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.setState("open");
      // Re-subscribe channels, then ask the server to replay anything missed.
      for (const ch of this.channels.values()) ch._resubscribe();
      if (this.lastEventID) {
        this._send({ type: SYS.Resume, channel: "*", id: this.lastEventID });
      }
      this.startHeartbeat();
      this.opts.onOpen?.();
    };

    ws.onmessage = (msg: any) => this.onMessage(msg.data);

    ws.onerror = (err: any) => this.opts.onError?.(err);

    ws.onclose = (ev: any) => {
      this.stopHeartbeat();
      this.setState("closed");
      this.opts.onClose?.(ev);
      if (!this.closedByUser && this.opts.reconnect) this.scheduleReconnect();
    };
  }

  private onMessage(data: any): void {
    let e: OniEvent;
    try {
      e = JSON.parse(typeof data === "string" ? data : data.toString());
    } catch {
      return;
    }

    // Resume delivery is at-least-once: after a reconnect the server may replay
    // events we've already processed. Server event IDs are monotonic, fixed-width
    // strings, so anything not strictly greater than the last one we saw is a
    // duplicate and is dropped. (System frames carry no id and are unaffected.)
    if (e.id) {
      if (e.id <= this.lastEventID) return;
      this.lastEventID = e.id;
    }

    if (e.type === SYS.Pong || e.type === SYS.Ack) return;
    if (e.type === SYS.Error) {
      this.opts.onError?.(e.payload ?? e);
      return;
    }
    if (e.channel) {
      this.channels.get(e.channel)?._dispatch(e);
    }
  }

  private scheduleReconnect(): void {
    const delay = Math.min(
      this.opts.reconnectMaxMs!,
      this.opts.reconnectBaseMs! * 2 ** this.reconnectAttempts
    );
    this.reconnectAttempts++;
    setTimeout(() => {
      if (!this.closedByUser) this.connect();
    }, delay);
  }

  private startHeartbeat(): void {
    if (!this.opts.heartbeatMs) return;
    this.stopHeartbeat();
    this.heartbeat = setInterval(() => {
      this._send({ type: SYS.Ping });
    }, this.opts.heartbeatMs);
  }

  private stopHeartbeat(): void {
    if (this.heartbeat) {
      clearInterval(this.heartbeat);
      this.heartbeat = null;
    }
  }

  private buildURL(): string {
    let url = this.url;
    // Resolve a relative path against the page origin (browser only).
    if (url.startsWith("/") && typeof location !== "undefined") {
      const proto = location.protocol === "https:" ? "wss:" : "ws:";
      url = `${proto}//${location.host}${url}`;
    }
    const params: string[] = [];
    if (this.opts.token) params.push(`token=${encodeURIComponent(this.opts.token)}`);
    if (this.lastEventID) params.push(`last_event_id=${encodeURIComponent(this.lastEventID)}`);
    if (params.length) url += (url.includes("?") ? "&" : "?") + params.join("&");
    return url;
  }

  private setState(s: ConnectionState): void {
    if (this.state !== s) {
      this.state = s;
      this.opts.onStateChange?.(s);
    }
  }

  /** @internal send a frame if the socket is open (drops otherwise). */
  _send(e: OniEvent): void {
    if (this.ws && this.ws.readyState === 1 /* OPEN */) {
      this.ws.send(JSON.stringify(e));
    }
  }

  /** @internal remove a channel from the registry. */
  _dropChannel(name: string): void {
    this.channels.delete(name);
  }
}

export default OniSocket;
