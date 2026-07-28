import { useEffect, useState } from "preact/hooks";
import { getAccount, getJson, listIncidents, listProjects } from "../api.js";

function formatDateTime(s) {
  if (!s) return "—";
  return new Date(s).toLocaleString();
}

function StatusPill({ status }) {
  let base = "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ";
  if (status === "ok") base += "bg-emerald-100 text-emerald-800";
  else if (status === "late") base += "bg-amber-100 text-amber-800";
  else base += "bg-slate-100 text-slate-700";
  return <span class={base}>{status}</span>;
}

export default function OverviewPage() {
  const [stats, setStats] = useState(null);
  const [account, setAccount] = useState(null);
  const [incidents, setIncidents] = useState([]);
  const [projects, setProjects] = useState([]);
  const [error, setError] = useState("");

  async function load() {
    try {
      const [overviewData, accountData, incidentData, projectData] = await Promise.all([
        getJson("/api/overview"),
        getAccount(),
        listIncidents("open"),
        listProjects(),
      ]);
      setStats(overviewData);
      setAccount(accountData);
      setIncidents(incidentData.incidents || []);
      setProjects(projectData.projects || []);
      setError("");
    } catch (err) {
      console.error(err);
      setError(err.message || "Failed to load overview");
    }
  }

  useEffect(() => {
    load();
    const id = setInterval(load, 15000);
    return () => clearInterval(id);
  }, []);

  if (!stats && error) return <div class="p-6 text-sm text-red-600">{error}</div>;
  if (!stats) return <div class="p-6 text-sm text-slate-500">Loading overview…</div>;

  const s = stats || {};
  const unhealthy = (s.lateJobs || 0) + (s.neverSeenJobs || 0);
  const plan = account?.plan || {};
  const maxJobs = plan?.max_jobs ?? 1;
  const jobsUsed = s.totalJobs ?? 0;

  return (
    <div class="max-w-6xl mx-auto p-6 space-y-6">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-baseline sm:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-slate-900">Overview</h1>
          <p class="text-sm text-slate-500">A fast read on jobs, active incidents, and the projects currently carrying risk.</p>
        </div>
      </div>

      {error && <div class="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>}

      <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
        <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Plan & limits</p>
            <p class="mt-1 text-sm font-semibold text-slate-900">{plan.label || "Free"} plan</p>
            <p class="text-xs text-slate-500 mt-0.5">{(plan.bullets && plan.bullets[0]) || "Usage limits and features are configured server-side."}</p>
          </div>
          <div class="flex flex-col items-end gap-2 text-[11px] text-slate-600">
            <div class="font-medium text-slate-700">Jobs used: {jobsUsed}/{maxJobs}</div>
            <div class="w-40 h-1.5 rounded-full bg-slate-100 overflow-hidden"><div class={(jobsUsed >= maxJobs ? "bg-red-400" : "bg-emerald-400") + " h-full rounded-full"} style={{ width: `${Math.min(100, (jobsUsed / maxJobs) * 100 || 0)}%` }} /></div>
            <p class="text-[11px] text-slate-500">Projects: {projects.length} · Active incidents: {incidents.length}</p>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"><p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Total jobs</p><p class="mt-2 text-2xl font-semibold text-slate-900">{s.totalJobs ?? 0}</p></div>
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"><p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Healthy</p><p class="mt-2 text-2xl font-semibold text-emerald-700">{s.okJobs ?? 0}</p></div>
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"><p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Needs attention</p><p class="mt-2 text-2xl font-semibold text-amber-700">{unhealthy}</p><p class="mt-1 text-xs text-slate-500">Late: {s.lateJobs ?? 0} • Never seen: {s.neverSeenJobs ?? 0}</p></div>
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"><p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Incidents (24h)</p><p class="mt-2 text-2xl font-semibold text-slate-900">{s.incidentCount24h ?? 0}</p><p class="mt-1 text-xs text-slate-500">Last incident: {formatDateTime(s.lastIncidentAt)}</p></div>
        <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"><p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Open incidents</p><p class="mt-2 text-2xl font-semibold text-red-700">{incidents.length}</p><p class="mt-1 text-xs text-slate-500">Across {projects.filter((p) => p.open_incidents > 0).length} projects</p></div>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <div class="rounded-2xl border border-slate-200 bg-white shadow-sm overflow-hidden">
          <div class="border-b border-slate-200 px-4 py-3"><h2 class="text-sm font-medium text-slate-900">Jobs needing attention</h2></div>
          {!s.jobsNeedingAttention || s.jobsNeedingAttention.length === 0 ? <div class="px-4 py-6 text-sm text-slate-500">Everything looks calm. No jobs need attention right now.</div> : (
            <table class="min-w-full divide-y divide-slate-200 text-sm"><thead class="bg-slate-50"><tr><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Job</th><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Status</th><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Last seen</th></tr></thead><tbody class="divide-y divide-slate-100 bg-white">{s.jobsNeedingAttention.map((job) => <tr key={job.id}><td class="px-4 py-2 text-slate-900">{job.name}</td><td class="px-4 py-2"><StatusPill status={job.status} /></td><td class="px-4 py-2 text-slate-500 text-xs">{formatDateTime(job.last_heartbeat_at)}</td></tr>)}</tbody></table>
          )}
        </div>
        <div class="rounded-2xl border border-slate-200 bg-white shadow-sm overflow-hidden">
          <div class="border-b border-slate-200 px-4 py-3"><h2 class="text-sm font-medium text-slate-900">Projects under pressure</h2></div>
          {projects.length === 0 ? <div class="px-4 py-6 text-sm text-slate-500">No projects yet. Create one to group related jobs and incidents.</div> : (
            <table class="min-w-full divide-y divide-slate-200 text-sm"><thead class="bg-slate-50"><tr><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Project</th><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Environment</th><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Jobs</th><th class="px-4 py-2 text-left text-xs uppercase tracking-wide text-slate-500">Open incidents</th></tr></thead><tbody class="divide-y divide-slate-100 bg-white">{projects.map((project) => <tr key={project.id}><td class="px-4 py-2 text-slate-900">{project.name}</td><td class="px-4 py-2 text-slate-500">{project.environment || "—"}</td><td class="px-4 py-2 text-slate-500">{project.job_count}</td><td class="px-4 py-2 text-slate-500">{project.open_incidents}</td></tr>)}</tbody></table>
          )}
        </div>
      </div>
    </div>
  );
}
