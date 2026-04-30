import { ACPClient } from "../acp/client";
import { useStore, TimelineEvent } from "./store";

type Pending = {
  reply: (r: any) => void;
  fail: (c: number, m: string) => void;
};

let _pid = 0;
function nextPermissionId(): number {
  return ++_pid;
}

export function attachSession(client: ACPClient, sessionId: string) {
  const store = useStore.getState();
  const pendings = new Map<number | string, Pending>();

  client.onNotification((f) => {
    if (f.method !== "session/update" || !f.params) return;
    const ev = mapUpdate(f.params);
    if (!ev) return;
    store.appendEvent(sessionId, { ...ev, eventId: f.eventId });
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

function mapUpdate(p: any): TimelineEvent | null {
  if (p?.kind === "user_message" && typeof p.text === "string") {
    return { kind: "user", text: p.text };
  }
  if (p?.kind === "agent_text" && typeof p.text === "string") {
    return { kind: "assistant", text: p.text };
  }
  if (p?.kind === "tool_call") {
    return {
      kind: "tool",
      toolName: p.name ?? "tool",
      toolArgs: p.args,
      toolResult: p.result,
    };
  }
  if (p?.kind === "thinking" && typeof p.text === "string") {
    return { kind: "thinking", text: p.text };
  }
  return { kind: "system", text: JSON.stringify(p) };
}
