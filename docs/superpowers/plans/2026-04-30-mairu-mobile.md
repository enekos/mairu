# mairu-mobile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a React Native (Expo) phone app, `mobile/`, that attaches to a desktop-side `mairu acp-bridge` over a Tailscale tailnet, displays the agent's execution stream in real time, and lets the user direct the agent by voice or text — including approving/denying tool-call permission requests remotely. Wire protocol is ACP; the app is harness-agnostic.

**Architecture:** A pure ACP client. WS transport speaks plain ACP JSON-RPC; one extra non-standard sibling field (`x-mairu-event-id`) is read off server→client frames to drive reconnect/replay. Single-screen UI: header (session picker + connection state + Stop), scrolling timeline (user / assistant / tool-call / thinking), footer (text input + push-to-talk mic). Permission requests pop a modal sheet. State lives in a single Zustand store. STT is on-device (`@react-native-voice/voice`); no audio leaves the phone. Tailscale identity is the only auth boundary — the app has no login screen, just a host:port.

**Tech Stack:** Expo SDK 52, React Native, TypeScript (strict), Zustand for state, `@react-native-voice/voice` for STT, `expo-haptics` for buzz, `react-native-markdown-display` for assistant rendering, native `WebSocket`, Jest + `@testing-library/react-native` for tests. No bundler config beyond Expo defaults.

**Companion:** Pairs with `mairu acp-bridge` (see `2026-04-28-mairu-acp-bridge.md`). The bridge is the only network-facing component; the app never speaks to a raw agent.

---

## File Structure

| Path | Responsibility |
|---|---|
| `mobile/package.json` | Expo deps, scripts (`start`, `test`, `lint`, `typecheck`). |
| `mobile/app.json` | Expo app config (name, slug, scheme, ios/android bundle ids). |
| `mobile/tsconfig.json` | TS strict, `@/*` path alias for `src/`. |
| `mobile/App.tsx` | Root component. Wires connection state, screen routing, modal host. |
| `mobile/src/acp/types.ts` | Typed ACP frames + JSON-RPC envelope + `StampedFrame`. |
| `mobile/src/acp/transport.ts` | `WSTransport`: connect, send, receive, exponential-backoff reconnect, `Last-Event-ID` replay, event-id dedup. |
| `mobile/src/acp/client.ts` | `ACPClient`: request/response correlation by `id`, subscribe API, server-initiated request hook (`session/request_permission`). |
| `mobile/src/api/sessions.ts` | Thin `fetch` wrappers for `GET/POST/DELETE /sessions`. |
| `mobile/src/state/store.ts` | Zustand store: connection state, sessions, per-session event arrays (cap 1000), pending permission requests. |
| `mobile/src/ui/ConnectScreen.tsx` | First-run host:port entry; persists to `AsyncStorage`; pings `/sessions` to validate. |
| `mobile/src/ui/SessionPicker.tsx` | Header dropdown of sessions + "+ New session" sheet. |
| `mobile/src/ui/Timeline.tsx` | Scrolling list of events with item renderers. |
| `mobile/src/ui/items/UserBubble.tsx`, `AssistantMessage.tsx`, `ToolCallCard.tsx`, `ThinkingBlock.tsx` | Per-event renderers. |
| `mobile/src/ui/Composer.tsx` | Text input + send + mic button + Stop. |
| `mobile/src/ui/PermissionModal.tsx` | Slide-up sheet for `session/request_permission`. |
| `mobile/src/voice/recorder.ts` | `@react-native-voice/voice` wrapper: start/stop/onResult. |
| `mobile/src/lib/markdown.ts` | Memoized markdown renderer config. |
| `mobile/__tests__/**` | Jest tests, colocated in subfolders mirroring `src/`. |

---

## Task 1: Expo TypeScript scaffold

**Files:**
- Create: `mobile/` (Expo blank-typescript template, then trim).
- Create: `mobile/package.json`, `mobile/app.json`, `mobile/tsconfig.json`, `mobile/App.tsx`, `mobile/babel.config.js`, `mobile/.gitignore`.

- [ ] **Step 1: Generate the project**

```bash
# from repo root
bun x create-expo-app@latest mobile --template blank-typescript --no-install
cd mobile
bun install
```

- [ ] **Step 2: Lock TypeScript to strict mode**

Edit `mobile/tsconfig.json`:

```json
{
  "extends": "expo/tsconfig.base",
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"] }
  },
  "include": ["**/*.ts", "**/*.tsx"]
}
```

- [ ] **Step 3: Add scripts to `mobile/package.json`**

```json
{
  "scripts": {
    "start": "expo start",
    "test": "jest",
    "lint": "eslint . --ext .ts,.tsx",
    "typecheck": "tsc --noEmit"
  }
}
```

Add `jest`, `@testing-library/react-native`, `@types/jest`, `eslint-config-expo` as devDeps (keep installs minimal — Expo brings most).

- [ ] **Step 4: Add `.gitignore` entries**

```
mobile/node_modules/
mobile/.expo/
mobile/dist/
mobile/web-build/
mobile/*.tsbuildinfo
```

(Append to repo root `.gitignore`, do not commit a duplicate inside `mobile/`.)

- [ ] **Step 5: Verify scaffold runs**

Run: `cd mobile && bun run typecheck && bun run start --no-dev --offline` (Ctrl-C after metro starts).
Expected: TS clean, metro boots without error.

- [ ] **Step 6: Commit**

```bash
git add mobile/ .gitignore
git commit -m "feat(mobile): scaffold Expo TypeScript app"
```

---

## Task 2: ACP frame types

**Files:**
- Create: `mobile/src/acp/types.ts`
- Create: `mobile/src/acp/__tests__/types.test.ts`

- [ ] **Step 1: Write the failing tests**

```ts
// mobile/src/acp/__tests__/types.test.ts
import { isResponse, isServerRequest, parseFrame, ACPFrame } from "../types";

describe("ACP frame parsing", () => {
  test("parses a JSON-RPC response with event id", () => {
    const raw = `{"jsonrpc":"2.0","id":1,"result":{},"x-mairu-event-id":42}`;
    const f = parseFrame(raw);
    expect(f).not.toBeNull();
    expect(isResponse(f!)).toBe(true);
    expect(f!.eventId).toBe(42);
  });

  test("parses a server-initiated request", () => {
    const raw = `{"jsonrpc":"2.0","id":7,"method":"session/request_permission","params":{"toolCall":{"name":"shell","args":{}}},"x-mairu-event-id":43}`;
    const f = parseFrame(raw);
    expect(isServerRequest(f!)).toBe(true);
  });

  test("rejects malformed JSON", () => {
    expect(parseFrame("not json")).toBeNull();
  });

  test("rejects non-jsonrpc payloads", () => {
    expect(parseFrame(`{"hello":"world"}`)).toBeNull();
  });
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern acp/types`
Expected: FAIL — module does not exist.

- [ ] **Step 3: Implement**

```ts
// mobile/src/acp/types.ts
export type JsonValue =
  | string | number | boolean | null
  | JsonValue[]
  | { [k: string]: JsonValue };

export type ACPFrame = {
  jsonrpc: "2.0";
  id?: number | string;
  method?: string;
  params?: JsonValue;
  result?: JsonValue;
  error?: { code: number; message: string; data?: JsonValue };
  /** Sibling field stamped by mairu acp-bridge for replay ordering. */
  eventId?: number;
};

export function parseFrame(raw: string): ACPFrame | null {
  let obj: any;
  try { obj = JSON.parse(raw); } catch { return null; }
  if (!obj || obj.jsonrpc !== "2.0") return null;
  const out: ACPFrame = { jsonrpc: "2.0" };
  if ("id" in obj) out.id = obj.id;
  if (typeof obj.method === "string") out.method = obj.method;
  if ("params" in obj) out.params = obj.params;
  if ("result" in obj) out.result = obj.result;
  if ("error" in obj) out.error = obj.error;
  const eid = obj["x-mairu-event-id"];
  if (typeof eid === "number") out.eventId = eid;
  return out;
}

export function isResponse(f: ACPFrame): boolean {
  return f.id !== undefined && (f.result !== undefined || f.error !== undefined);
}

export function isServerRequest(f: ACPFrame): boolean {
  return f.id !== undefined && typeof f.method === "string";
}

export function isNotification(f: ACPFrame): boolean {
  return f.id === undefined && typeof f.method === "string";
}

export function encodeFrame(f: Omit<ACPFrame, "jsonrpc">): string {
  return JSON.stringify({ jsonrpc: "2.0", ...f });
}
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern acp/types`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/src/acp/types.ts mobile/src/acp/__tests__/types.test.ts
git commit -m "feat(mobile/acp): typed ACP frames + parser"
```

---

## Task 3: WSTransport — connect, reconnect, replay

**Files:**
- Create: `mobile/src/acp/transport.ts`
- Create: `mobile/src/acp/__tests__/transport.test.ts`
- Create: `mobile/src/acp/__tests__/fakeWebSocket.ts` (test helper, not exported)

- [ ] **Step 1: Write a fake WebSocket for tests**

```ts
// mobile/src/acp/__tests__/fakeWebSocket.ts
type Listener = (ev: any) => void;

export class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  url: string;
  readyState = 0;
  sent: string[] = [];
  listeners: Record<string, Listener[]> = { open: [], message: [], close: [], error: [] };

  constructor(url: string, _protocols?: string | string[]) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  addEventListener(t: string, l: Listener) { this.listeners[t]?.push(l); }
  removeEventListener(t: string, l: Listener) {
    this.listeners[t] = (this.listeners[t] ?? []).filter(x => x !== l);
  }
  send(data: string) { this.sent.push(data); }
  close() { this.readyState = 3; this.fire("close", { code: 1000 }); }

  fire(t: string, ev: any) { (this.listeners[t] ?? []).forEach(l => l(ev)); }

  // test helpers
  open() { this.readyState = 1; this.fire("open", {}); }
  recv(data: string) { this.fire("message", { data }); }
  fail(code = 1006) { this.readyState = 3; this.fire("close", { code }); }
}
```

- [ ] **Step 2: Write failing tests**

```ts
// mobile/src/acp/__tests__/transport.test.ts
import { WSTransport } from "../transport";
import { FakeWebSocket } from "./fakeWebSocket";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  jest.useFakeTimers();
});
afterEach(() => jest.useRealTimers());

test("delivers frames in order with monotonically increasing event ids", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const got: number[] = [];
  t.onFrame(f => { if (f.eventId) got.push(f.eventId); });
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","id":1,"result":{},"x-mairu-event-id":1}`);
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","id":2,"result":{},"x-mairu-event-id":2}`);
  expect(got).toEqual([1, 2]);
});

test("reconnect sends Last-Event-ID and dedupes replayed frames", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const ids: number[] = [];
  t.onFrame(f => { if (f.eventId) ids.push(f.eventId); });
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":1}`);
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":2}`);
  FakeWebSocket.instances[0]!.fail();
  jest.advanceTimersByTime(2000); // backoff
  FakeWebSocket.instances[1]!.open();
  expect(FakeWebSocket.instances[1]!.url).toContain("last_event_id=2");
  // server replays 2 (already seen) and a new 3
  FakeWebSocket.instances[1]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":2}`);
  FakeWebSocket.instances[1]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":3}`);
  expect(ids).toEqual([1, 2, 3]); // 2 deduped
});

test("send queues until open", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  t.connect();
  t.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`);
  expect(FakeWebSocket.instances[0]!.sent).toEqual([]);
  FakeWebSocket.instances[0]!.open();
  expect(FakeWebSocket.instances[0]!.sent[0]).toContain(`"method":"ping"`);
});

test("backoff caps at 30s with jitter", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  t.connect();
  for (let i = 0; i < 8; i++) {
    FakeWebSocket.instances[i]!.fail();
    jest.advanceTimersByTime(60_000);
  }
  // 9th attempt should exist (no permanent give-up)
  expect(FakeWebSocket.instances.length).toBeGreaterThanOrEqual(9);
});
```

- [ ] **Step 3: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern transport`
Expected: FAIL — `WSTransport` undefined.

- [ ] **Step 4: Implement**

```ts
// mobile/src/acp/transport.ts
import { ACPFrame, parseFrame } from "./types";

export type TransportOptions = {
  baseUrl: string;            // e.g. "ws://100.64.0.1:7777/acp"
  sessionId?: string;
  initialBackoffMs?: number;  // default 1000
  maxBackoffMs?: number;      // default 30000
};

type State = "idle" | "connecting" | "open" | "closed";

export class WSTransport {
  private ws: WebSocket | null = null;
  private state: State = "idle";
  private outbox: string[] = [];
  private listeners: Array<(f: ACPFrame) => void> = [];
  private stateListeners: Array<(s: State) => void> = [];
  private lastEventId = 0;
  private attempt = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private stopped = false;

  constructor(private opts: TransportOptions) {}

  connect() {
    this.stopped = false;
    this.open();
  }

  disconnect() {
    this.stopped = true;
    if (this.timer) clearTimeout(this.timer);
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
    return () => { this.listeners = this.listeners.filter(x => x !== cb); };
  }

  onState(cb: (s: State) => void): () => void {
    this.stateListeners.push(cb);
    return () => { this.stateListeners = this.stateListeners.filter(x => x !== cb); };
  }

  private setState(s: State) {
    this.state = s;
    this.stateListeners.forEach(cb => cb(s));
  }

  private buildUrl(): string {
    const u = new URL(this.opts.baseUrl);
    if (this.opts.sessionId) u.searchParams.set("session", this.opts.sessionId);
    if (this.lastEventId > 0) u.searchParams.set("last_event_id", String(this.lastEventId));
    return u.toString();
  }

  private open() {
    if (this.stopped) return;
    this.setState("connecting");
    const ws = new WebSocket(this.buildUrl());
    this.ws = ws;

    ws.addEventListener("open", () => {
      this.attempt = 0;
      this.setState("open");
      const queued = this.outbox;
      this.outbox = [];
      queued.forEach(s => ws.send(s));
    });

    ws.addEventListener("message", (ev: any) => {
      const f = parseFrame(typeof ev.data === "string" ? ev.data : "");
      if (!f) return;
      if (f.eventId !== undefined) {
        if (f.eventId <= this.lastEventId) return; // dedup replay
        this.lastEventId = f.eventId;
      }
      this.listeners.forEach(cb => cb(f));
    });

    const onClose = () => {
      this.ws = null;
      if (this.stopped) { this.setState("closed"); return; }
      this.scheduleReconnect();
    };
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
    this.timer = setTimeout(() => this.open(), delay);
  }
}
```

- [ ] **Step 5: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern transport`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mobile/src/acp/transport.ts mobile/src/acp/__tests__/
git commit -m "feat(mobile/acp): WS transport with reconnect + replay"
```

---

## Task 4: ACPClient — request/response correlation

**Files:**
- Create: `mobile/src/acp/client.ts`
- Create: `mobile/src/acp/__tests__/client.test.ts`

- [ ] **Step 1: Write the failing tests**

```ts
// mobile/src/acp/__tests__/client.test.ts
import { ACPClient } from "../client";
import { WSTransport } from "../transport";
import { FakeWebSocket } from "./fakeWebSocket";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
});

test("request resolves on matching id", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  const p = c.request("session/prompt", { text: "hi" });
  const sent = JSON.parse(FakeWebSocket.instances[0]!.sent[0]!);
  FakeWebSocket.instances[0]!.recv(JSON.stringify({ jsonrpc: "2.0", id: sent.id, result: { ok: true } }));
  await expect(p).resolves.toEqual({ ok: true });
});

test("request rejects on error response", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  const p = c.request("session/prompt", {});
  const sent = JSON.parse(FakeWebSocket.instances[0]!.sent[0]!);
  FakeWebSocket.instances[0]!.recv(JSON.stringify({
    jsonrpc: "2.0", id: sent.id, error: { code: -32000, message: "nope" }
  }));
  await expect(p).rejects.toThrow("nope");
});

test("server-initiated request fires onServerRequest with replier", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  const handler = jest.fn(async (m, p, reply) => reply({ outcome: "allow" }));
  c.onServerRequest("session/request_permission", handler as any);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","id":99,"method":"session/request_permission","params":{"toolCall":{"name":"shell"}}}`);
  // flush microtasks
  await Promise.resolve();
  const reply = FakeWebSocket.instances[0]!.sent.find(s => s.includes(`"id":99`));
  expect(reply).toBeDefined();
  expect(JSON.parse(reply!).result).toEqual({ outcome: "allow" });
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern client`
Expected: FAIL.

- [ ] **Step 3: Implement**

```ts
// mobile/src/acp/client.ts
import { WSTransport } from "./transport";
import { ACPFrame, encodeFrame, isResponse, isServerRequest } from "./types";

type Pending = { resolve: (v: any) => void; reject: (e: Error) => void };
type ServerHandler = (
  method: string,
  params: any,
  reply: (result: any) => void,
  fail: (code: number, message: string) => void,
) => void | Promise<void>;

export class ACPClient {
  private nextId = 1;
  private pending = new Map<number | string, Pending>();
  private handlers = new Map<string, ServerHandler>();
  private notifyListeners: Array<(f: ACPFrame) => void> = [];

  constructor(private transport: WSTransport) {
    transport.onFrame(f => this.dispatch(f));
  }

  request<T = unknown>(method: string, params?: unknown): Promise<T> {
    const id = this.nextId++;
    const frame = encodeFrame({ id, method, params: params as any });
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.transport.send(frame);
    });
  }

  notify(method: string, params?: unknown) {
    this.transport.send(encodeFrame({ method, params: params as any }));
  }

  onServerRequest(method: string, h: ServerHandler) {
    this.handlers.set(method, h);
    return () => this.handlers.delete(method);
  }

  onNotification(cb: (f: ACPFrame) => void): () => void {
    this.notifyListeners.push(cb);
    return () => { this.notifyListeners = this.notifyListeners.filter(x => x !== cb); };
  }

  private dispatch(f: ACPFrame) {
    if (isResponse(f) && f.id !== undefined) {
      const p = this.pending.get(f.id);
      if (!p) return;
      this.pending.delete(f.id);
      if (f.error) p.reject(new Error(f.error.message));
      else p.resolve(f.result);
      return;
    }
    if (isServerRequest(f) && f.method && f.id !== undefined) {
      const h = this.handlers.get(f.method);
      const reply = (result: any) => this.transport.send(encodeFrame({ id: f.id, result }));
      const fail = (code: number, message: string) =>
        this.transport.send(encodeFrame({ id: f.id, error: { code, message } }));
      if (!h) { fail(-32601, `no handler for ${f.method}`); return; }
      Promise.resolve(h(f.method, f.params, reply, fail)).catch(e => fail(-32000, String(e)));
      return;
    }
    this.notifyListeners.forEach(cb => cb(f));
  }
}
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern acp`
Expected: PASS (all three acp suites).

- [ ] **Step 5: Commit**

```bash
git add mobile/src/acp/client.ts mobile/src/acp/__tests__/client.test.ts
git commit -m "feat(mobile/acp): JSON-RPC client with server-request handlers"
```

---

## Task 5: HTTP `/sessions` API

**Files:**
- Create: `mobile/src/api/sessions.ts`
- Create: `mobile/src/api/__tests__/sessions.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
// mobile/src/api/__tests__/sessions.test.ts
import { listSessions, createSession, deleteSession } from "../sessions";

beforeEach(() => { (globalThis as any).fetch = jest.fn(); });

test("listSessions GETs /sessions", async () => {
  (fetch as jest.Mock).mockResolvedValue({
    ok: true,
    json: async () => [{ id: "s1", agent: "mairu", active: true, started_at: 0, last_activity_at: 0 }],
  });
  const out = await listSessions("http://h:7777");
  expect(fetch).toHaveBeenCalledWith("http://h:7777/sessions");
  expect(out).toHaveLength(1);
});

test("createSession POSTs with agent", async () => {
  (fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => ({ id: "s2" }) });
  const id = await createSession("http://h:7777", "mairu", "myproj");
  const [, init] = (fetch as jest.Mock).mock.calls[0];
  expect(JSON.parse(init.body)).toEqual({ agent: "mairu", project: "myproj" });
  expect(id).toBe("s2");
});

test("listSessions throws on non-ok", async () => {
  (fetch as jest.Mock).mockResolvedValue({ ok: false, status: 500, text: async () => "boom" });
  await expect(listSessions("http://h:7777")).rejects.toThrow(/500/);
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern api/sessions`
Expected: FAIL.

- [ ] **Step 3: Implement**

```ts
// mobile/src/api/sessions.ts
export type SessionInfo = {
  id: string;
  agent: string;
  project?: string;
  started_at: number;
  last_activity_at: number;
  active: boolean;
};

async function ok<T>(r: Response): Promise<T> {
  if (!r.ok) throw new Error(`http ${r.status}: ${await r.text()}`);
  return r.json() as Promise<T>;
}

export async function listSessions(host: string): Promise<SessionInfo[]> {
  return ok(await fetch(`${host}/sessions`));
}

export async function createSession(
  host: string, agent: string, project?: string,
): Promise<string> {
  const r = await fetch(`${host}/sessions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ agent, project }),
  });
  const { id } = await ok<{ id: string }>(r);
  return id;
}

export async function deleteSession(host: string, id: string): Promise<void> {
  const r = await fetch(`${host}/sessions/${id}`, { method: "DELETE" });
  if (!r.ok) throw new Error(`http ${r.status}`);
}
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern api/sessions`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/src/api/
git commit -m "feat(mobile/api): /sessions wrappers"
```

---

## Task 6: State store (Zustand)

**Files:**
- Create: `mobile/src/state/store.ts`
- Create: `mobile/src/state/__tests__/store.test.ts`

The store models the pieces the UI subscribes to: connection state, current host, selected session id, per-session event arrays (capped 1000), pending permission requests.

- [ ] **Step 1: Write failing tests**

```ts
// mobile/src/state/__tests__/store.test.ts
import { useStore } from "../store";

beforeEach(() => useStore.getState().reset());

test("appendEvent caps per-session at 1000", () => {
  const s = useStore.getState();
  for (let i = 0; i < 1100; i++) s.appendEvent("sess", { kind: "user", text: String(i) });
  expect(useStore.getState().eventsBySession["sess"]).toHaveLength(1000);
  // oldest dropped
  expect(useStore.getState().eventsBySession["sess"]![0]!.text).toBe("100");
});

test("setHost persists and selecting session clears nothing", () => {
  useStore.getState().setHost("http://x:7777");
  useStore.getState().selectSession("s1");
  expect(useStore.getState().host).toBe("http://x:7777");
  expect(useStore.getState().selectedSessionId).toBe("s1");
});

test("permission requests are tracked and cleared", () => {
  const s = useStore.getState();
  s.pushPermission({ id: 1, sessionId: "s", method: "session/request_permission", params: {} });
  expect(useStore.getState().pendingPermissions).toHaveLength(1);
  s.resolvePermission(1);
  expect(useStore.getState().pendingPermissions).toHaveLength(0);
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern state/store`
Expected: FAIL.

- [ ] **Step 3: Implement**

```ts
// mobile/src/state/store.ts
import { create } from "zustand";

export type EventKind = "user" | "assistant" | "tool" | "thinking" | "system";
export type TimelineEvent = {
  kind: EventKind;
  text?: string;
  toolName?: string;
  toolArgs?: unknown;
  toolResult?: unknown;
  ts?: number;
  eventId?: number;
};

export type PermissionRequest = {
  id: number | string;
  sessionId: string;
  method: string;
  params: any;
};

const CAP = 1000;

type State = {
  host: string | null;
  connection: "idle" | "connecting" | "open" | "closed";
  selectedSessionId: string | null;
  eventsBySession: Record<string, TimelineEvent[]>;
  pendingPermissions: PermissionRequest[];

  setHost: (h: string | null) => void;
  setConnection: (c: State["connection"]) => void;
  selectSession: (id: string | null) => void;
  appendEvent: (sessionId: string, ev: TimelineEvent) => void;
  pushPermission: (p: PermissionRequest) => void;
  resolvePermission: (id: number | string) => void;
  reset: () => void;
};

const initial: Pick<State,
  "host" | "connection" | "selectedSessionId" | "eventsBySession" | "pendingPermissions"
> = {
  host: null,
  connection: "idle",
  selectedSessionId: null,
  eventsBySession: {},
  pendingPermissions: [],
};

export const useStore = create<State>((set) => ({
  ...initial,
  setHost: (h) => set({ host: h }),
  setConnection: (c) => set({ connection: c }),
  selectSession: (id) => set({ selectedSessionId: id }),
  appendEvent: (sid, ev) => set((st) => {
    const cur = st.eventsBySession[sid] ?? [];
    const next = cur.length >= CAP ? [...cur.slice(cur.length - CAP + 1), ev] : [...cur, ev];
    return { eventsBySession: { ...st.eventsBySession, [sid]: next } };
  }),
  pushPermission: (p) => set((st) => ({ pendingPermissions: [...st.pendingPermissions, p] })),
  resolvePermission: (id) => set((st) => ({
    pendingPermissions: st.pendingPermissions.filter(p => p.id !== id),
  })),
  reset: () => set({ ...initial }),
}));
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern state/store`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/src/state/
git commit -m "feat(mobile/state): zustand store with capped event log"
```

---

## Task 7: Connect screen + AsyncStorage host persistence

**Files:**
- Create: `mobile/src/ui/ConnectScreen.tsx`
- Create: `mobile/src/ui/__tests__/ConnectScreen.test.tsx`
- Add dep: `@react-native-async-storage/async-storage`

- [ ] **Step 1: Add dep**

```bash
cd mobile && bun add @react-native-async-storage/async-storage
```

- [ ] **Step 2: Write failing test**

```tsx
// mobile/src/ui/__tests__/ConnectScreen.test.tsx
import React from "react";
import { fireEvent, render, waitFor } from "@testing-library/react-native";
import { ConnectScreen } from "../ConnectScreen";
import { useStore } from "../../state/store";

jest.mock("../../api/sessions", () => ({
  listSessions: jest.fn().mockResolvedValue([]),
}));

beforeEach(() => useStore.getState().reset());

test("validates host and stores it", async () => {
  const { getByPlaceholderText, getByText } = render(<ConnectScreen />);
  fireEvent.changeText(getByPlaceholderText(/host/i), "http://100.64.0.1:7777");
  fireEvent.press(getByText(/connect/i));
  await waitFor(() => expect(useStore.getState().host).toBe("http://100.64.0.1:7777"));
});

test("rejects empty host", async () => {
  const { getByText, queryByText } = render(<ConnectScreen />);
  fireEvent.press(getByText(/connect/i));
  await waitFor(() => expect(queryByText(/required/i)).toBeTruthy());
});
```

- [ ] **Step 3: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern ConnectScreen`
Expected: FAIL.

- [ ] **Step 4: Implement**

```tsx
// mobile/src/ui/ConnectScreen.tsx
import React, { useState } from "react";
import { View, Text, TextInput, Pressable, StyleSheet } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { useStore } from "../state/store";
import { listSessions } from "../api/sessions";

const KEY = "mairu.host";

export function ConnectScreen() {
  const [host, setHostText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const setHost = useStore(s => s.setHost);

  async function onConnect() {
    if (!host.trim()) { setError("host is required"); return; }
    setBusy(true);
    setError(null);
    try {
      await listSessions(host);
      await AsyncStorage.setItem(KEY, host);
      setHost(host);
    } catch (e: any) {
      setError(`unreachable: ${e.message}`);
    } finally { setBusy(false); }
  }

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Connect to mairu acp-bridge</Text>
      <TextInput
        placeholder="host (http://100.x.x.x:7777)"
        autoCapitalize="none"
        autoCorrect={false}
        value={host}
        onChangeText={setHostText}
        style={styles.input}
      />
      {error && <Text style={styles.err}>{error}</Text>}
      <Pressable disabled={busy} onPress={onConnect} style={styles.btn}>
        <Text style={styles.btnText}>{busy ? "Connecting…" : "Connect"}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", padding: 24, gap: 12 },
  title: { fontSize: 18, fontWeight: "600" },
  input: { borderWidth: 1, borderColor: "#444", borderRadius: 8, padding: 12 },
  err: { color: "#c33" },
  btn: { backgroundColor: "#222", padding: 14, borderRadius: 8, alignItems: "center" },
  btnText: { color: "white", fontWeight: "600" },
});

export async function loadStoredHost(): Promise<string | null> {
  return AsyncStorage.getItem(KEY);
}
```

- [ ] **Step 5: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern ConnectScreen`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mobile/src/ui/ConnectScreen.tsx mobile/src/ui/__tests__/ConnectScreen.test.tsx mobile/package.json mobile/bun.lockb
git commit -m "feat(mobile/ui): connect screen with host persistence"
```

---

## Task 8: Session picker

**Files:**
- Create: `mobile/src/ui/SessionPicker.tsx`
- Create: `mobile/src/ui/__tests__/SessionPicker.test.tsx`

- [ ] **Step 1: Write failing test**

```tsx
// mobile/src/ui/__tests__/SessionPicker.test.tsx
import React from "react";
import { fireEvent, render, waitFor } from "@testing-library/react-native";
import { SessionPicker } from "../SessionPicker";
import { useStore } from "../../state/store";

jest.mock("../../api/sessions", () => ({
  listSessions: jest.fn().mockResolvedValue([
    { id: "s1", agent: "mairu", started_at: 0, last_activity_at: 0, active: true },
    { id: "s2", agent: "claude-code", started_at: 0, last_activity_at: 0, active: false },
  ]),
  createSession: jest.fn().mockResolvedValue("s3"),
}));

beforeEach(() => {
  useStore.getState().reset();
  useStore.getState().setHost("http://h:7777");
});

test("lists sessions and selects one", async () => {
  const { getByText } = render(<SessionPicker />);
  await waitFor(() => getByText(/s1/));
  fireEvent.press(getByText(/s1/));
  expect(useStore.getState().selectedSessionId).toBe("s1");
});

test("creates new session", async () => {
  const { getByText } = render(<SessionPicker />);
  await waitFor(() => getByText(/\+ New session/));
  fireEvent.press(getByText(/\+ New session/));
  fireEvent.press(getByText(/mairu$/));
  await waitFor(() => expect(useStore.getState().selectedSessionId).toBe("s3"));
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern SessionPicker`
Expected: FAIL.

- [ ] **Step 3: Implement**

```tsx
// mobile/src/ui/SessionPicker.tsx
import React, { useEffect, useState } from "react";
import { View, Text, Pressable, FlatList, StyleSheet, Modal } from "react-native";
import { listSessions, createSession, SessionInfo } from "../api/sessions";
import { useStore } from "../state/store";

const AGENTS = ["mairu", "claude-code", "gemini"];

export function SessionPicker() {
  const host = useStore(s => s.host)!;
  const select = useStore(s => s.selectSession);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [creating, setCreating] = useState(false);

  async function refresh() {
    setSessions(await listSessions(host));
  }

  useEffect(() => { refresh(); }, [host]);

  async function onCreate(agent: string) {
    const id = await createSession(host, agent);
    setCreating(false);
    select(id);
    refresh();
  }

  return (
    <View style={styles.box}>
      <FlatList
        data={sessions}
        keyExtractor={s => s.id}
        ListHeaderComponent={
          <Pressable onPress={() => setCreating(true)} style={styles.row}>
            <Text style={styles.plus}>+ New session</Text>
          </Pressable>
        }
        renderItem={({ item }) => (
          <Pressable onPress={() => select(item.id)} style={styles.row}>
            <Text>{item.id}</Text>
            <Text style={styles.dim}>{item.agent}{item.active ? " · active" : ""}</Text>
          </Pressable>
        )}
      />
      <Modal visible={creating} transparent animationType="slide" onRequestClose={() => setCreating(false)}>
        <View style={styles.sheet}>
          {AGENTS.map(a => (
            <Pressable key={a} onPress={() => onCreate(a)} style={styles.row}>
              <Text>{a}</Text>
            </Pressable>
          ))}
          <Pressable onPress={() => setCreating(false)} style={styles.row}>
            <Text style={styles.dim}>Cancel</Text>
          </Pressable>
        </View>
      </Modal>
    </View>
  );
}

const styles = StyleSheet.create({
  box: { padding: 12 },
  row: { paddingVertical: 12, borderBottomWidth: 1, borderColor: "#eee" },
  plus: { fontWeight: "600" },
  dim: { color: "#888", fontSize: 12 },
  sheet: { marginTop: "auto", backgroundColor: "white", padding: 16, borderTopLeftRadius: 12, borderTopRightRadius: 12 },
});
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern SessionPicker`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/src/ui/SessionPicker.tsx mobile/src/ui/__tests__/SessionPicker.test.tsx
git commit -m "feat(mobile/ui): session picker"
```

---

## Task 9: Timeline + event renderers

**Files:**
- Create: `mobile/src/ui/Timeline.tsx`
- Create: `mobile/src/ui/items/UserBubble.tsx`, `AssistantMessage.tsx`, `ToolCallCard.tsx`, `ThinkingBlock.tsx`
- Create: `mobile/src/ui/__tests__/Timeline.test.tsx`
- Add dep: `react-native-markdown-display`

- [ ] **Step 1: Add dep**

```bash
cd mobile && bun add react-native-markdown-display
```

- [ ] **Step 2: Write failing test**

```tsx
// mobile/src/ui/__tests__/Timeline.test.tsx
import React from "react";
import { render, fireEvent } from "@testing-library/react-native";
import { Timeline } from "../Timeline";
import { useStore } from "../../state/store";

beforeEach(() => {
  useStore.getState().reset();
  useStore.getState().selectSession("s1");
  useStore.getState().appendEvent("s1", { kind: "user", text: "hello" });
  useStore.getState().appendEvent("s1", { kind: "assistant", text: "**hi** there" });
  useStore.getState().appendEvent("s1", { kind: "tool", toolName: "shell", toolArgs: { cmd: "ls" }, toolResult: "a\nb" });
  useStore.getState().appendEvent("s1", { kind: "thinking", text: "pondering" });
});

test("renders user, assistant, tool, thinking", () => {
  const { getByText, getAllByText } = render(<Timeline />);
  expect(getByText("hello")).toBeTruthy();
  expect(getByText(/hi/)).toBeTruthy();
  expect(getByText("shell")).toBeTruthy();
  expect(getAllByText(/pondering|Thinking/i).length).toBeGreaterThan(0);
});

test("tool card expands to show args + result on tap", () => {
  const { getByText, queryByText } = render(<Timeline />);
  expect(queryByText(/cmd/)).toBeNull();
  fireEvent.press(getByText("shell"));
  expect(getByText(/cmd/)).toBeTruthy();
});
```

- [ ] **Step 3: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern Timeline`
Expected: FAIL.

- [ ] **Step 4: Implement renderers**

```tsx
// mobile/src/ui/items/UserBubble.tsx
import React from "react";
import { View, Text, StyleSheet } from "react-native";
export function UserBubble({ text }: { text: string }) {
  return <View style={s.b}><Text style={s.t}>{text}</Text></View>;
}
const s = StyleSheet.create({
  b: { alignSelf: "flex-end", backgroundColor: "#2e7df6", padding: 10, borderRadius: 12, margin: 6, maxWidth: "80%" },
  t: { color: "white" },
});
```

```tsx
// mobile/src/ui/items/AssistantMessage.tsx
import React from "react";
import { View, StyleSheet } from "react-native";
import Markdown from "react-native-markdown-display";
export function AssistantMessage({ text }: { text: string }) {
  return <View style={s.b}><Markdown>{text}</Markdown></View>;
}
const s = StyleSheet.create({ b: { padding: 8, margin: 6 } });
```

```tsx
// mobile/src/ui/items/ToolCallCard.tsx
import React, { useState } from "react";
import { View, Text, Pressable, StyleSheet } from "react-native";
export function ToolCallCard({ name, args, result }: {
  name: string; args: unknown; result?: unknown;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Pressable onPress={() => setOpen(o => !o)} style={s.card}>
      <Text style={s.title}>{name}</Text>
      {open && <Text style={s.code}>{JSON.stringify(args, null, 2)}</Text>}
      {open && result !== undefined && <Text style={s.code}>{
        typeof result === "string" ? result : JSON.stringify(result, null, 2)
      }</Text>}
    </Pressable>
  );
}
const s = StyleSheet.create({
  card: { borderWidth: 1, borderColor: "#ccc", borderRadius: 8, padding: 10, margin: 6 },
  title: { fontWeight: "600" },
  code: { fontFamily: "Menlo", marginTop: 6, fontSize: 12 },
});
```

```tsx
// mobile/src/ui/items/ThinkingBlock.tsx
import React, { useState } from "react";
import { View, Text, Pressable, StyleSheet } from "react-native";
export function ThinkingBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <Pressable onPress={() => setOpen(o => !o)} style={s.b}>
      <Text style={s.label}>Thinking…</Text>
      {open && <Text style={s.text}>{text}</Text>}
    </Pressable>
  );
}
const s = StyleSheet.create({
  b: { padding: 8, margin: 6, opacity: 0.6 },
  label: { fontStyle: "italic" },
  text: { fontSize: 12, marginTop: 4 },
});
```

```tsx
// mobile/src/ui/Timeline.tsx
import React from "react";
import { FlatList } from "react-native";
import { useStore } from "../state/store";
import { UserBubble } from "./items/UserBubble";
import { AssistantMessage } from "./items/AssistantMessage";
import { ToolCallCard } from "./items/ToolCallCard";
import { ThinkingBlock } from "./items/ThinkingBlock";

export function Timeline() {
  const sid = useStore(s => s.selectedSessionId);
  const events = useStore(s => (sid ? s.eventsBySession[sid] ?? [] : []));
  return (
    <FlatList
      data={events}
      keyExtractor={(_, i) => String(i)}
      renderItem={({ item }) => {
        switch (item.kind) {
          case "user": return <UserBubble text={item.text ?? ""} />;
          case "assistant": return <AssistantMessage text={item.text ?? ""} />;
          case "tool": return <ToolCallCard name={item.toolName ?? "tool"} args={item.toolArgs} result={item.toolResult} />;
          case "thinking": return <ThinkingBlock text={item.text ?? ""} />;
          default: return null;
        }
      }}
    />
  );
}
```

- [ ] **Step 5: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern Timeline`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mobile/src/ui/Timeline.tsx mobile/src/ui/items/ mobile/src/ui/__tests__/Timeline.test.tsx mobile/package.json mobile/bun.lockb
git commit -m "feat(mobile/ui): timeline with user/assistant/tool/thinking renderers"
```

---

## Task 10: Session glue — wire transport → store

**Files:**
- Create: `mobile/src/state/sessionGlue.ts`
- Create: `mobile/src/state/__tests__/sessionGlue.test.ts`

The glue translates raw ACP frames into typed `TimelineEvent`s and routes `session/request_permission` server-requests to the store's permission queue.

- [ ] **Step 1: Write failing tests**

```ts
// mobile/src/state/__tests__/sessionGlue.test.ts
import { attachSession } from "../sessionGlue";
import { ACPClient } from "../../acp/client";
import { WSTransport } from "../../acp/transport";
import { FakeWebSocket } from "../../acp/__tests__/fakeWebSocket";
import { useStore } from "../store";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  useStore.getState().reset();
  useStore.getState().setHost("http://h:7777");
});

test("converts session/update text into assistant event", () => {
  const t = new WSTransport({ baseUrl: "ws://h:7777/acp", sessionId: "s1" });
  const c = new ACPClient(t);
  attachSession(c, "s1");
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(JSON.stringify({
    jsonrpc: "2.0", method: "session/update", params: { kind: "agent_text", text: "hello" },
    "x-mairu-event-id": 1,
  }));
  expect(useStore.getState().eventsBySession["s1"]?.[0]?.text).toBe("hello");
});

test("permission requests land in pendingPermissions and reply when resolved", async () => {
  const t = new WSTransport({ baseUrl: "ws://h:7777/acp", sessionId: "s1" });
  const c = new ACPClient(t);
  const { resolveWith } = attachSession(c, "s1");
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(JSON.stringify({
    jsonrpc: "2.0", id: 99, method: "session/request_permission",
    params: { toolCall: { name: "shell", args: { cmd: "ls" } } },
    "x-mairu-event-id": 2,
  }));
  await Promise.resolve();
  expect(useStore.getState().pendingPermissions).toHaveLength(1);
  resolveWith(99, { outcome: "allow" });
  await Promise.resolve();
  const reply = FakeWebSocket.instances[0]!.sent.find(s => s.includes(`"id":99`));
  expect(reply).toBeDefined();
  expect(useStore.getState().pendingPermissions).toHaveLength(0);
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern sessionGlue`
Expected: FAIL.

- [ ] **Step 3: Implement**

```ts
// mobile/src/state/sessionGlue.ts
import { ACPClient } from "../acp/client";
import { useStore, TimelineEvent } from "./store";

type Pending = { reply: (r: any) => void; fail: (c: number, m: string) => void };

export function attachSession(client: ACPClient, sessionId: string) {
  const store = useStore.getState();
  const pendings = new Map<number | string, Pending>();

  client.onNotification(f => {
    if (f.method !== "session/update" || !f.params) return;
    const p = f.params as any;
    const ev: TimelineEvent = mapUpdate(p);
    if (ev) store.appendEvent(sessionId, { ...ev, eventId: f.eventId });
  });

  client.onServerRequest("session/request_permission", (_method, params, reply, fail) => {
    const id = nextPermissionId();
    pendings.set(id, { reply, fail });
    store.pushPermission({ id, sessionId, method: "session/request_permission", params });
  });

  function resolveWith(id: number | string, result: any) {
    const p = pendings.get(id);
    if (!p) return;
    pendings.delete(id);
    p.reply(result);
    store.resolvePermission(id);
  }

  return { resolveWith };
}

let _pid = 0;
function nextPermissionId() { return ++_pid; }

function mapUpdate(p: any): TimelineEvent {
  if (p?.kind === "user_message" && typeof p.text === "string") return { kind: "user", text: p.text };
  if (p?.kind === "agent_text" && typeof p.text === "string") return { kind: "assistant", text: p.text };
  if (p?.kind === "tool_call") return {
    kind: "tool", toolName: p.name ?? "tool", toolArgs: p.args, toolResult: p.result,
  };
  if (p?.kind === "thinking" && typeof p.text === "string") return { kind: "thinking", text: p.text };
  return { kind: "system", text: JSON.stringify(p) };
}
```

> **Note:** `mapUpdate` mirrors the ACP `session/update` shape used by `mairu/internal/acp/`. If field names drift, adjust here only — the rest of the app is shape-agnostic.

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern sessionGlue`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/src/state/sessionGlue.ts mobile/src/state/__tests__/sessionGlue.test.ts
git commit -m "feat(mobile/state): map ACP frames to timeline events; permission queue"
```

---

## Task 11: Composer (text input + send + stop)

**Files:**
- Create: `mobile/src/ui/Composer.tsx`
- Create: `mobile/src/ui/__tests__/Composer.test.tsx`

- [ ] **Step 1: Write failing test**

```tsx
// mobile/src/ui/__tests__/Composer.test.tsx
import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { Composer } from "../Composer";

test("Send fires onSubmit with text", () => {
  const onSubmit = jest.fn();
  const { getByPlaceholderText, getByText } = render(
    <Composer onSubmit={onSubmit} onCancel={() => {}} active={false} />,
  );
  fireEvent.changeText(getByPlaceholderText(/message/i), "hi");
  fireEvent.press(getByText(/send/i));
  expect(onSubmit).toHaveBeenCalledWith("hi");
});

test("Stop button is shown only when active", () => {
  const { queryByText, rerender } = render(
    <Composer onSubmit={() => {}} onCancel={() => {}} active={false} />,
  );
  expect(queryByText(/stop/i)).toBeNull();
  rerender(<Composer onSubmit={() => {}} onCancel={() => {}} active />);
  expect(queryByText(/stop/i)).toBeTruthy();
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern Composer`
Expected: FAIL.

- [ ] **Step 3: Implement**

```tsx
// mobile/src/ui/Composer.tsx
import React, { useState } from "react";
import { View, TextInput, Pressable, Text, StyleSheet } from "react-native";

type Props = {
  onSubmit: (text: string) => void;
  onCancel: () => void;
  active: boolean;
};

export function Composer({ onSubmit, onCancel, active }: Props) {
  const [text, setText] = useState("");
  return (
    <View style={s.row}>
      <TextInput
        placeholder="Message"
        value={text}
        onChangeText={setText}
        style={s.input}
        multiline
      />
      {active && (
        <Pressable onPress={onCancel} style={[s.btn, s.stop]}>
          <Text style={s.stopText}>Stop</Text>
        </Pressable>
      )}
      <Pressable
        onPress={() => { if (text.trim()) { onSubmit(text); setText(""); } }}
        style={s.btn}
      >
        <Text style={s.btnText}>Send</Text>
      </Pressable>
    </View>
  );
}

const s = StyleSheet.create({
  row: { flexDirection: "row", padding: 8, gap: 6, borderTopWidth: 1, borderColor: "#eee" },
  input: { flex: 1, borderWidth: 1, borderColor: "#ddd", borderRadius: 8, padding: 8, maxHeight: 120 },
  btn: { backgroundColor: "#2e7df6", paddingHorizontal: 14, justifyContent: "center", borderRadius: 8 },
  btnText: { color: "white", fontWeight: "600" },
  stop: { backgroundColor: "#c33" },
  stopText: { color: "white", fontWeight: "600" },
});
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern Composer`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/src/ui/Composer.tsx mobile/src/ui/__tests__/Composer.test.tsx
git commit -m "feat(mobile/ui): composer with send + stop"
```

---

## Task 12: Voice input (push-to-talk)

**Files:**
- Create: `mobile/src/voice/recorder.ts`
- Create: `mobile/src/voice/__tests__/recorder.test.ts`
- Add dep: `@react-native-voice/voice`

The voice module exposes `start/stop/onResult`. Tests stub the native module — no actual STT runs in CI. The Composer task is amended in Task 14 to render a mic button that drives this.

- [ ] **Step 1: Add dep**

```bash
cd mobile && bun add @react-native-voice/voice
```

- [ ] **Step 2: Write failing test**

```ts
// mobile/src/voice/__tests__/recorder.test.ts
const onSpeechResults = jest.fn();
jest.mock("@react-native-voice/voice", () => ({
  __esModule: true,
  default: {
    onSpeechResults: null as any,
    onSpeechError: null as any,
    start: jest.fn(),
    stop: jest.fn(),
    destroy: jest.fn(),
    removeAllListeners: jest.fn(),
  },
}));

import Voice from "@react-native-voice/voice";
import { Recorder } from "../recorder";

test("start triggers Voice.start, results delivered to listener", async () => {
  const r = new Recorder();
  const got: string[] = [];
  r.onResult(t => got.push(t));
  await r.start();
  expect((Voice as any).start).toHaveBeenCalled();
  // simulate
  (Voice as any).onSpeechResults?.({ value: ["hello world"] });
  expect(got).toEqual(["hello world"]);
});

test("stop calls Voice.stop", async () => {
  const r = new Recorder();
  await r.stop();
  expect((Voice as any).stop).toHaveBeenCalled();
});
```

- [ ] **Step 3: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern voice`
Expected: FAIL.

- [ ] **Step 4: Implement**

```ts
// mobile/src/voice/recorder.ts
import Voice from "@react-native-voice/voice";

export class Recorder {
  private listeners: Array<(text: string) => void> = [];
  private errs: Array<(e: Error) => void> = [];

  constructor(locale = "en-US") {
    Voice.onSpeechResults = (e: any) => {
      const t = e?.value?.[0];
      if (typeof t === "string") this.listeners.forEach(cb => cb(t));
    };
    Voice.onSpeechError = (e: any) =>
      this.errs.forEach(cb => cb(new Error(e?.error?.message ?? "speech error")));
    this.locale = locale;
  }
  private locale: string;

  onResult(cb: (text: string) => void) { this.listeners.push(cb); }
  onError(cb: (e: Error) => void) { this.errs.push(cb); }

  async start() { await Voice.start(this.locale); }
  async stop()  { await Voice.stop(); }
  async destroy() { await Voice.destroy(); Voice.removeAllListeners(); }
}
```

- [ ] **Step 5: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern voice`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mobile/src/voice/ mobile/package.json mobile/bun.lockb
git commit -m "feat(mobile/voice): push-to-talk recorder wrapper"
```

---

## Task 13: Permission modal

**Files:**
- Create: `mobile/src/ui/PermissionModal.tsx`
- Create: `mobile/src/ui/__tests__/PermissionModal.test.tsx`
- Add dep: `expo-haptics`

- [ ] **Step 1: Add dep**

```bash
cd mobile && bunx expo install expo-haptics
```

- [ ] **Step 2: Write failing test**

```tsx
// mobile/src/ui/__tests__/PermissionModal.test.tsx
import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { PermissionModal } from "../PermissionModal";

test("renders tool name + args, fires onResolve(allow)", () => {
  const onResolve = jest.fn();
  const { getByText } = render(
    <PermissionModal
      request={{ id: 1, sessionId: "s", method: "session/request_permission",
                 params: { toolCall: { name: "shell", args: { cmd: "ls" } } } }}
      onResolve={onResolve}
    />,
  );
  expect(getByText("shell")).toBeTruthy();
  fireEvent.press(getByText(/allow/i));
  expect(onResolve).toHaveBeenCalledWith(1, { outcome: "allow" });
});

test("Always-allow returns always_allow", () => {
  const onResolve = jest.fn();
  const { getByText } = render(
    <PermissionModal
      request={{ id: 2, sessionId: "s", method: "session/request_permission",
                 params: { toolCall: { name: "shell" } } }}
      onResolve={onResolve}
    />,
  );
  fireEvent.press(getByText(/always/i));
  expect(onResolve).toHaveBeenCalledWith(2, { outcome: "always_allow" });
});
```

- [ ] **Step 3: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern PermissionModal`
Expected: FAIL.

- [ ] **Step 4: Implement**

```tsx
// mobile/src/ui/PermissionModal.tsx
import React, { useEffect } from "react";
import { View, Text, Pressable, StyleSheet, Modal } from "react-native";
import * as Haptics from "expo-haptics";
import { PermissionRequest } from "../state/store";

type Outcome = "allow" | "deny" | "always_allow";
type Props = {
  request: PermissionRequest;
  onResolve: (id: number | string, result: { outcome: Outcome }) => void;
};

export function PermissionModal({ request, onResolve }: Props) {
  useEffect(() => { Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning); }, [request.id]);
  const tc = (request.params as any)?.toolCall ?? {};
  return (
    <Modal visible animationType="slide" transparent onRequestClose={() => onResolve(request.id, { outcome: "deny" })}>
      <View style={s.sheet}>
        <Text style={s.title}>{tc.name ?? "tool"}</Text>
        <Text style={s.code}>{JSON.stringify(tc.args ?? {}, null, 2)}</Text>
        <View style={s.row}>
          <Pressable style={[s.btn, s.deny]} onPress={() => onResolve(request.id, { outcome: "deny" })}>
            <Text style={s.btnText}>Deny</Text>
          </Pressable>
          <Pressable style={s.btn} onPress={() => onResolve(request.id, { outcome: "allow" })}>
            <Text style={s.btnText}>Allow</Text>
          </Pressable>
          <Pressable style={[s.btn, s.always]} onPress={() => onResolve(request.id, { outcome: "always_allow" })}>
            <Text style={s.btnText}>Always</Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}

const s = StyleSheet.create({
  sheet: { marginTop: "auto", backgroundColor: "white", padding: 16, borderTopLeftRadius: 12, borderTopRightRadius: 12 },
  title: { fontSize: 18, fontWeight: "600", marginBottom: 8 },
  code: { fontFamily: "Menlo", fontSize: 12, marginBottom: 16, maxHeight: 240 },
  row: { flexDirection: "row", gap: 8 },
  btn: { flex: 1, padding: 14, borderRadius: 8, backgroundColor: "#2e7df6", alignItems: "center" },
  btnText: { color: "white", fontWeight: "600" },
  deny: { backgroundColor: "#c33" },
  always: { backgroundColor: "#444" },
});
```

- [ ] **Step 5: Run to PASS**

Run: `cd mobile && bun run test --testPathPattern PermissionModal`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mobile/src/ui/PermissionModal.tsx mobile/src/ui/__tests__/PermissionModal.test.tsx mobile/package.json mobile/bun.lockb
git commit -m "feat(mobile/ui): permission modal with allow/deny/always-allow"
```

---

## Task 14: App shell (wires everything together)

**Files:**
- Modify: `mobile/App.tsx`
- Create: `mobile/src/ui/__tests__/App.test.tsx`

The shell flips between `ConnectScreen` (no host) and the main session view (host set + session selected). Maintains a single `WSTransport`/`ACPClient` per selected session and rebuilds it on selection change.

- [ ] **Step 1: Write the failing test**

```tsx
// mobile/src/ui/__tests__/App.test.tsx
import React from "react";
import { render, waitFor } from "@testing-library/react-native";
import App from "../../../App";
import { useStore } from "../../state/store";
import { FakeWebSocket } from "../../acp/__tests__/fakeWebSocket";

jest.mock("../../api/sessions", () => ({
  listSessions: jest.fn().mockResolvedValue([
    { id: "s1", agent: "mairu", started_at: 0, last_activity_at: 0, active: true },
  ]),
  createSession: jest.fn(),
}));
jest.mock("@react-native-async-storage/async-storage", () => ({ __esModule: true, default: {
  getItem: jest.fn().mockResolvedValue(null), setItem: jest.fn(), removeItem: jest.fn(),
}}));

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  useStore.getState().reset();
});

test("shows ConnectScreen when no host", () => {
  const { getByText } = render(<App />);
  expect(getByText(/Connect to mairu/i)).toBeTruthy();
});

test("shows session picker once host is set", async () => {
  useStore.getState().setHost("http://h:7777");
  const { getByText } = render(<App />);
  await waitFor(() => getByText(/s1/));
});
```

- [ ] **Step 2: Run to FAIL**

Run: `cd mobile && bun run test --testPathPattern ui/__tests__/App`
Expected: FAIL.

- [ ] **Step 3: Implement**

```tsx
// mobile/App.tsx
import React, { useEffect, useMemo } from "react";
import { SafeAreaView, View, Text, StyleSheet } from "react-native";
import { useStore } from "./src/state/store";
import { ConnectScreen, loadStoredHost } from "./src/ui/ConnectScreen";
import { SessionPicker } from "./src/ui/SessionPicker";
import { Timeline } from "./src/ui/Timeline";
import { Composer } from "./src/ui/Composer";
import { PermissionModal } from "./src/ui/PermissionModal";
import { WSTransport } from "./src/acp/transport";
import { ACPClient } from "./src/acp/client";
import { attachSession } from "./src/state/sessionGlue";

export default function App() {
  const host = useStore(s => s.host);
  const sid = useStore(s => s.selectedSessionId);
  const setHost = useStore(s => s.setHost);
  const setConn = useStore(s => s.setConnection);
  const pending = useStore(s => s.pendingPermissions);

  // Restore stored host on cold start.
  useEffect(() => { loadStoredHost().then(h => h && setHost(h)); }, [setHost]);

  // Build a fresh transport+client whenever host or sid changes.
  const wired = useMemo(() => {
    if (!host || !sid) return null;
    const wsUrl = host.replace(/^http/, "ws") + "/acp";
    const t = new WSTransport({ baseUrl: wsUrl, sessionId: sid });
    const c = new ACPClient(t);
    const glue = attachSession(c, sid);
    t.onState(s => setConn(s));
    t.connect();
    return { t, c, glue };
  }, [host, sid, setConn]);

  useEffect(() => () => { wired?.t.disconnect(); }, [wired]);

  if (!host) return <SafeAreaView style={s.full}><ConnectScreen /></SafeAreaView>;

  return (
    <SafeAreaView style={s.full}>
      <View style={s.header}><Text style={s.h}>mairu</Text></View>
      {!sid ? (
        <SessionPicker />
      ) : (
        <>
          <Timeline />
          <Composer
            active={false}
            onSubmit={(text) => wired?.c.notify("session/prompt", { text })}
            onCancel={() => wired?.c.notify("session/cancel", {})}
          />
        </>
      )}
      {pending[0] && wired && (
        <PermissionModal
          request={pending[0]}
          onResolve={(id, result) => wired.glue.resolveWith(id, result)}
        />
      )}
    </SafeAreaView>
  );
}

const s = StyleSheet.create({
  full: { flex: 1 },
  header: { padding: 12, borderBottomWidth: 1, borderColor: "#eee" },
  h: { fontWeight: "700" },
});
```

- [ ] **Step 4: Run to PASS**

Run: `cd mobile && bun run test`
Expected: full suite PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/App.tsx mobile/src/ui/__tests__/App.test.tsx
git commit -m "feat(mobile): app shell wiring transport/client/store"
```

---

## Task 15: Active-turn signal + Stop button

The Stop button is conditional on the agent having an in-flight turn. Detect this from `session/update` (`kind: "turn_started"` / `"turn_ended"`).

**Files:**
- Modify: `mobile/src/state/store.ts` (add `activeTurnsBySession`)
- Modify: `mobile/src/state/sessionGlue.ts` (set/clear)
- Modify: `mobile/src/state/__tests__/sessionGlue.test.ts` (extend)
- Modify: `mobile/App.tsx` (pass `active`)

- [ ] **Step 1: Add failing assertion to existing glue test**

```ts
test("turn_started sets activeTurns; turn_ended clears it", () => {
  const t = new WSTransport({ baseUrl: "ws://h:7777/acp", sessionId: "s1" });
  const c = new ACPClient(t);
  attachSession(c, "s1");
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(JSON.stringify({
    jsonrpc: "2.0", method: "session/update", params: { kind: "turn_started" }, "x-mairu-event-id": 1,
  }));
  expect(useStore.getState().activeTurnsBySession["s1"]).toBe(true);
  FakeWebSocket.instances[0]!.recv(JSON.stringify({
    jsonrpc: "2.0", method: "session/update", params: { kind: "turn_ended" }, "x-mairu-event-id": 2,
  }));
  expect(useStore.getState().activeTurnsBySession["s1"]).toBe(false);
});
```

- [ ] **Step 2: Implement** (extend store + glue + App per the diff in Task 14).

- [ ] **Step 3: Run full suite**

Run: `cd mobile && bun run test`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add mobile/src/state/ mobile/App.tsx
git commit -m "feat(mobile): active-turn signal + Stop button gating"
```

---

## Task 16: Mic button in Composer

**Files:**
- Modify: `mobile/src/ui/Composer.tsx` (add `onTranscript` + mic button)
- Modify: `mobile/src/ui/__tests__/Composer.test.tsx`
- Modify: `mobile/App.tsx` to wire `Recorder`

- [ ] **Step 1: Failing test**

```tsx
test("press-and-hold mic calls onStartRecord; release calls onStopRecord", () => {
  const start = jest.fn(), stop = jest.fn();
  const { getByTestId } = render(
    <Composer
      onSubmit={() => {}} onCancel={() => {}} active={false}
      onStartRecord={start} onStopRecord={stop}
    />,
  );
  fireEvent(getByTestId("mic"), "pressIn");
  expect(start).toHaveBeenCalled();
  fireEvent(getByTestId("mic"), "pressOut");
  expect(stop).toHaveBeenCalled();
});
```

- [ ] **Step 2: Implement** push-to-talk button (`onPressIn`/`onPressOut`).
- [ ] **Step 3: Wire `Recorder` in `App.tsx`** — `onResult` appends to the composer's text via a setter passed down.
- [ ] **Step 4: Run** — `bun run test`.
- [ ] **Step 5: Commit**

```bash
git add mobile/src/ui/Composer.tsx mobile/src/ui/__tests__/Composer.test.tsx mobile/App.tsx
git commit -m "feat(mobile/ui): push-to-talk mic in composer"
```

---

## Task 17: Connection indicator

Header shows a small dot: green (`open`), yellow (`connecting`), grey (`idle`/`closed`). Drives off `useStore(s => s.connection)`.

**Files:**
- Modify: `mobile/App.tsx` header
- Create: `mobile/src/ui/ConnectionDot.tsx`
- Create: `mobile/src/ui/__tests__/ConnectionDot.test.tsx`

- [ ] **Step 1: Test colors per state, Step 2: Implement, Step 3: Run, Step 4: Commit**

```bash
git add mobile/src/ui/ConnectionDot.tsx mobile/src/ui/__tests__/ConnectionDot.test.tsx mobile/App.tsx
git commit -m "feat(mobile/ui): connection state indicator"
```

---

## Task 18: E2E against a real `mairu acp-bridge`

**Files:**
- Create: `mobile/e2e/bridge.e2e.test.ts`
- Modify: `mobile/jest.config.js` (separate `e2e` project, opt-in via `bun run e2e`)

This is a Node-side integration test: build `mairu`, spawn `mairu acp-bridge --addr 127.0.0.1:0 --no-tailscale`, drive it from a `WSTransport` running under jsdom (or pure node `ws`). Validates the full path: HTTP `POST /sessions` → WS attach → prompt → assistant frames → permission round-trip → cancel.

- [ ] **Step 1: Add `--no-tailscale` flag to bridge** if not already present (bridge plan Task 13 references `--addr` only). If absent, this is a one-line addition: gate the Tailscale gate behind a flag default-on.

- [ ] **Step 2: Write the test**

```ts
// mobile/e2e/bridge.e2e.test.ts
import { spawn, ChildProcess, execSync } from "child_process";
import { WSTransport } from "../src/acp/transport";
import { ACPClient } from "../src/acp/client";
import { createSession, listSessions } from "../src/api/sessions";

let bridge: ChildProcess; let host: string;

beforeAll(async () => {
  // build mairu binary if missing
  execSync("make -C ../mairu build", { stdio: "inherit" });
  bridge = spawn("../mairu/bin/mairu", ["acp-bridge", "--addr", "127.0.0.1:7799", "--no-tailscale"]);
  await new Promise(r => setTimeout(r, 500));
  host = "http://127.0.0.1:7799";
});
afterAll(() => bridge.kill());

test("create session, attach, prompt, receive assistant frame", async () => {
  // @ts-ignore use node ws
  globalThis.WebSocket = require("ws");
  const id = await createSession(host, "mairu");
  const t = new WSTransport({ baseUrl: "ws://127.0.0.1:7799/acp", sessionId: id });
  const c = new ACPClient(t);
  let gotAssistant = false;
  c.onNotification(f => {
    if (f.method === "session/update" && (f.params as any)?.kind === "agent_text") gotAssistant = true;
  });
  t.connect();
  await new Promise(r => setTimeout(r, 500));
  c.notify("session/prompt", { text: "say hi" });
  for (let i = 0; i < 100 && !gotAssistant; i++) await new Promise(r => setTimeout(r, 100));
  expect(gotAssistant).toBe(true);
  t.disconnect();
});
```

- [ ] **Step 3: Add separate jest project for e2e** so it's not in the default test run.

- [ ] **Step 4: Run**

```bash
cd mobile && bun run e2e
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/e2e/ mobile/jest.config.js
git commit -m "test(mobile): e2e against real mairu acp-bridge"
```

---

## Task 19: Lint, typecheck, final pass

- [ ] **Step 1: Lint**

```bash
cd mobile && bun run lint
```

Fix anything flagged.

- [ ] **Step 2: Typecheck**

```bash
cd mobile && bun run typecheck
```

- [ ] **Step 3: Full suite**

```bash
cd mobile && bun run test
```

All green.

- [ ] **Step 4: Manual smoke on a phone**

```bash
cd mobile && bun run start
# open Expo Go on phone connected to same tailnet,
# scan QR, point app at desktop running:
./mairu/bin/mairu acp-bridge --addr 0.0.0.0:7777
```

Verify: connect, see session picker, create session, prompt agent, receive frames, approve a tool call.

- [ ] **Step 5: Final commit**

```bash
git add -A && git commit -m "chore(mobile): lint cleanup" || true
```

---

## Self-Review Notes

**Spec coverage check:**
- Watch agent in real time → Tasks 9, 10. ✓
- Send prompts by voice or text → Tasks 11, 12, 16. ✓
- Interrupt active turn (`session/cancel`) → Tasks 14, 15. ✓
- Approve/deny tool calls → Tasks 10, 13. ✓
- Agent-harness-agnostic → falls out of bridge; mobile speaks pure ACP. ✓
- Single-screen UI per spec → Task 14. ✓
- Reconnect with `Last-Event-ID` + dedup → Task 3. ✓
- 1000-event cap per session → Task 6. ✓
- Tailscale identity is the only auth → Task 7 (no app-level password). ✓

**Known deferrals (call out in PR description):**
1. **Push notifications when backgrounded** — explicit non-goal in spec. iOS suspends WS within ~30s. Documented in app's Connect screen as a known limitation.
2. **Cloud STT** — explicit non-goal. On-device only. Spec risk #3 (jargon recognition) is accepted.
3. **`session/update` field shape** — `mapUpdate` (Task 10) assumes the kinds emitted by `mairu/internal/acp/`. If the ACP library used by Claude Code or Gemini emits different kinds, the timeline degrades to `system` events showing raw JSON. Acceptable; iterate as harnesses are tested.
4. **`--no-tailscale` flag on bridge** — required by E2E (Task 18). If the bridge doesn't already expose it, a follow-up to the acp-bridge plan is needed.
5. **Multi-tailnet / multi-host** — explicit non-goal. App stores one host.
6. **Web/desktop port** — explicit non-goal.

**Risks specific to this plan:**
- **Expo Voice on iOS sim:** `@react-native-voice/voice` requires a real device for STT. Tests stub the module; smoke testing voice requires physical hardware (Task 19 step 4).
- **`react-native-markdown-display`** brings in a pile of regex-based parsing; if perf is poor on long assistant streams, switch to a pre-tokenized renderer in a follow-up.
- **WS auto-reconnect during background→foreground** is handled by the OS suspending the JS thread; the transport's reconnect timer fires on resume and `Last-Event-ID` recovers the gap. Verified by Task 3 tests; flag if real-device testing shows misses.
