import { writable } from 'svelte/store';

const isWails = typeof window !== 'undefined' && !!window.go?.desktop?.App;
let EventsOn: any = () => {};
if (isWails) {
  const runtime = window.runtime;
  EventsOn = runtime?.EventsOn ?? (() => {});
}

export type AgentEvent = {
  Type: "text" | "status" | "error" | "done" | "tool_call" | "tool_result" | "log" | "bash_output" | "approval_request";
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
  bashOutput: string;
  statuses: string[];
  logs: string[];
  toolCalls: ToolCall[];
};

type SavedPart = {
  type: string;
  text?: string;
  func_name?: string;
  func_args?: any;
  func_resp?: any;
};

type SessionMessage = {
  role: "user" | "model";
  content?: string;
  parts?: SavedPart[];
};

export type ViewType = "chat" | "workspace" | "dashboard" | "logs" | "settings";

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
  let payload: { messages?: SessionMessage[] } = { messages: [] };

  if (isWails) {
    try {
      const msgs = await (window as any).go.desktop.App.LoadSessionHistory(sessionName);
      payload.messages = msgs;
    } catch {
      // ignore Wails session history load error
    }
  } else {
    const response = await fetch(`/api/sessions/${encodeURIComponent(sessionName)}/messages`);
    if (!response.ok) {
      throw new Error(`failed to load session: ${response.statusText}`);
    }
    payload = await response.json() as { messages?: SessionMessage[] };
  }
  
  const loaded: Message[] = [];
  
  for (const msg of payload.messages ?? []) {
    if (msg.role === "user") {
      const textParts = msg.parts?.filter(p => p.type === "text").map(p => p.text).join("") || msg.content || "";
      const funcRespParts = msg.parts?.filter(p => p.type === "function_response") || [];
      
      if (textParts.length > 0) {
        loaded.push({
          id: crypto.randomUUID(),
          role: "user",
          content: textParts,
          bashOutput: "",
          statuses: [], logs: [], toolCalls: []
        });
      }
      
      for (const resp of funcRespParts) {
        for (let i = loaded.length - 1; i >= 0; i--) {
          if (loaded[i].role === "assistant") {
            const tc = loaded[i].toolCalls.find(t => t.name === resp.func_name && t.status === "running");
            if (tc) {
              tc.result = resp.func_resp;
              tc.status = "completed";
              break;
            } else {
              const tcCompleted = loaded[i].toolCalls.find(t => t.name === resp.func_name && !t.result);
              if (tcCompleted) {
                tcCompleted.result = resp.func_resp;
              }
            }
          }
        }
      }
    } else if (msg.role === "model") {
      const textParts = msg.parts?.filter(p => p.type === "text").map(p => p.text).join("") || msg.content || "";
      const funcCallParts = msg.parts?.filter(p => p.type === "function_call") || [];
      
      const toolCalls: ToolCall[] = funcCallParts.map(fc => ({
        id: crypto.randomUUID(),
        name: fc.func_name || "unknown",
        args: fc.func_args || {},
        status: "running"
      }));
      
      if (textParts.length > 0 || toolCalls.length > 0) {
        loaded.push({
          id: crypto.randomUUID(),
          role: "assistant",
          content: textParts,
          bashOutput: "",
          statuses: [], logs: [], toolCalls: toolCalls
        });
      }
    }
  }
  
  for (const msg of loaded) {
    for (const tc of msg.toolCalls) {
      if (tc.status === "running") {
        tc.status = "completed";
      }
    }
  }

  messages.set(loaded);
}

export async function loadSessions() {
  if (isWails) {
    try {
      const sessList = await (window as any).go.desktop.App.ListSessions();
      const available = [...new Set([...(sessList ?? []), "default"])].sort();
      sessions.set(available);
    } catch {
      sessions.set(["default"]);
    }
    return;
  }
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

  if (isWails) {
    try {
      await (window as any).go.desktop.App.CreateSession(trimmed);
      await loadSessions();
      await switchSession(trimmed);
      return;
    } catch(e) {
      throw new Error(typeof e === "string" ? e : "failed to create session");
    }
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
  await connectChat(trimmed, true);
}

function handleAgentEvent(data: AgentEvent) {
  if (data.Type === "text") {
    messages.update(msgs => {
      const lastMsg = msgs[msgs.length - 1];
      if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
        lastMsg.bashOutput = "";
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
        lastMsg.bashOutput = "";
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
    messages.update(msgs => {
      const lastMsg = msgs[msgs.length - 1];
      if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
        lastMsg.bashOutput = "";
      }
      return msgs;
    });
    isGenerating.set(false);
    currentMessageId = null;
  } else if (data.Type === "approval_request") {
    messages.update(msgs => [
      ...msgs, 
      { id: crypto.randomUUID(), role: "system", content: data.Content, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
    ]);
  } else if (data.Type === "error") {
    messages.update(msgs => [
      ...msgs, 
      { id: crypto.randomUUID(), role: "system", content: `Error: ${data.Content}`, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
    ]);
    isGenerating.set(false);
    currentMessageId = null;
  } else if (data.Type === "bash_output") {
    messages.update(msgs => {
      const lastMsg = msgs[msgs.length - 1];
      if (lastMsg && lastMsg.role === "assistant" && lastMsg.id === currentMessageId) {
        lastMsg.bashOutput += data.Content;
        if (lastMsg.bashOutput.length > 4000) {
          lastMsg.bashOutput = lastMsg.bashOutput.slice(lastMsg.bashOutput.length - 4000);
        }
        return [...msgs];
      }
      return msgs;
    });
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
}

export async function connectChat(sessionName?: string, forceReconnect = false) {
  const activeSession = sessionName?.trim() || "default";
  currentSession.set(activeSession);

  if (isWails) {
    connectWailsChat(activeSession);
    return;
  }

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
  } catch {
    // ignore initial session load error
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(`${protocol}//${window.location.host}/api/chat?session=${encodeURIComponent(activeSession)}`);

  ws.onopen = () => {
    connectionState.set("connected");
  };

  ws.onclose = () => {
    connectionState.set("disconnected");
    reconnectTimer = window.setTimeout(() => {
      void connectChat(activeSession);
    }, 3000);
  };

  ws.onmessage = (event) => {
    const data: AgentEvent = JSON.parse(event.data);
    handleAgentEvent(data);
  };
}

function connectWailsChat(session: string) {
  connectionState.set("connecting");
  
  loadSessions().then(() => loadSessionMessages(session)).catch(() => {
    // ignore initial session load error
  });

  connectionState.set("connected");

  EventsOn("chat:message", (ev: any) => {
    handleAgentEvent(ev);
  });

  EventsOn("chat:error", (err: string) => {
    messages.update(m => [...m, {
      id: crypto.randomUUID(),
      role: "system" as const,
      content: `Error: ${err}`,
      bashOutput: "",
      statuses: [],
      logs: [],
      toolCalls: [],
    }]);
    isGenerating.set(false);
  });

  EventsOn("chat:done", () => {
    isGenerating.set(false);
  });
}

export function sendMessage(content: string) {
  messages.update(msgs => [
    ...msgs, 
    { id: crypto.randomUUID(), role: "user", content, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
  ]);
  
  currentMessageId = crypto.randomUUID();
  messages.update(msgs => [
    ...msgs, 
    { id: currentMessageId!, role: "assistant", content: "", bashOutput: "", statuses: [], logs: [], toolCalls: [] }
  ]);
  
  isGenerating.set(true);

  if (isWails) {
    let session = "default";
    currentSession.subscribe(v => session = v)();
    window.go?.desktop?.App?.SendMessage(session, content);
  } else if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(content);
  } else {
    isGenerating.set(false);
  }
}
