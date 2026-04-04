import { writable } from 'svelte/store';

export type AgentEvent = {
  Type: "text" | "status" | "error" | "done" | "tool_call" | "tool_result" | "log";
  Content: string;
  ToolName?: string;
  ToolArgs?: any;
  ToolResult?: any;
};

export type ToolCall = {
  id: string;
  name: string;
  args: any;
  result?: any;
  status: "running" | "completed" | "error";
};

export type Message = {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  statuses: string[];
  logs: string[];
  toolCalls: ToolCall[];
};

type SessionMessage = {
  role: "user" | "model";
  content: string;
};

export type ViewType = "chat" | "workspace" | "dashboard" | "settings";

export const activeView = writable<ViewType>("chat");
export const messages = writable<Message[]>([]);
export const isGenerating = writable<boolean>(false);
export const connectionState = writable<"connecting" | "connected" | "disconnected">("disconnected");
export const sessions = writable<string[]>([]);
export const currentSession = writable<string>("default");

let ws: WebSocket | null = null;
let currentMessageId: string | null = null;
let reconnectTimer: number | null = null;

function mapSavedRole(role: "user" | "model"): "user" | "assistant" {
  return role === "model" ? "assistant" : "user";
}

async function loadSessionMessages(sessionName: string) {
  const response = await fetch(`/api/sessions/${encodeURIComponent(sessionName)}/messages`);
  if (!response.ok) {
    throw new Error(`failed to load session: ${response.statusText}`);
  }

  const payload = await response.json() as { messages?: SessionMessage[] };
  const loaded = (payload.messages ?? []).map((msg): Message => ({
    id: crypto.randomUUID(),
    role: mapSavedRole(msg.role),
    content: msg.content,
    statuses: [],
    logs: [],
    toolCalls: []
  }));
  messages.set(loaded);
}

export async function loadSessions() {
  const response = await fetch("/api/sessions");
  if (!response.ok) {
    throw new Error(`failed to list sessions: ${response.statusText}`);
  }

  const payload = await response.json() as { sessions?: string[] };
  const available = [...new Set([...(payload.sessions ?? []), "default"])].sort();
  sessions.set(available);
}

export async function createSession(name: string) {
  const trimmed = name.trim();
  if (!trimmed) {
    throw new Error("session name cannot be empty");
  }

  const response = await fetch("/api/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: trimmed })
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error ?? "failed to create session");
  }

  await loadSessions();
  await switchSession(trimmed);
}

export async function switchSession(sessionName: string) {
  const trimmed = sessionName.trim() || "default";
  currentSession.set(trimmed);
  currentMessageId = null;
  isGenerating.set(false);
  await connectWs(trimmed, true);
}

export async function connectWs(sessionName?: string, forceReconnect = false) {
  const activeSession = sessionName?.trim() || "default";
  currentSession.set(activeSession);

  if (forceReconnect && ws) {
    ws.onclose = null;
    ws.close();
    ws = null;
  }

  if (ws && ws.readyState === WebSocket.OPEN) {
    await loadSessionMessages(activeSession);
    return;
  }

  if (reconnectTimer !== null) {
    window.clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }

  connectionState.set("connecting");

  try {
    await loadSessions();
    await loadSessionMessages(activeSession);
  } catch (err) {
    console.error("Failed to load initial session data", err);
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(`${protocol}//${window.location.host}/api/chat?session=${encodeURIComponent(activeSession)}`);

  ws.onopen = () => {
    connectionState.set("connected");
  };

  ws.onclose = () => {
    connectionState.set("disconnected");
    reconnectTimer = window.setTimeout(() => {
      void connectWs(activeSession);
    }, 3000);
  };

  ws.onmessage = (event) => {
    const data: AgentEvent = JSON.parse(event.data);

    if (data.Type === "text") {
      messages.update(msgs => {
        const lastMsg = msgs[msgs.length - 1];
        if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
          lastMsg.content += data.Content;
          return [...msgs];
        }
        return msgs;
      });
    } else if (data.Type === "status") {
      messages.update(msgs => {
        const lastMsg = msgs[msgs.length - 1];
        if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
          lastMsg.statuses = [...lastMsg.statuses, data.Content];
          return [...msgs];
        }
        return msgs;
      });
    } else if (data.Type === "tool_call") {
      messages.update(msgs => {
        const lastMsg = msgs[msgs.length - 1];
        if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
          lastMsg.toolCalls = [...lastMsg.toolCalls, {
            id: crypto.randomUUID(),
            name: data.ToolName || 'unknown',
            args: data.ToolArgs || {},
            status: "running"
          }];
          return [...msgs];
        }
        return msgs;
      });
    } else if (data.Type === "tool_result") {
      messages.update(msgs => {
        const lastMsg = msgs[msgs.length - 1];
        if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
          // Find the last tool call with matching name that is still running
          for (let i = lastMsg.toolCalls.length - 1; i >= 0; i--) {
            const tc = lastMsg.toolCalls[i];
            if (tc.name === data.ToolName && tc.status === "running") {
              tc.status = "completed";
              tc.result = data.ToolResult;
              break;
            }
          }
          return [...msgs];
        }
        return msgs;
      });
    } else if (data.Type === "done") {
      isGenerating.set(false);
      currentMessageId = null;
    } else if (data.Type === "error") {
      messages.update(msgs => [
        ...msgs, 
        { id: crypto.randomUUID(), role: "system", content: `Error: ${data.Content}`, statuses: [], logs: [], toolCalls: [] }
      ]);
      isGenerating.set(false);
      currentMessageId = null;
    } else if (data.Type === "log") {
      messages.update(msgs => {
        const lastMsg = msgs[msgs.length - 1];
        if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
          lastMsg.logs = [...lastMsg.logs, data.Content];
          return [...msgs];
        }
        return msgs;
      });
    }
  };
}

export function sendMessage(content: string) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    messages.update(msgs => [
      ...msgs, 
      { id: crypto.randomUUID(), role: "user", content, statuses: [], logs: [], toolCalls: [] }
    ]);
    
    currentMessageId = crypto.randomUUID();
    messages.update(msgs => [
      ...msgs, 
      { id: currentMessageId!, role: "assistant", content: "", statuses: [], logs: [], toolCalls: [] }
    ]);
    
    isGenerating.set(true);
    ws.send(content);
  }
}
