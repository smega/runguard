import { route } from "preact-router";
import { useEffect, useState } from "preact/hooks";
import { adminListUsers, adminPromoteUser, adminDemoteUser, adminAuditLog } from "../api.js";

export default function AdminPage({ user }) {
  const [users, setUsers] = useState([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState("");
  const [auditEntries, setAuditEntries] = useState([]);
  const [showAudit, setShowAudit] = useState(false);

  useEffect(() => {
    if (!user) { route("/", true); return; }
    if (!user.is_admin) { route("/overview", true); return; }
  }, [user]);

  async function load(q = "") {
    setLoading(true);
    setError("");
    try {
      const res = await adminListUsers(q);
      setUsers(res.users || []);
    } catch (err) {
      setError(err.message || "Failed to load users");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { if (user?.is_admin) load(); }, [user]);

  async function loadAudit() {
    try {
      const res = await adminAuditLog();
      setAuditEntries(res.entries || []);
      setShowAudit(true);
    } catch (err) {
      setError(err.message || "Failed to load audit log");
    }
  }

  async function handlePromote(id) {
    setBusyId(id);
    setError("");
    try {
      await adminPromoteUser(id);
      await load(query);
    } catch (err) {
      setError(err.message || "Failed to promote user");
    } finally {
      setBusyId("");
    }
  }

  async function handleDemote(id) {
    setBusyId(id);
    setError("");
    try {
      await adminDemoteUser(id);
      await load(query);
    } catch (err) {
      setError(err.message || "Failed to demote user");
    } finally {
      setBusyId("");
    }
  }

  if (!user || !user.is_admin) return null;

  return (
    <div class="max-w-5xl mx-auto px-4 py-8">
      <div class="flex items-center justify-between mb-6">
        <h1 class="text-xl font-semibold text-slate-900">Admin</h1>
        <button
          type="button"
          onClick={loadAudit}
          class="text-sm px-3 py-1.5 rounded-full border border-slate-300 text-slate-700 hover:bg-slate-100"
        >
          View audit log
        </button>
      </div>

      {error && (
        <div class="mb-4 text-sm text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2">{error}</div>
      )}

      <div class="mb-4 flex gap-2">
        <input
          type="text"
          value={query}
          onInput={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") load(query); }}
          placeholder="Search by email…"
          class="flex-1 border border-slate-300 rounded-lg px-3 py-1.5 text-sm"
        />
        <button
          type="button"
          onClick={() => load(query)}
          class="text-sm px-3 py-1.5 rounded-lg border border-slate-300 text-slate-700 hover:bg-slate-100"
        >
          Search
        </button>
      </div>

      <div class="border border-slate-200 rounded-xl overflow-hidden bg-white">
        <table class="w-full text-sm">
          <thead class="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
            <tr>
              <th class="text-left px-4 py-2">Email</th>
              <th class="text-left px-4 py-2">Plan</th>
              <th class="text-left px-4 py-2">Billing status</th>
              <th class="text-left px-4 py-2">Admin</th>
              <th class="text-right px-4 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr><td colSpan="5" class="px-4 py-6 text-center text-slate-500">Loading…</td></tr>
            )}
            {!loading && users.length === 0 && (
              <tr><td colSpan="5" class="px-4 py-6 text-center text-slate-500">No users found.</td></tr>
            )}
            {!loading && users.map((u) => (
              <tr key={u.id} class="border-t border-slate-100">
                <td class="px-4 py-2 text-slate-800">{u.email}</td>
                <td class="px-4 py-2 text-slate-600">{u.plan}</td>
                <td class="px-4 py-2 text-slate-600">{u.billing_status || "—"}</td>
                <td class="px-4 py-2">
                  {u.is_admin ? (
                    <span class="text-xs px-2 py-0.5 rounded-full bg-blue-50 text-blue-700 border border-blue-100">admin</span>
                  ) : (
                    <span class="text-xs text-slate-400">—</span>
                  )}
                </td>
                <td class="px-4 py-2 text-right">
                  {u.is_admin ? (
                    <button
                      type="button"
                      disabled={busyId === u.id || u.id === user.id}
                      onClick={() => handleDemote(u.id)}
                      title={u.id === user.id ? "You can't demote yourself" : ""}
                      class="text-xs px-2.5 py-1 rounded-full border border-slate-300 text-slate-700 hover:bg-slate-100 disabled:opacity-50"
                    >
                      Demote
                    </button>
                  ) : (
                    <button
                      type="button"
                      disabled={busyId === u.id}
                      onClick={() => handlePromote(u.id)}
                      class="text-xs px-2.5 py-1 rounded-full border border-slate-300 text-slate-700 hover:bg-slate-100 disabled:opacity-50"
                    >
                      Promote
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showAudit && (
        <div class="mt-8">
          <h2 class="text-sm font-medium text-slate-900 mb-2">Recent admin actions</h2>
          <div class="border border-slate-200 rounded-xl overflow-hidden bg-white">
            <table class="w-full text-sm">
              <thead class="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
                <tr>
                  <th class="text-left px-4 py-2">When</th>
                  <th class="text-left px-4 py-2">Actor</th>
                  <th class="text-left px-4 py-2">Action</th>
                  <th class="text-left px-4 py-2">Target</th>
                </tr>
              </thead>
              <tbody>
                {auditEntries.length === 0 && (
                  <tr><td colSpan="4" class="px-4 py-6 text-center text-slate-500">No admin actions recorded yet.</td></tr>
                )}
                {auditEntries.map((entry) => (
                  <tr key={entry.id} class="border-t border-slate-100">
                    <td class="px-4 py-2 text-slate-600">{new Date(entry.at).toLocaleString()}</td>
                    <td class="px-4 py-2 text-slate-800">{entry.actor_email}</td>
                    <td class="px-4 py-2 text-slate-600">{entry.action}</td>
                    <td class="px-4 py-2 text-slate-800">{entry.target_email}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
