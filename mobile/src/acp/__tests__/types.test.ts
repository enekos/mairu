import { isResponse, isServerRequest, isNotification, parseFrame, encodeFrame } from "../types";

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
    expect(f).not.toBeNull();
    expect(isServerRequest(f!)).toBe(true);
    expect(isResponse(f!)).toBe(false);
  });

  test("parses a notification (no id)", () => {
    const raw = `{"jsonrpc":"2.0","method":"session/update","params":{"kind":"agent_text","text":"hi"}}`;
    const f = parseFrame(raw);
    expect(f).not.toBeNull();
    expect(isNotification(f!)).toBe(true);
  });

  test("rejects malformed JSON", () => {
    expect(parseFrame("not json")).toBeNull();
  });

  test("rejects non-jsonrpc payloads", () => {
    expect(parseFrame(`{"hello":"world"}`)).toBeNull();
  });

  test("ignores non-numeric x-mairu-event-id", () => {
    const f = parseFrame(`{"jsonrpc":"2.0","method":"x","x-mairu-event-id":"oops"}`);
    expect(f).not.toBeNull();
    expect(f!.eventId).toBeUndefined();
  });
});

describe("encodeFrame", () => {
  test("emits jsonrpc 2.0 envelope", () => {
    const s = encodeFrame({ id: 5, method: "session/prompt", params: { text: "hi" } });
    const obj = JSON.parse(s);
    expect(obj.jsonrpc).toBe("2.0");
    expect(obj.id).toBe(5);
    expect(obj.method).toBe("session/prompt");
    expect(obj.params).toEqual({ text: "hi" });
  });
});
