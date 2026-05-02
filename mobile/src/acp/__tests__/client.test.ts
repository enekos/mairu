import { ACPClient } from "../client";
import { WSTransport } from "../transport";
import { FakeWebSocket } from "../testing/fakeWebSocket";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
});

test("request resolves on matching id", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  const p = c.request<{ ok: boolean }>("session/prompt", { text: "hi" });
  const sent = JSON.parse(FakeWebSocket.instances[0]!.sent[0]!);
  FakeWebSocket.instances[0]!.recv(
    JSON.stringify({ jsonrpc: "2.0", id: sent.id, result: { ok: true } }),
  );
  await expect(p).resolves.toEqual({ ok: true });
});

test("request rejects on error response", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  const p = c.request("session/prompt", {});
  const sent = JSON.parse(FakeWebSocket.instances[0]!.sent[0]!);
  FakeWebSocket.instances[0]!.recv(
    JSON.stringify({
      jsonrpc: "2.0",
      id: sent.id,
      error: { code: -32000, message: "nope" },
    }),
  );
  await expect(p).rejects.toThrow("nope");
});

test("server-initiated request fires handler with reply", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  c.onServerRequest("session/request_permission", async (_m, _p, reply) => reply({ outcome: "allow" }));
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(
    `{"jsonrpc":"2.0","id":99,"method":"session/request_permission","params":{"toolCall":{"name":"shell"}}}`,
  );
  await Promise.resolve();
  await Promise.resolve();
  const reply = FakeWebSocket.instances[0]!.sent.find((s) => s.includes(`"id":99`));
  expect(reply).toBeDefined();
  expect(JSON.parse(reply!).result).toEqual({ outcome: "allow" });
});

test("missing handler replies with method-not-found", async () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  new ACPClient(t);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(
    `{"jsonrpc":"2.0","id":7,"method":"unknown/method","params":{}}`,
  );
  await Promise.resolve();
  const reply = JSON.parse(FakeWebSocket.instances[0]!.sent[0]!);
  expect(reply.error.code).toBe(-32601);
});

test("notification reaches onNotification", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  const got: any[] = [];
  c.onNotification((f) => got.push(f));
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(
    `{"jsonrpc":"2.0","method":"session/update","params":{"kind":"agent_text","text":"hi"}}`,
  );
  expect(got).toHaveLength(1);
  expect(got[0].method).toBe("session/update");
});

test("notify sends a JSON-RPC notification (no id)", () => {
  const t = new WSTransport({ baseUrl: "ws://x/acp" });
  const c = new ACPClient(t);
  t.connect();
  FakeWebSocket.instances[0]!.open();
  c.notify("session/cancel", {});
  const sent = JSON.parse(FakeWebSocket.instances[0]!.sent[0]!);
  expect(sent.id).toBeUndefined();
  expect(sent.method).toBe("session/cancel");
});
