export function copy(text: string) {
  navigator.clipboard.writeText(text);
}

export function fmt(v: unknown): string {
  if (v === null || v === undefined) return "";
  if (typeof v === "object") return JSON.stringify(v, null, 2);
  return String(v);
}

export function fmtDate(s: string): string {
  if (!s) return "";
  return new Date(s).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function scoreColor(s: number): string {
  if (s >= 0.8) return "#22c55e";
  if (s >= 0.6) return "#f59e0b";
  if (s >= 0.4) return "#f97316";
  return "#ef4444";
}

export function impColor(n: number): string {
  if (n >= 8) return "imp-high";
  if (n >= 5) return "imp-med";
  return "imp-low";
}

export const categoryColors: Record<string, string> = {
  profile: "#6366f1",
  preferences: "#8b5cf6",
  entities: "#0ea5e9",
  events: "#14b8a6",
  cases: "#f59e0b",
  patterns: "#f97316",
  observation: "#64748b",
  reflection: "#a855f7",
  decision: "#ef4444",
  constraint: "#dc2626",
  architecture: "#2563eb",
};
