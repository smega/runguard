// src/pages/PublicJobPage.jsx
import { useEffect, useState } from "preact/hooks";

async function getJson(url) {
  const res = await fetch(url, { method: "GET" });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {}
  if (!res.ok) throw new Error(data?.error || text || `HTTP ${res.status}`);
  return data;
}

function formatDateTime(s) {
  if (!s) return "—";
  const d = new Date(s);
  return d.toLocaleString();
}

function StatusPill({ status }) {
  let base =
    "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ";
  if (status === "ok") base += "bg-emerald-100 text-emerald-800";
  else if (status === "late") base += "bg-amber-100 text-amber-800";
  else base += "bg-slate-100 text-slate-700";
  return <span class={base}>{status}</span>;
}

export default function PublicJobPage({ token }) {
  const [job, setJob] = useState(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    (async () => {
      try {
        const data = await getJson(
          `/api/public/job/${encodeURIComponent(token)}`
        );
        setJob(data);
      } catch (err) {
        console.error(err);
        setError(err.message || "Failed to load job");
      } finally {
        setLoading(false);
      }
    })();
  }, [token]);

  if (loading) {
    return (
      <div class="min-h-screen flex items-center justify-center bg-slate-50">
        <p class="text-sm text-slate-500">Loading status…</p>
      </div>
    );
  }

  if (error || !job) {
    return (
      <div class="min-h-screen flex items-center justify-center bg-slate-50">
        <div class="max-w-md rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p class="text-sm font-medium text-slate-900 mb-1">
            Status unavailable
          </p>
          <p class="text-sm text-slate-500">
            {error || "This public link is invalid or has been disabled."}
          </p>
        </div>
      </div>
    );
  }

  const u = job.uptime7d || {};
  const percentOk = (u.percentOk ?? 0).toFixed(1);

  return (
    <div class="min-h-screen flex items-center justify-center bg-slate-50 px-4 py-8">
      <div class="w-full max-w-xl rounded-2xl border border-slate-200 bg-white p-6 shadow-sm space-y-4">
        <div class="flex items-center justify-between gap-3">
          <div>
            <h1 class="text-base font-semibold text-slate-900">{job.name}</h1>
            <p class="text-xs text-slate-500">Monitored by RunGuard</p>
          </div>
          <StatusPill status={job.status} />
        </div>

        <div class="grid grid-cols-2 gap-4 text-sm">
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">
              Interval
            </p>
            <p class="mt-1 text-slate-900">
              Every {Math.round(job.expected_interval_seconds / 60)} min
            </p>
          </div>
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">
              Grace
            </p>
            <p class="mt-1 text-slate-900">
              {Math.round(job.grace_seconds / 60)} min
            </p>
          </div>
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">
              Last heartbeat
            </p>
            <p class="mt-1 text-slate-900">
              {formatDateTime(job.last_heartbeat_at)}
            </p>
          </div>
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">
              Uptime (7d)
            </p>
            <p class="mt-1 text-slate-900">{percentOk}% OK</p>
            <p class="text-xs text-slate-500 mt-0.5">
              OK: {u.ok ?? 0} • Late: {u.late ?? 0} / {u.total ?? 0} runs
            </p>
          </div>
        </div>

        <p class="text-[11px] text-slate-400">
          No credentials are exposed. This link only shows high-level job
          health.
        </p>
      </div>
    </div>
  );
}
