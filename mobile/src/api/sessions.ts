export type SessionInfo = {
  id: string;
  agent: string;
  project?: string;
  started_at: number;
  last_activity_at: number;
  active: boolean;
};

async function ok<T>(r: Response): Promise<T> {
  if (!r.ok) {
    const body = typeof r.text === "function" ? await r.text() : "";
    throw new Error(`http ${r.status}${body ? `: ${body}` : ""}`);
  }
  return (await r.json()) as T;
}

export async function listSessions(host: string): Promise<SessionInfo[]> {
  return ok<SessionInfo[]>(await fetch(`${host}/sessions`));
}

export async function createSession(
  host: string,
  agent: string,
  project?: string,
): Promise<string> {
  const body: Record<string, string> = { agent };
  if (project !== undefined) body.project = project;
  const r = await fetch(`${host}/sessions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  const { id } = await ok<{ id: string }>(r);
  return id;
}

export async function deleteSession(host: string, id: string): Promise<void> {
  const r = await fetch(`${host}/sessions/${id}`, { method: "DELETE" });
  if (!r.ok) throw new Error(`http ${r.status}`);
}
