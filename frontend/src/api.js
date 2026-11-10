const API_BASE = "/api";

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });

  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {}

  if (!res.ok) {
    throw new Error(data?.error || data?.detail || text || `HTTP ${res.status}`);
  }
  return data;
}

export const signup = (email, password) => request("/auth/signup", { method: "POST", body: JSON.stringify({ email, password }) });
export const login = (email, password) => request("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
export const me = () => request("/auth/me", { method: "GET" });
export const logout = () => request("/auth/logout", { method: "POST" });
export const requestMagicLink = (email) => request("/auth/magic/request", { method: "POST", body: JSON.stringify({ email }) });
export const consumeMagicLink = (token) => request("/auth/magic/consume", { method: "POST", body: JSON.stringify({ token }) });
export const requestPasswordReset = (email) => request("/auth/password/reset/request", { method: "POST", body: JSON.stringify({ email }) });
export const confirmPasswordReset = (token, password) => request("/auth/password/reset/confirm", { method: "POST", body: JSON.stringify({ token, password }) });

export const listJobs = () => request("/jobs", { method: "GET" });
export const createJob = (name, intervalMinutes, graceMinutes, projectId = "", cronExpr = "") => request("/jobs", { method: "POST", body: JSON.stringify({ name, interval_minutes: cronExpr ? undefined : intervalMinutes, grace_minutes: graceMinutes, project_id: projectId || undefined, cron_expr: cronExpr || undefined }) });
export const updateJob = (id, payload) => request(`/jobs/${id}`, { method: "PATCH", body: JSON.stringify(payload) });
export const deleteJob = (id) => request(`/jobs/${id}`, { method: "DELETE" });
export const getJob = (id) => request(`/jobs/${id}`);
export const listJobRuns = (id, limit = 50) => request(`/jobs/${id}/runs?limit=${encodeURIComponent(limit)}`);
export const getJobStats = (id) => request(`/jobs/${id}/stats`);

export const getAccount = () => request("/account", { method: "GET" });
export const updateAccount = (payload) => request("/account", { method: "PATCH", body: JSON.stringify(payload) });
export const logoutAllDevices = () => request("/account/logout-all", { method: "POST" });

// Agent token: authenticates the runguard CLI's `register` subcommand
// (Authorization: Bearer <token> against /api/agent/register). Generated
// lazily on first GET; regenerating invalidates the old token immediately.
export const getAgentToken = () => request("/account/agent-token", { method: "GET" });
export const regenerateAgentToken = () => request("/account/agent-token/regenerate", { method: "POST" });

export const adminListUsers = (q = "") => request(`/admin/users${q ? `?q=${encodeURIComponent(q)}` : ""}`, { method: "GET" });
export const adminPromoteUser = (id) => request(`/admin/users/${id}/promote`, { method: "POST" });
export const adminDemoteUser = (id) => request(`/admin/users/${id}/demote`, { method: "POST" });
export const adminAuditLog = () => request("/admin/audit-log", { method: "GET" });

export async function getJson(url) {
  const res = await fetch(url, { method: "GET", credentials: "include" });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch {}
  if (!res.ok) throw new Error(data?.error || text || `HTTP ${res.status}`);
  return data;
}

export const getJobWebhookLogs = (jobId, limit = 50) => getJson(`/api/jobs/${jobId}/webhook-logs?limit=${limit}`);
export async function testJobWebhooks(jobId) {
  const res = await fetch(`/api/jobs/${jobId}/test-webhooks`, { method: "POST", headers: { "Content-Type": "application/json" }, credentials: "include" });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch {}
  if (!res.ok) throw new Error((data && (data.error || data.detail)) || text || `HTTP ${res.status}`);
  return data || {};
}

async function sendJson(url, method, body) {
  const res = await fetch(url, { method, credentials: "include", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch {}
  if (!res.ok) throw new Error(data?.error || text || `HTTP ${res.status}`);
  return data;
}

export const listWebhookTemplates = () => getJson("/api/webhook-templates");
export const createWebhookTemplate = (name, url, type = "generic", body = "") => sendJson("/api/webhook-templates", "POST", { name, url, type, body });
export const deleteWebhookTemplate = (id) => sendJson(`/api/webhook-templates/${id}`, "DELETE", {});

export const listProjects = () => request("/projects", { method: "GET" });
export const createProject = (payload) => request("/projects", { method: "POST", body: JSON.stringify(payload) });
export const updateProject = (id, payload) => request(`/projects/${id}`, { method: "PATCH", body: JSON.stringify(payload) });
export const deleteProject = (id) => request(`/projects/${id}`, { method: "DELETE" });

export const listIncidents = (state = "") => request(`/incidents${state ? `?state=${encodeURIComponent(state)}` : ""}`, { method: "GET" });
export const updateIncident = (id, state) => request(`/incidents/${id}`, { method: "PATCH", body: JSON.stringify({ state }) });

// Doesn't use request() because a 503 (service unhealthy) is a normal,
// expected response here — the status page needs the JSON body either way,
// not an exception.
export async function getPublicStatus() {
  const res = await fetch(`${API_BASE}/public/status`, { method: "GET" });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch {}
  return { ok: res.ok, data };
}

