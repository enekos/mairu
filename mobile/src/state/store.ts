import { create } from "zustand";

export type EventKind = "user" | "assistant" | "tool" | "thinking" | "system";
export type TimelineEvent = {
  kind: EventKind;
  text?: string;
  toolName?: string;
  toolArgs?: unknown;
  toolResult?: unknown;
  ts?: number;
  eventId?: number;
};

export type ConnectionState = "idle" | "connecting" | "open" | "closed";

export type PermissionRequest = {
  id: number | string;
  sessionId: string;
  method: string;
  params: any;
};

const CAP = 1000;

type StoreState = {
  host: string | null;
  connection: ConnectionState;
  selectedSessionId: string | null;
  eventsBySession: Record<string, TimelineEvent[]>;
  pendingPermissions: PermissionRequest[];
  activeTurnsBySession: Record<string, boolean>;
};

type StoreActions = {
  setHost: (h: string | null) => void;
  setConnection: (c: ConnectionState) => void;
  selectSession: (id: string | null) => void;
  appendEvent: (sessionId: string, ev: TimelineEvent) => void;
  pushPermission: (p: PermissionRequest) => void;
  resolvePermission: (id: number | string) => void;
  setActiveTurn: (sessionId: string, active: boolean) => void;
  reset: () => void;
};

const initial: StoreState = {
  host: null,
  connection: "idle",
  selectedSessionId: null,
  eventsBySession: {},
  pendingPermissions: [],
  activeTurnsBySession: {},
};

export const useStore = create<StoreState & StoreActions>((set) => ({
  ...initial,
  setHost: (h) => set({ host: h }),
  setConnection: (c) => set({ connection: c }),
  selectSession: (id) => set({ selectedSessionId: id }),
  appendEvent: (sid, ev) =>
    set((st) => {
      const cur = st.eventsBySession[sid] ?? [];
      const next =
        cur.length >= CAP ? [...cur.slice(cur.length - CAP + 1), ev] : [...cur, ev];
      return { eventsBySession: { ...st.eventsBySession, [sid]: next } };
    }),
  pushPermission: (p) =>
    set((st) => ({ pendingPermissions: [...st.pendingPermissions, p] })),
  resolvePermission: (id) =>
    set((st) => ({
      pendingPermissions: st.pendingPermissions.filter((p) => p.id !== id),
    })),
  setActiveTurn: (sid, active) =>
    set((st) => ({
      activeTurnsBySession: { ...st.activeTurnsBySession, [sid]: active },
    })),
  reset: () =>
    set({
      host: initial.host,
      connection: initial.connection,
      selectedSessionId: initial.selectedSessionId,
      eventsBySession: {},
      pendingPermissions: [],
      activeTurnsBySession: {},
    }),
}));
