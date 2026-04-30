import { ACPFrame, parseFrame } from "./types";

export type TransportOptions = {
  baseUrl: string;
  sessionId?: string;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
};

export type ConnectionState = "idle" | "connecting" | "open" | "closed";

export class WSTransport {
  private ws: WebSocket | null = null;
  private state: ConnectionState = "idle";
  private outbox: string[] = [];
  private listeners: Array<(f: ACPFrame) => void> = [];
  private stateListeners: Array<(s: ConnectionState) => void> = [];
  private lastEventId = 0;
  private attempt = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private stopped = false;

  constructor(private opts: TransportOptions) {}

  connect() {
    this.stopped = false;
    this.openSocket();
  }

  disconnect() {
    this.stopped = true;
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.setState("closed");
  }

  send(frame: string) {
    if (this.state === "open" && this.ws) this.ws.send(frame);
    else this.outbox.push(frame);
  }

  onFrame(cb: (f: ACPFrame) => void): () => void {
    this.listeners.push(cb);
    return () => {
      this.listeners = this.listeners.filter((x) => x !== cb);
    };
  }

  onState(cb: (s: ConnectionState) => void): () => void {
    this.stateListeners.push(cb);
    return () => {
      this.stateListeners = this.stateListeners.filter((x) => x !== cb);
    };
  }

  private setState(s: ConnectionState) {
    this.state = s;
    this.stateListeners.forEach((cb) => cb(s));
  }

  private buildUrl(): string {
    const u = new URL(this.opts.baseUrl);
    if (this.opts.sessionId) u.searchParams.set("session", this.opts.sessionId);
    if (this.lastEventId > 0) u.searchParams.set("last_event_id", String(this.lastEventId));
    return u.toString();
  }

  private openSocket() {
    if (this.stopped) return;
    this.setState("connecting");
    const ws: WebSocket = new (globalThis as any).WebSocket(this.buildUrl());
    this.ws = ws;

    const onOpen = () => {
      this.attempt = 0;
      this.setState("open");
      const queued = this.outbox;
      this.outbox = [];
      queued.forEach((s) => ws.send(s));
    };
    const onMessage = (ev: any) => {
      const data = typeof ev.data === "string" ? ev.data : "";
      const f = parseFrame(data);
      if (!f) return;
      if (f.eventId !== undefined) {
        if (f.eventId <= this.lastEventId) return; // dedup
        this.lastEventId = f.eventId;
      }
      this.listeners.forEach((cb) => cb(f));
    };
    const onClose = () => {
      this.ws = null;
      if (this.stopped) {
        this.setState("closed");
        return;
      }
      this.scheduleReconnect();
    };

    ws.addEventListener("open", onOpen);
    ws.addEventListener("message", onMessage);
    ws.addEventListener("close", onClose);
    ws.addEventListener("error", onClose);
  }

  private scheduleReconnect() {
    this.setState("connecting");
    const base = this.opts.initialBackoffMs ?? 1000;
    const cap = this.opts.maxBackoffMs ?? 30_000;
    const exp = Math.min(cap, base * 2 ** this.attempt);
    const jitter = Math.random() * exp * 0.3;
    const delay = exp + jitter;
    this.attempt++;
    this.timer = setTimeout(() => this.openSocket(), delay);
  }
}
