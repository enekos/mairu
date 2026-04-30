import { attachSession } from "../sessionGlue";
import { ACPClient } from "../../acp/client";
import { WSTransport } from "../../acp/transport";
import { FakeWebSocket } from "../../acp/testing/fakeWebSocket";
import { useStore } from "../store";

beforeEach(() => {
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  useStore.getState().reset();
  useStore.getState().setHost("http://h:7777");
});

test("converts session/update agent_text into assistant event", () => {
  const t = new WSTransport({ baseUrl: "ws://h:7777/acp", sessionId: "s1" });
  const c = new ACPClient(t);
  attachSession(c, "s1");
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(
    JSON.stringify({
      jsonrpc: "2.0",
      method: "session/update",
      params: { kind: "agent_text", text: "hello" },
      "x-mairu-event-id": 1,
    }),
  );
  const arr = useStore.getState().eventsBySession["s1"]!;
  expect(arr).toHaveLength(1);
  expect(arr[0]!.kind).toBe("assistant");
  expect(arr[0]!.text).toBe("hello");
});

test("converts user_message into user event", () => {
  const t = new WSTransport({ baseUrl: "ws://h:7777/acp", sessionId: "s1" });
  const c = new ACPClient(t);
  attachSession(c, "s1");
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(
    JSON.stringify({
      jsonrpc: "2.0",
      method: "session/update",
      params: { kind: "user_message", text: "hi from me" },
      "x-mairu-event-id": 1,
    }),
  );
  expect(useStore.getState().eventsBySession["s1"]![0]!.kind).toBe("user");
});

test("permission requests land in pendingPermissions and reply when resolved", async () => {
  const t = new WSTransport({ baseUrl: "ws://h:7777/acp", sessionId: "s1" });
  const c = new ACPClient(t);
  const { resolveWith } = attachSession(c, "s1");
  t.connect();
  FakeWebSocket.instances[0]!.open();
  FakeWebSocket.instances[0]!.recv(
    JSON.stringify({
      jsonrpc: "2.0",
      id: 99,
      method: "session/request_permission",
      params: { toolCall: { name: "shell", args: { cmd: "ls" } } },
      "x-mairu-event-id": 2,
    }),
  );
  await Promise.resolve();
  await Promise.resolve();
  const pending = useStore.getState().pendingPermissions;
  expect(pending).toHaveLength(1);
  resolveWith(pending[0]!.id, { outcome: "allow" });
  await Promise.resolve();
  const reply = FakeWebSocket.instances[0]!.sent.find((s) => s.includes(`"id":99`));
  expect(reply).toBeDefined();
  expect(useStore.getState().pendingPermissions).toHaveLength(0);
});
