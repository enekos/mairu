import { WSTransport } from "../transport";
import { FakeWebSocket } from "../testing/fakeWebSocket";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  jest.useFakeTimers();
});
afterEach(() => jest.useRealTimers());

test("delivers frames in order with monotonic event ids", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const got: number[] = [];
  t.onFrame((f) => {
    if (f.eventId) got.push(f.eventId);
  });
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","id":1,"result":{},"x-mairu-event-id":1}`);
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","id":2,"result":{},"x-mairu-event-id":2}`);
  expect(got).toEqual([1, 2]);
});

test("reconnect appends last_event_id and dedupes replayed frames", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const ids: number[] = [];
  t.onFrame((f) => {
    if (f.eventId) ids.push(f.eventId);
  });
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":1}`);
  FakeWebSocket.instances[0]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":2}`);
  FakeWebSocket.instances[0]!.fail();
  jest.advanceTimersByTime(60_000);
  expect(FakeWebSocket.instances.length).toBeGreaterThanOrEqual(2);
  FakeWebSocket.instances[1]!.open();
  expect(FakeWebSocket.instances[1]!.url).toContain("last_event_id=2");
  // server replays 2 (already seen) and a new 3
  FakeWebSocket.instances[1]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":2}`);
  FakeWebSocket.instances[1]!.recv(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":3}`);
  expect(ids).toEqual([1, 2, 3]); // 2 deduped on reconnect
});

test("send queues until open", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  t.connect();
  t.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`);
  expect(FakeWebSocket.instances[0]!.sent).toEqual([]);
  FakeWebSocket.instances[0]!.open();
  expect(FakeWebSocket.instances[0]!.sent[0]).toContain(`"method":"ping"`);
});

test("backoff schedules many retries without giving up", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  t.connect();
  for (let i = 0; i < 5; i++) {
    FakeWebSocket.instances[i]!.fail();
    jest.advanceTimersByTime(60_000);
  }
  expect(FakeWebSocket.instances.length).toBeGreaterThanOrEqual(6);
});

test("disconnect stops reconnect attempts", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  t.connect();
  FakeWebSocket.instances[0]!.open();
  t.disconnect();
  FakeWebSocket.instances[0]!.fail();
  jest.advanceTimersByTime(60_000);
  expect(FakeWebSocket.instances.length).toBe(1);
});

test("emits state transitions", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const states: string[] = [];
  t.onState((s) => states.push(s));
  t.connect();
  FakeWebSocket.instances[0]!.open();
  expect(states).toContain("connecting");
  expect(states).toContain("open");
});
