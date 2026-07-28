// src/pages/ProfilePage.jsx
import { h } from "preact";
import { route } from "preact-router";
import { useEffect, useState } from "preact/hooks";
import { listJobs, createJob } from "../api.js";

export default function ProfilePage({ user, onLogout }) {
  const [jobs, setJobs] = useState([]);
  const [loadingJobs, setLoadingJobs] = useState(true);
  const [jobError, setJobError] = useState("");

  const [jobName, setJobName] = useState("");
  const [intervalMinutes, setIntervalMinutes] = useState(60);
  const [graceMinutes, setGraceMinutes] = useState(5);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (!user) {
      route("/", true);
      return;
    }
    loadJobs();
  }, [user]);

  const loadJobs = async () => {
    setLoadingJobs(true);
    setJobError("");
    try {
      const res = await listJobs();
      setJobs(res.jobs || []);
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to load jobs.");
    } finally {
      setLoadingJobs(false);
    }
  };

  const handleCreateJob = async (e) => {
    e.preventDefault();
    if (!jobName.trim()) return;
    setCreating(true);
    setJobError("");
    try {
      const res = await createJob(jobName.trim(), Number(intervalMinutes), Number(graceMinutes));
      setJobs((prev) => [...prev, res.job]);
      setJobName("");
      setIntervalMinutes(60);
      setGraceMinutes(5);
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to create job.");
    } finally {
      setCreating(false);
    }
  };

  const renderStatusPill = (status) => {
    let label = "Unknown";
    let classes =
      "inline-flex items-center px-2 py-0.5 rounded-full text-[11px] border";

    if (status === "ok") {
      label = "Healthy";
      classes += " bg-emerald-50 border-emerald-200 text-emerald-700";
    } else if (status === "late") {
      label = "Late";
      classes += " bg-amber-50 border-amber-200 text-amber-700";
    } else if (status === "never_seen") {
      label = "No data";
      classes += " bg-slate-50 border-slate-200 text-slate-500";
    }

    return (
      <span class={classes}>
        <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
        {label}
      </span>
    );
  };

  const formatRelative = (iso) => {
    if (!iso) return "never";
    const dt = new Date(iso);
    return dt.toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const renderCronSnippet = (job) => {
    const minutes = Math.round(job.expected_interval_seconds / 60);
    const cron =
      minutes === 60
        ? `0 * * * *`
        : minutes === 1440
        ? `0 3 * * *`
        : `*/${minutes} * * * *`;

    return `${cron} /opt/scripts/${job.name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "_")
      .replace(/^_+|_+$/g, "") || "job"}.sh && \\\n  curl -fsS "${job.heartbeat_url}" >/dev/null`;
  };

  if (!user) {
    return null;
  }

  return (
    <div class="max-w-5xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-semibold text-slate-900 mb-1">
        Profile & settings
      </h1>
      <p class="text-xs text-slate-500 mb-5">
        Account details and the foundation for API keys and monitored jobs.
      </p>

      <div class="grid gap-4 md:grid-cols-2">
        <section class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
          <h2 class="text-sm font-semibold text-slate-900 mb-2">Account</h2>
          <div class="text-sm text-slate-700 space-y-1">
            <div>
              <span class="text-slate-500">Email: </span>
              <span>{user.email}</span>
            </div>
            {user.created_at && (
              <div>
                <span class="text-slate-500">Member since: </span>
                <span>
                  {new Date(user.created_at).toLocaleString(undefined, {
                    year: "numeric",
                    month: "short",
                    day: "2-digit",
                  })}
                </span>
              </div>
            )}
          </div>
        </section>

        {/* <section class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
          <h2 class="text-sm font-semibold text-slate-900 mb-2">
            Monitored jobs
          </h2>
          <p class="text-xs text-slate-600 mb-3">
            Define a job, then add the heartbeat URL to your cron, systemd
            timer, or CI pipeline. RunGuard watches for missed check-ins.
          </p>

          <form class="space-y-3 mb-4" onSubmit={handleCreateJob}>
            <div>
              <label class="block text-xs text-slate-600 mb-1">
                Job name
              </label>
              <input
                type="text"
                class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={jobName}
                onInput={(e) => setJobName(e.target.value)}
                placeholder="Nightly orders sync"
              />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-slate-600 mb-1">
                  Expected interval (minutes)
                </label>
                <input
                  type="number"
                  min="1"
                  class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={intervalMinutes}
                  onInput={(e) => setIntervalMinutes(e.target.value)}
                />
              </div>
              <div>
                <label class="block text-xs text-slate-600 mb-1">
                  Grace period (minutes)
                </label>
                <input
                  type="number"
                  min="0"
                  class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={graceMinutes}
                  onInput={(e) => setGraceMinutes(e.target.value)}
                />
              </div>
            </div>
            <button
              type="submit"
              disabled={creating || !jobName.trim()}
              class="w-full mt-1 py-2 rounded-lg text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60 shadow-sm"
            >
              {creating ? "Creating job…" : "Add job"}
            </button>
          </form>

          {jobError && (
            <div class="text-[11px] text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2 mb-3">
              {jobError}
            </div>
          )}

          <div class="border-t border-slate-200 pt-3 mt-1">
            {loadingJobs ? (
              <p class="text-[11px] text-slate-500">Loading jobs…</p>
            ) : jobs.length === 0 ? (
              <p class="text-[11px] text-slate-500">
                No jobs yet. Create one above and add the heartbeat snippet to
                your cron.
              </p>
            ) : (
              <div class="space-y-3 max-h-72 overflow-y-auto pr-1">
                {jobs.map((job) => (
                  <div
                    key={job.id}
                    class="border border-slate-200 rounded-xl p-3 bg-slate-50/50"
                  >
                    <div class="flex items-center justify-between mb-1.5">
                      <div class="text-xs font-semibold text-slate-900">
                        {job.name}
                      </div>
                      {renderStatusPill(job.status)}
                    </div>
                    <div class="text-[11px] text-slate-500 mb-1">
                      Last heartbeat: {formatRelative(job.last_heartbeat_at)}
                    </div>
                    <div class="text-[11px] text-slate-500 mb-2">
                      Interval ~{" "}
                      {Math.round(job.expected_interval_seconds / 60)} min, grace{" "}
                      {Math.round(job.grace_seconds / 60)} min
                    </div>
                    <div class="text-[11px] text-slate-700 font-mono bg-white border border-slate-200 rounded-lg px-2.5 py-2 overflow-x-auto">
                      <div class="mb-1 text-slate-500">cron example</div>
                      <pre class="whitespace-pre leading-snug">
                        {renderCronSnippet(job)}
                      </pre>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </section> */}
      </div>

      {/* <div class="mt-6">
        <button
          onClick={onLogout}
          class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100"
        >
          Logout
        </button>
      </div> */}
    </div>
  );
}
