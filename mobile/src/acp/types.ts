export type JsonValue =
  | string
  | number
  | boolean
  | null
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
  try {
    obj = JSON.parse(raw);
  } catch {
    return null;
  }
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
