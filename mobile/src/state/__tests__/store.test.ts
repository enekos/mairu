import { useStore } from "../store";

beforeEach(() => useStore.getState().reset());

test("appendEvent caps per-session at 1000", () => {
  for (let i = 0; i < 1100; i++) {
    useStore.getState().appendEvent("sess", { kind: "user", text: String(i) });
  }
  const arr = useStore.getState().eventsBySession["sess"];
  expect(arr).toHaveLength(1000);
  expect(arr![0]!.text).toBe("100"); // oldest 100 dropped
});

test("setHost and selectSession persist independently", () => {
  useStore.getState().setHost("http://x:7777");
  useStore.getState().selectSession("s1");
  expect(useStore.getState().host).toBe("http://x:7777");
  expect(useStore.getState().selectedSessionId).toBe("s1");
});

test("permission requests are tracked and cleared", () => {
  useStore.getState().pushPermission({
    id: 1,
    sessionId: "s",
    method: "session/request_permission",
    params: {},
  });
  expect(useStore.getState().pendingPermissions).toHaveLength(1);
  useStore.getState().resolvePermission(1);
  expect(useStore.getState().pendingPermissions).toHaveLength(0);
});

test("setConnection updates connection state", () => {
  useStore.getState().setConnection("open");
  expect(useStore.getState().connection).toBe("open");
});

test("reset returns to initial state", () => {
  useStore.getState().setHost("http://x");
  useStore.getState().appendEvent("s", { kind: "user", text: "x" });
  useStore.getState().reset();
  expect(useStore.getState().host).toBeNull();
  expect(useStore.getState().eventsBySession).toEqual({});
});
