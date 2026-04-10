// Unified API layer — routes to Wails bindings in desktop mode, fetch() in web mode.
// Wails auto-generates types in ../wailsjs/ at build time.

const isWails = typeof window !== 'undefined' && !!window.go?.desktop?.App;

function getWailsApp(): any {
  return window.go?.desktop?.App;
}

async function fetchJSON(url: string, init?: RequestInit): Promise<any> {
  const resp = await fetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  });
  if (!resp.ok) throw new Error(`API error: ${resp.status}`);
  return resp.json();
}

// ── Memories ────────────────────────────────────────────────────

export async function listMemories(project: string, limit = 100) {
  if (isWails) return getWailsApp().ListMemories(project, limit);
  return fetchJSON(`/api/memories?project=${encodeURIComponent(project)}&limit=${limit}`);
}

export async function createMemory(input: any) {
  if (isWails) return getWailsApp().CreateMemory(input);
  return fetchJSON('/api/memories', { method: 'POST', body: JSON.stringify(input) });
}

export async function updateMemory(input: any) {
  if (isWails) return getWailsApp().UpdateMemory(input);
  return fetchJSON('/api/memories', { method: 'PUT', body: JSON.stringify(input) });
}

export async function deleteMemory(id: string) {
  if (isWails) return getWailsApp().DeleteMemory(id);
  return fetchJSON(`/api/memories?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function applyMemoryFeedback(id: string, reward: number) {
  if (isWails) return getWailsApp().ApplyMemoryFeedback(id, reward);
  return fetchJSON('/api/memories/feedback', { method: 'POST', body: JSON.stringify({ id, reward }) });
}

// ── Skills ──────────────────────────────────────────────────────

export async function listSkills(project: string, limit = 100) {
  if (isWails) return getWailsApp().ListSkills(project, limit);
  return fetchJSON(`/api/skills?project=${encodeURIComponent(project)}&limit=${limit}`);
}

export async function createSkill(input: any) {
  if (isWails) return getWailsApp().CreateSkill(input);
  return fetchJSON('/api/skills', { method: 'POST', body: JSON.stringify(input) });
}

export async function updateSkill(input: any) {
  if (isWails) return getWailsApp().UpdateSkill(input);
  return fetchJSON('/api/skills', { method: 'PUT', body: JSON.stringify(input) });
}

export async function deleteSkill(id: string) {
  if (isWails) return getWailsApp().DeleteSkill(id);
  return fetchJSON(`/api/skills?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
}

// ── Context Nodes ───────────────────────────────────────────────

export async function listContextNodes(project: string, parentURI?: string, limit = 100) {
  if (isWails) return getWailsApp().ListContextNodes(project, parentURI ?? null, limit);
  let url = `/api/context?project=${encodeURIComponent(project)}&limit=${limit}`;
  if (parentURI) url += `&parentUri=${encodeURIComponent(parentURI)}`;
  return fetchJSON(url);
}

export async function createContextNode(input: any) {
  if (isWails) return getWailsApp().CreateContextNode(input);
  return fetchJSON('/api/context', { method: 'POST', body: JSON.stringify(input) });
}

export async function updateContextNode(input: any) {
  if (isWails) return getWailsApp().UpdateContextNode(input);
  return fetchJSON('/api/context', { method: 'PUT', body: JSON.stringify(input) });
}

export async function deleteContextNode(uri: string) {
  if (isWails) return getWailsApp().DeleteContextNode(uri);
  return fetchJSON(`/api/context?uri=${encodeURIComponent(uri)}`, { method: 'DELETE' });
}

// ── Search ──────────────────────────────────────────────────────

export async function search(opts: any) {
  if (isWails) return getWailsApp().Search(opts);
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(opts)) {
    if (v !== undefined && v !== null && v !== '') params.set(k, String(v));
  }
  return fetchJSON(`/api/search?${params}`);
}

// ── Dashboard ───────────────────────────────────────────────────

export async function dashboard(limit = 1000, project = '') {
  if (isWails) return getWailsApp().Dashboard(limit, project);
  return fetchJSON(`/api/dashboard?limit=${limit}&project=${encodeURIComponent(project)}`);
}

export async function health() {
  if (isWails) return getWailsApp().Health();
  return fetchJSON('/api/health');
}

export async function clusterStats() {
  if (isWails) return getWailsApp().ClusterStats();
  return fetchJSON('/api/cluster');
}

// ── Vibe ────────────────────────────────────────────────────────

export async function vibeQuery(prompt: string, project: string, topK: number) {
  if (isWails) return getWailsApp().VibeQuery(prompt, project, topK);
  return fetchJSON('/api/vibe/query', { method: 'POST', body: JSON.stringify({ prompt, project, topK }) });
}

export async function vibeMutationPlan(prompt: string, project: string, topK: number) {
  if (isWails) return getWailsApp().VibeMutationPlan(prompt, project, topK);
  return fetchJSON('/api/vibe/mutation/plan', { method: 'POST', body: JSON.stringify({ prompt, project, topK }) });
}

export async function vibeMutationExecute(operations: any[], project: string) {
  if (isWails) return getWailsApp().VibeMutationExecute(operations, project);
  return fetchJSON('/api/vibe/mutation/execute', { method: 'POST', body: JSON.stringify({ operations, project }) });
}

// ── Moderation ──────────────────────────────────────────────────

export async function listModerationQueue(limit = 100) {
  if (isWails) return getWailsApp().ListModerationQueue(limit);
  return fetchJSON(`/api/moderation/queue?limit=${limit}`);
}

export async function reviewModeration(input: any) {
  if (isWails) return getWailsApp().ReviewModeration(input);
  return fetchJSON('/api/moderation/review', { method: 'POST', body: JSON.stringify(input) });
}

// ── Sessions (chat) ─────────────────────────────────────────────

export async function listSessions() {
  if (isWails) return getWailsApp().ListSessions();
  return fetchJSON('/api/sessions').then((r: any) => r.sessions);
}

export async function createSession(name: string) {
  if (isWails) return getWailsApp().CreateSession(name);
  return fetchJSON('/api/sessions', { method: 'POST', body: JSON.stringify({ name }) });
}

export async function loadSessionHistory(session: string) {
  if (isWails) return getWailsApp().LoadSessionHistory(session);
  return fetchJSON(`/api/sessions/${encodeURIComponent(session)}/messages`).then((r: any) => r.messages);
}
