import { listSessions, createSession, deleteSession } from "../sessions";

beforeEach(() => {
  (globalThis as any).fetch = jest.fn();
});

test("listSessions GETs /sessions", async () => {
  (fetch as jest.Mock).mockResolvedValue({
    ok: true,
    json: async () => [
      { id: "s1", agent: "mairu", active: true, started_at: 0, last_activity_at: 0 },
    ],
  });
  const out = await listSessions("http://h:7777");
  expect(fetch).toHaveBeenCalledWith("http://h:7777/sessions");
  expect(out).toHaveLength(1);
  expect(out[0]?.id).toBe("s1");
});

test("createSession POSTs with agent + project", async () => {
  (fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => ({ id: "s2" }) });
  const id = await createSession("http://h:7777", "mairu", "myproj");
  const args = (fetch as jest.Mock).mock.calls[0]!;
  expect(args[0]).toBe("http://h:7777/sessions");
  expect(JSON.parse(args[1].body)).toEqual({ agent: "mairu", project: "myproj" });
  expect(id).toBe("s2");
});

test("createSession omits project when not given", async () => {
  (fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => ({ id: "s3" }) });
  await createSession("http://h:7777", "claude-code");
  const args = (fetch as jest.Mock).mock.calls[0]!;
  expect(JSON.parse(args[1].body)).toEqual({ agent: "claude-code" });
});

test("deleteSession DELETEs /sessions/:id", async () => {
  (fetch as jest.Mock).mockResolvedValue({ ok: true });
  await deleteSession("http://h:7777", "s1");
  expect(fetch).toHaveBeenCalledWith("http://h:7777/sessions/s1", { method: "DELETE" });
});

test("listSessions throws on non-ok", async () => {
  (fetch as jest.Mock).mockResolvedValue({
    ok: false,
    status: 500,
    text: async () => "boom",
  });
  await expect(listSessions("http://h:7777")).rejects.toThrow(/500/);
});
