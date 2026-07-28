import { useEffect, useState } from "preact/hooks";
import { route } from "preact-router";
import { listIncidents, updateIncident } from "../api.js";

function stateBadge(state) {
  const base = "inline-flex rounded-full px-2 py-0.5 text-xs font-medium ";
  if (state === "open") return base + "bg-red-50 text-red-700 border border-red-200";
  if (state === "acknowledged") return base + "bg-amber-50 text-amber-700 border border-amber-200";
  return base + "bg-emerald-50 text-emerald-700 border border-emerald-200";
}

export default function IncidentsPage({ user }) {
  const [incidents, setIncidents] = useState([]);
  const [state, setState] = useState("");
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState("");

  useEffect(() => { if (!user) route("/", true); }, [user]);
  useEffect(() => { if (user) reload(); }, [user, state]);

  async function reload() {
    try { setError(""); const res = await listIncidents(state); setIncidents(res.incidents || []); } catch (err) { setError(err.message || "Failed to load incidents"); }
  }

  async function changeState(id, next) {
    setBusyId(id + next);
    try { await updateIncident(id, next); await reload(); } catch (err) { setError(err.message || "Failed to update incident"); }
    finally { setBusyId(""); }
  }

  if (!user) return null;

  return (
    <div class="max-w-6xl mx-auto px-4 py-8 space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-slate-900">Incidents</h1>
          <p class="text-sm text-slate-500">Track active late or failed jobs, acknowledge them, and keep a lightweight incident trail.</p>
        </div>
        <select value={state} onChange={(e) => setState(e.target.value)} class="rounded-xl border border-slate-300 px-3 py-2 text-sm bg-white">
          <option value="">All states</option>
          <option value="open">Open</option>
          <option value="acknowledged">Acknowledged</option>
          <option value="resolved">Resolved</option>
        </select>
      </div>
      {error && <div class="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>}
      <div class="rounded-2xl border border-slate-200 bg-white shadow-sm overflow-hidden">
        {incidents.length === 0 ? <div class="px-4 py-8 text-sm text-slate-500">No incidents in this view.</div> : (
          <table class="min-w-full text-sm">
            <thead class="bg-slate-50 text-xs uppercase tracking-wide text-slate-500"><tr><th class="px-4 py-2 text-left">Incident</th><th class="px-4 py-2 text-left">Project</th><th class="px-4 py-2 text-left">State</th><th class="px-4 py-2 text-left">Opened</th><th class="px-4 py-2 text-left">Occurrences</th><th class="px-4 py-2 text-right">Actions</th></tr></thead>
            <tbody class="divide-y divide-slate-100">
              {incidents.map((incident) => (
                <tr key={incident.id}>
                  <td class="px-4 py-3"><div class="font-medium text-slate-900">{incident.title}</div><div class="text-xs text-slate-500">{incident.job_name || incident.job_id} · {incident.summary || incident.kind}</div></td>
                  <td class="px-4 py-3 text-slate-600">{incident.project_name || "Unassigned"}</td>
                  <td class="px-4 py-3"><span class={stateBadge(incident.state)}>{incident.state}</span></td>
                  <td class="px-4 py-3 text-slate-600">{new Date(incident.opened_at).toLocaleString()}</td>
                  <td class="px-4 py-3 text-slate-600">{incident.occurrence_count}</td>
                  <td class="px-4 py-3"><div class="flex justify-end gap-2">{incident.state !== "acknowledged" && incident.state !== "resolved" && <button disabled={busyId===incident.id+"acknowledged"} onClick={() => changeState(incident.id, "acknowledged")} class="rounded-full border border-slate-300 px-3 py-1 text-xs text-slate-700 hover:bg-slate-100">Acknowledge</button>}{incident.state !== "resolved" && <button disabled={busyId===incident.id+"resolved"} onClick={() => changeState(incident.id, "resolved")} class="rounded-full bg-slate-900 px-3 py-1 text-xs text-white hover:bg-slate-800">Resolve</button>}</div></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
