// src/pages/JobsPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import {
  listJobs,
  createJob,
  updateJob,
  deleteJob,
  getJob,
  listJobRuns,
  getJobStats,
  getAccount,
  getJobWebhookLogs,
  testJobWebhooks,
  listWebhookTemplates,
  listProjects,
} from "../api.js";

const SNIPPET_TYPES = ["cron", "bash-wrapper", "systemd", "python", "node"];

const SEVERITY_OPTIONS = [
  { key: "low", label: "Low" },
  { key: "normal", label: "Normal" },
  { key: "high", label: "High" },
  { key: "critical", label: "Critical" },
];

// public read-only host
const PUBLIC_BASE_URL = "https://runguard-public.smega.eu";

// remembers the last job the user had open so re-visiting the page (or
// reloading it) drops them back where they left off instead of a blank pane
const LAST_JOB_KEY = "runguard:lastSelectedJobId";

export default function JobsPage({ user }) {
  const [jobs, setJobs] = useState([]);
  const [loadingJobs, setLoadingJobs] = useState(true);
  const [jobError, setJobError] = useState("");

  // New-job creation now lives in a slide-over panel (rather than an
  // always-open card at the top of the page) so the jobs list is the first
  // thing visible when the page loads.
  const [showNewJobPanel, setShowNewJobPanel] = useState(false);
  const [jobName, setJobName] = useState("");
  const [intervalMinutes, setIntervalMinutes] = useState(60);
  const [graceMinutes, setGraceMinutes] = useState(5);
  const [scheduleMode, setScheduleMode] = useState("interval"); // "interval" | "cron"
  const [cronExpr, setCronExpr] = useState("");
  const [creating, setCreating] = useState(false);
  const [projects, setProjects] = useState([]);
  const [projectId, setProjectId] = useState("");

  const [editingId, setEditingId] = useState(null);
  const [editName, setEditName] = useState("");
  const [editInterval, setEditInterval] = useState(60);
  const [editGrace, setEditGrace] = useState(5);
  const [editCronExpr, setEditCronExpr] = useState("");
  const [editingBusy, setEditingBusy] = useState(false);
  const [editProjectId, setEditProjectId] = useState("");

  const [deletingId, setDeletingId] = useState(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState(null);

  const [statusFilter, setStatusFilter] = useState("all");
  const [severityFilter, setSeverityFilter] = useState("all");
  const [silencedOnly, setSilencedOnly] = useState(false);
  const [togglingEmailId, setTogglingEmailId] = useState(null);
  const [togglingWebhookId, setTogglingWebhookId] = useState(null);
  const [togglingPublicId, setTogglingPublicId] = useState(null);

  // account / plan
  const [account, setAccount] = useState(null);
  const [accountLoading, setAccountLoading] = useState(true);
  const [accountError, setAccountError] = useState("");

  // inline detail panel
  const [selectedJobId, setSelectedJobId] = useState(null);
  const [detailJob, setDetailJob] = useState(null);
  const [detailRuns, setDetailRuns] = useState([]);
  const [detailStats, setDetailStats] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");
  const [snippetType, setSnippetType] = useState("cron");
  const [copyFeedback, setCopyFeedback] = useState("");

  // webhook logs in detail panel
  const [detailWebhookLogs, setDetailWebhookLogs] = useState([]);
  const [detailWebhookError, setDetailWebhookError] = useState("");

  // test webhook button state
  const [testWebhookBusy, setTestWebhookBusy] = useState(false);
  const [testWebhookFeedback, setTestWebhookFeedback] = useState("");

  // webhook templates (list-only; managed elsewhere)
  const [webhookTemplates, setWebhookTemplates] = useState([]);
  const [webhookTemplatesLoading, setWebhookTemplatesLoading] = useState(true);
  const [webhookTemplatesError, setWebhookTemplatesError] = useState("");

  // runbook / notes for detail job
  const [runbookUrl, setRunbookUrl] = useState("");
  const [runbookNotes, setRunbookNotes] = useState("");
  const [runbookSaving, setRunbookSaving] = useState(false);

  // toast notifications
  const [toasts, setToasts] = useState([]);

  const pushToast = (variant, message) => {
    const id =
      Date.now().toString() + Math.random().toString(36).slice(2, 8);
    setToasts((prev) => [...prev, { id, variant, message }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4000);
  };

  // Load jobs + periodic refresh
  useEffect(() => {
    if (!user) return;
    loadJobs();

    const id = setInterval(() => {
      loadJobs(true);
    }, 15000);

    return () => clearInterval(id);
  }, [user]);

  // Load account / plan once per login
  useEffect(() => {
    if (!user) return;
    loadAccount();
  }, [user]);

  // Load webhook templates once per login
  useEffect(() => {
    if (!user) return;
    loadWebhookTemplates();
  }, [user]);

  useEffect(() => {
    if (!user) return;
    listProjects().then((res) => setProjects(res.projects || [])).catch(() => setProjects([]));
  }, [user]);

  // Periodic refresh for selected job details
  useEffect(() => {
    if (!selectedJobId) return;

    const id = setInterval(() => {
      loadDetail(selectedJobId, true);
    }, 15000);

    return () => clearInterval(id);
  }, [selectedJobId]);

  // Restore the last-viewed job once, after the first successful job load
  useEffect(() => {
    if (selectedJobId || jobs.length === 0) return;
    let lastId = "";
    try {
      lastId = window.localStorage.getItem(LAST_JOB_KEY) || "";
    } catch {
      lastId = "";
    }
    if (lastId && jobs.some((j) => j.id === lastId)) {
      setSelectedJobId(lastId);
      loadDetail(lastId);
    }
  }, [jobs]);

  // Persist (or clear) the last-viewed job whenever the selection changes
  useEffect(() => {
    try {
      if (selectedJobId) {
        window.localStorage.setItem(LAST_JOB_KEY, selectedJobId);
      } else {
        window.localStorage.removeItem(LAST_JOB_KEY);
      }
    } catch {
      // localStorage unavailable (private mode, etc.) — safe to ignore
    }
  }, [selectedJobId]);

  // Escape dismisses the New job slide-over, same as clicking its backdrop.
  useEffect(() => {
    if (!showNewJobPanel) return;
    const onKeyDown = (e) => {
      if (e.key === "Escape") setShowNewJobPanel(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [showNewJobPanel]);

  const isJobSilenced = (job) => {
    if (job.maintenance_mode === "until_success" || job.maintenance_mode === "manual") {
      return true;
    }
    const raw = job.maintenance_until || job.maintenanceUntil;
    if (!raw) return false;
    const until = new Date(raw);
    return !isNaN(until.getTime()) && until > new Date();
  };

  // sync runbook state when detail job changes
  useEffect(() => {
    if (detailJob) {
      setRunbookUrl(detailJob.runbook_url || "");
      setRunbookNotes(detailJob.notes || "");
    } else {
      setRunbookUrl("");
      setRunbookNotes("");
    }
  }, [detailJob]);

  const loadJobs = async (silent = false) => {
    if (!silent) {
      setLoadingJobs(true);
      setJobError("");
    }
    try {
      const res = await listJobs();
      setJobs(res.jobs || []);
    } catch (err) {
      console.error(err);
      if (!silent) {
        setJobError(err.message || "Failed to load jobs.");
      }
    } finally {
      if (!silent) {
        setLoadingJobs(false);
      }
    }
  };

  const loadAccount = async (silent = false) => {
    if (!silent) {
      setAccountLoading(true);
      setAccountError("");
    }
    try {
      const data = await getAccount();
      setAccount(data);
    } catch (err) {
      console.error("load account", err);
      if (!silent) {
        setAccountError(err.message || "Failed to load account/plan info.");
      }
    } finally {
      if (!silent) {
        setAccountLoading(false);
      }
    }
  };

  const loadWebhookTemplates = async (silent = false) => {
    if (!silent) {
      setWebhookTemplatesLoading(true);
      setWebhookTemplatesError("");
    }
    try {
      const res = await listWebhookTemplates();
      setWebhookTemplates(res.templates || []);
    } catch (err) {
      console.error("load webhook templates", err);
      if (!silent) {
        setWebhookTemplatesError(
          err.message || "Failed to load webhook templates."
        );
      }
    } finally {
      if (!silent) {
        setWebhookTemplatesLoading(false);
      }
    }
  };

  const handleCreateJob = async (e) => {
    e.preventDefault();
    if (!jobName.trim()) return;

    const plan = account?.plan;
    const maxJobs = plan?.max_jobs ?? null;
    const jobsUsed = jobs.length;

    if (maxJobs && jobsUsed >= maxJobs) {
      const msg = `Job limit reached for your current plan (max ${maxJobs} jobs).`;
      setJobError(msg);
      pushToast("info", msg);
      return;
    }

    setCreating(true);
    setJobError("");
    try {
      const res = await createJob(
        jobName.trim(),
        Number(intervalMinutes),
        Number(graceMinutes),
        projectId,
        scheduleMode === "cron" ? cronExpr.trim() : ""
      );
      setJobs((prev) => [...prev, res.job]);
      setJobName("");
      setIntervalMinutes(60);
      setGraceMinutes(5);
      setProjectId("");
      setCronExpr("");
      setScheduleMode("interval");
      setShowNewJobPanel(false);
      pushToast("success", "Job created.");
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to create job.");
      pushToast("error", err.message || "Failed to create job.");
    } finally {
      setCreating(false);
    }
  };

  const startEdit = (job) => {
    setEditingId(job.id);
    setEditName(job.name);
    setEditInterval(Math.round(job.expected_interval_seconds / 60));
    setEditGrace(Math.round(job.grace_seconds / 60));
    setEditProjectId(job.project_id || "");
    setEditCronExpr(job.cron_expr || "");
    setJobError("");
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingBusy(false);
  };

  const handleEditSave = async (id) => {
    setEditingBusy(true);
    setJobError("");
    try {
      const payload = {
        name: editName.trim(),
        interval_minutes: Number(editInterval),
        grace_minutes: Number(editGrace),
        cron_expr: editCronExpr.trim(),
        // project_id was previously omitted here despite the edit form
        // always having a Project selector -- changing it silently did
        // nothing. "" clears the assignment (handleUpdateJob treats an
        // empty project_id as "unassign").
        project_id: editProjectId,
      };
      const res = await updateJob(id, payload);
      setJobs((prev) => prev.map((j) => (j.id === id ? res.job : j)));
      setEditingId(null);

      if (detailJob && detailJob.id === id) {
        setDetailJob(res.job);
      }
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update job.");
      pushToast("error", err.message || "Failed to update job.");
    } finally {
      setEditingBusy(false);
    }
  };

  const handleDelete = async (id, name) => {
    setDeletingId(id);
    setJobError("");
    try {
      await deleteJob(id);
      setJobs((prev) => prev.filter((j) => j.id !== id));
      if (detailJob && detailJob.id === id) {
        setSelectedJobId(null);
        setDetailJob(null);
        setDetailRuns([]);
        setDetailStats(null);
        setDetailError("");
        setDetailWebhookLogs([]);
        setDetailWebhookError("");
      }
      pushToast("success", `Job "${name || "job"}" deleted.`);
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to delete job.");
      pushToast("error", err.message || "Failed to delete job.");
    } finally {
      setDeletingId(null);
    }
  };

  const handleDeleteClick = (job) => {
    if (confirmDeleteId === job.id) {
      handleDelete(job.id, job.name);
      setConfirmDeleteId(null);
      return;
    }

    setConfirmDeleteId(job.id);
    // auto-reset after a few seconds
    setTimeout(() => {
      setConfirmDeleteId((current) => (current === job.id ? null : current));
    }, 4000);
  };

  const handleToggleEmail = async (job) => {
    const plan = account?.plan;
    if (plan && !plan.allow_email_alerts) {
      const msg = "Email alerts are not available on your current plan.";
      setJobError(msg);
      pushToast("info", msg);
      return;
    }

    setTogglingEmailId(job.id);
    setJobError("");
    try {
      const res = await updateJob(job.id, {
        alert_email_enabled: !job.alert_email_enabled,
      });
      setJobs((prev) => prev.map((j) => (j.id === job.id ? res.job : j)));
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(res.job);
      }
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update email alerts.");
      pushToast("error", err.message || "Failed to update email alerts.");
    } finally {
      setTogglingEmailId(null);
    }
  };

  const handleToggleWebhook = async (job) => {
    setTogglingWebhookId(job.id);
    setJobError("");
    try {
      const res = await updateJob(job.id, {
        alert_webhook_enabled: !job.alert_webhook_enabled,
      });
      setJobs((prev) => prev.map((j) => (j.id === job.id ? res.job : j)));
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(res.job);
      }
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update webhook alerts.");
      pushToast("error", err.message || "Failed to update webhook alerts.");
    } finally {
      setTogglingWebhookId(null);
    }
  };

  const handleTogglePublic = async (job) => {
    const plan = account?.plan;
    if (plan && !plan.allow_public_links) {
      const msg = "Public status links are not available on your current plan.";
      setJobError(msg);
      pushToast("info", msg);
      return;
    }

    setTogglingPublicId(job.id);
    setJobError("");
    try {
      const res = await updateJob(job.id, {
        public_enabled: !job.publicEnabled,
      });
      setJobs((prev) => prev.map((j) => (j.id === job.id ? res.job : j)));
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(res.job);
      }
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update public link.");
      pushToast("error", err.message || "Failed to update public link.");
    } finally {
      setTogglingPublicId(null);
    }
  };

  const handleChangeSeverity = async (job, newSeverity) => {
    setJobError("");
    try {
      const res = await updateJob(job.id, { severity: newSeverity });
      setJobs((prev) => prev.map((j) => (j.id === job.id ? res.job : j)));
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(res.job);
      }
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update severity.");
      pushToast("error", err.message || "Failed to update severity.");
    }
  };

  const handleSetMaintenance = async (job, minutes) => {
    setJobError("");

    // Picking a fixed-duration preset (including "Off") always switches the
    // job back to plain duration-based silence, overriding either of the
    // two indefinite freeze modes if one was active.
    const payload = { maintenance_mode: "duration" };
    let newMaintenance = null;

    if (minutes <= 0) {
      payload.maintenance_until = "";
      newMaintenance = null;
    } else {
      const now = new Date();
      const until = new Date(now.getTime() + minutes * 60000);
      const iso = until.toISOString();
      payload.maintenance_until = iso;
      newMaintenance = iso;
    }

    try {
      const res = await updateJob(job.id, payload);
      const baseJob = res.job || job;
      const updatedJob = {
        ...baseJob,
        maintenance_mode: "",
        maintenance_until: newMaintenance,
      };

      setJobs((prev) =>
        prev.map((j) => (j.id === job.id ? updatedJob : j))
      );
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(updatedJob);
      }
      pushToast("success", "Maintenance window updated.");
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update maintenance window.");
      pushToast("error", err.message || "Failed to update maintenance window.");
    }
  };

  // "until_success" (auto-clears on the job's next ok heartbeat) or
  // "manual" (silenced indefinitely until explicitly cleared via the "Off"
  // preset above). Both are indefinite -- no maintenance_until timestamp.
  const handleSetMaintenanceMode = async (job, mode) => {
    setJobError("");
    try {
      const res = await updateJob(job.id, { maintenance_mode: mode });
      const baseJob = res.job || job;
      const updatedJob = {
        ...baseJob,
        maintenance_mode: mode,
        maintenance_until: null,
      };
      setJobs((prev) => prev.map((j) => (j.id === job.id ? updatedJob : j)));
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(updatedJob);
      }
      pushToast(
        "success",
        mode === "until_success"
          ? "Silenced until the next successful heartbeat."
          : "Silenced until manually unmuted."
      );
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update maintenance mode.");
      pushToast("error", err.message || "Failed to update maintenance mode.");
    }
  };

  const jobWebhookUrls = (job) => job?.alert_webhook_urls || [];

  const handleToggleTemplateForJob = async (job, tmpl) => {
    setJobError("");
    const current = jobWebhookUrls(job);
    const attached = current.includes(tmpl.url);
    let nextUrls;
    if (attached) {
      nextUrls = current.filter((u) => u !== tmpl.url);
    } else {
      nextUrls = [...current, tmpl.url];
    }

    const enableFlag = nextUrls.length > 0;

    try {
      const res = await updateJob(job.id, {
        alert_webhook_urls: nextUrls,
        alert_webhook_enabled:
          enableFlag || job.alert_webhook_enabled || false,
      });
      setJobs((prev) => prev.map((j) => (j.id === job.id ? res.job : j)));
      if (detailJob && detailJob.id === job.id) {
        setDetailJob(res.job);
      }
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update webhook templates.");
      pushToast("error", err.message || "Failed to update webhooks.");
    }
  };

  const handleSaveRunbook = async () => {
    if (!detailJob) return;
    setRunbookSaving(true);
    setJobError("");
    try {
      const patch = {
        runbook_url: runbookUrl.trim() || null,
        notes: runbookNotes.trim() || "",
      };
      const res = await updateJob(detailJob.id, patch);
      setJobs((prev) =>
        prev.map((j) => (j.id === detailJob.id ? res.job : j))
      );
      setDetailJob(res.job);
      pushToast("success", "Runbook details updated.");
    } catch (err) {
      console.error(err);
      setJobError(err.message || "Failed to update runbook details.");
      pushToast("error", err.message || "Failed to update runbook details.");
    } finally {
      setRunbookSaving(false);
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

  const renderSeverityPill = (severity) => {
    const s = (severity || "normal").toLowerCase();
    let label = "Normal";
    let classes =
      "inline-flex items-center px-2 py-0.5 rounded-full text-[11px] border ";

    if (s === "low") {
      label = "Low";
      classes += "bg-slate-50 border-slate-200 text-slate-600";
    } else if (s === "high") {
      label = "High";
      classes += "bg-amber-50 border-amber-200 text-amber-700";
    } else if (s === "critical") {
      label = "Critical";
      classes += "bg-rose-50 border-rose-200 text-rose-700";
    } else {
      classes += "bg-blue-50 border-blue-200 text-blue-700";
    }

    return (
      <span class={classes}>
        <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
        {label}
      </span>
    );
  };

  const renderWebhookStatusPill = (status) => {
    const base =
      "inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border ";

    if (status === "success") {
      return (
        <span class={base + "bg-emerald-50 border-emerald-200 text-emerald-700"}>
          <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
          Success
        </span>
      );
    } else if (status === "failed") {
      return (
        <span class={base + "bg-rose-50 border-rose-200 text-rose-700"}>
          <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
          Failed
        </span>
      );
    } else if (status === "skipped") {
      return (
        <span class={base + "bg-slate-50 border-slate-200 text-slate-600"}>
          <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
          Skipped
        </span>
      );
    }

    return (
      <span class={base + "bg-slate-50 border-slate-200 text-slate-600"}>
        <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
        Unknown
      </span>
    );
  };

  const formatTimestamp = (iso) => {
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

  const formatShortTime = (iso) => {
    if (!iso) return "never";
    const dt = new Date(iso);
    return dt.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const formatShortDateTime = (iso) => {
    if (!iso) return "never";
    const dt = new Date(iso);
    return dt.toLocaleString(undefined, {
      month: "short",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const cronExpression = (job) => {
    const minutes = Math.round(job.expected_interval_seconds / 60);
    if (minutes === 60) return "0 * * * *";
    if (minutes === 1440) return "0 3 * * *";
    return `*/${minutes} * * * *`;
  };

  const safeJobSlug = (job) => {
    return (
      job.name
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "_")
        .replace(/^_+|_+$/g, "") || "job"
    );
  };

  const renderSnippet = () => {
    if (!detailJob) return "";
    const cron = cronExpression(detailJob);
    const slug = safeJobSlug(detailJob);
    const url = detailJob.heartbeat_url;

    switch (snippetType) {
      case "cron":
        return `${cron} /opt/scripts/${slug}.sh && \\\n  curl -fsS "${url}" >/dev/null`;
      case "bash-wrapper":
        return `#!/usr/bin/env bash
set -euo pipefail

RUNGUARD_URL="${url}"

echo "[${slug}] starting"
if /opt/scripts/${slug}.sh; then
  curl -fsS "$RUNGUARD_URL" >/dev/null 2>&1 || true
else
  curl -fsS "$RUNGUARD_URL?status=error" >/dev/null 2>&1 || true
  exit 1
fi`;
      case "systemd":
        return `# /etc/systemd/system/${slug}.service
[Unit]
Description=${detailJob.name}

[Service]
Type=oneshot
ExecStart=/opt/scripts/${slug}.sh
ExecStartPost=/usr/bin/curl -fsS "${url}"

# /etc/systemd/system/${slug}.timer
[Unit]
Description=Run ${detailJob.name} on schedule

[Timer]
OnCalendar=*:0/${Math.max(
          1,
          Math.round(detailJob.expected_interval_seconds / 60)
        )}

[Install]
WantedBy=timers.target

# systemctl enable --now ${slug}.timer`;
      case "python":
        return `import subprocess
import requests

RUNGUARD_URL = "${url}"

def main():
    subprocess.check_call(["/opt/scripts/${slug}.sh"])
    requests.get(RUNGUARD_URL, timeout=5)

if __name__ == "__main__":
    main()`;
      case "node":
        return `import { exec } from "child_process";
import fetch from "node-fetch";

const RUNGUARD_URL = "${url}";

exec("/opt/scripts/${slug}.sh", async (err) => {
  if (err) {
    await fetch(RUNGUARD_URL + "?status=error").catch(() => {});
    process.exit(1);
  }

  try {
    await fetch(RUNGUARD_URL);
  } catch (e) {}
});`;
      default:
        return "";
    }
  };

  const loadDetail = async (jobId, silent = false) => {
    if (!silent) {
      setDetailLoading(true);
      setDetailError("");
      setDetailJob(null);
      setDetailRuns([]);
      setDetailStats(null);
      setSnippetType("cron");
      setCopyFeedback("");
      setDetailWebhookLogs([]);
      setDetailWebhookError("");
    }

    try {
      const [jobRes, runsRes, statsRes] = await Promise.all([
        getJob(jobId),
        listJobRuns(jobId, 50),
        getJobStats(jobId),
      ]);
      setDetailJob(jobRes.job);
      setDetailRuns(runsRes.runs || []);
      setDetailStats(statsRes.stats || null);

      try {
        const logsRes = await getJobWebhookLogs(jobId, 50);
        setDetailWebhookLogs(logsRes.logs || []);
        setDetailWebhookError("");
      } catch (err2) {
        console.error("load webhook logs", err2);
        if (!silent) {
          setDetailWebhookError(
            err2.message || "Failed to load webhook deliveries."
          );
        }
      }
    } catch (err) {
      console.error(err);
      if (!silent) {
        setDetailError(err.message || "Failed to load job details.");
      }
    } finally {
      if (!silent) {
        setDetailLoading(false);
      }
    }
  };

  const handleRowClick = (job) => {
    if (editingId && editingId === job.id) return;
    setSelectedJobId(job.id);
    loadDetail(job.id);
  };

  const handleTestWebhooks = async () => {
    if (!detailJob) return;

    setTestWebhookBusy(true);
    setTestWebhookFeedback("");
    try {
      const res = await testJobWebhooks(detailJob.id);
      if (res && res.ok) {
        setTestWebhookFeedback("Test sent");
        pushToast("success", "Test webhook sent.");
      } else if (res && res.last_error) {
        const msg = `Test sent (last error: ${res.last_error})`;
        setTestWebhookFeedback(msg);
        pushToast("info", msg);
      } else {
        setTestWebhookFeedback("Test sent");
        pushToast("success", "Test webhook sent.");
      }

      loadDetail(detailJob.id, true);

      setTimeout(() => setTestWebhookFeedback(""), 4000);
    } catch (err) {
      console.error(err);
      const msg = err.message || "Test failed";
      setTestWebhookFeedback(msg);
      pushToast("error", msg);
      setTimeout(() => setTestWebhookFeedback(""), 4000);
    } finally {
      setTestWebhookBusy(false);
    }
  };

  const buildRunSegments = (runs) => {
    if (!runs || runs.length === 0) return [];

    const sorted = [...runs].sort(
      (a, b) => new Date(a.at) - new Date(b.at)
    );

    const segments = [];
    let current = null;

    for (const r of sorted) {
      if (!current) {
        current = {
          status: r.status,
          startAt: r.at,
          endAt: r.at,
          count: 1,
          maxLateSeconds: r.late_by_seconds || 0,
        };
        continue;
      }

      if (r.status === current.status) {
        current.endAt = r.at;
        current.count += 1;
        const late = r.late_by_seconds || 0;
        if (late > current.maxLateSeconds) {
          current.maxLateSeconds = late;
        }
      } else {
        segments.push(current);
        current = {
          status: r.status,
          startAt: r.at,
          endAt: r.at,
          count: 1,
          maxLateSeconds: r.late_by_seconds || 0,
        };
      }
    }

    if (current) segments.push(current);

    return segments.reverse();
  };

  if (!user) return null;

  const filteredJobs = jobs.filter((j) => {
    if (statusFilter !== "all" && j.status !== statusFilter) return false;
    if (severityFilter !== "all" && (j.severity || "normal") !== severityFilter)
      return false;
    if (silencedOnly && !isJobSilenced(j)) return false;
    return true;
  });

  const runSegments = buildRunSegments(detailRuns);

  // Latest run (for logs/metadata view) – listJobRuns returns newest first
  const latestRun = detailRuns && detailRuns.length > 0 ? detailRuns[0] : null;

  // Derive logs + metadata from latestRun (handles snake_case + camelCase)
  const stdoutTailObj =
    latestRun?.stdout_tail || latestRun?.stdoutTail || null;
  const stderrTailObj =
    latestRun?.stderr_tail || latestRun?.stderrTail || null;

  const stdoutText =
    (stdoutTailObj && typeof stdoutTailObj === "object"
      ? stdoutTailObj.tail
      : stdoutTailObj) || "";
  const stderrText =
    (stderrTailObj && typeof stderrTailObj === "object"
      ? stderrTailObj.tail
      : stderrTailObj) || "";

  const stdoutTruncated =
    !!(stdoutTailObj && typeof stdoutTailObj === "object" && stdoutTailObj.truncated);
  const stderrTruncated =
    !!(stderrTailObj && typeof stderrTailObj === "object" && stderrTailObj.truncated);

  const latestDurationMs =
    latestRun && (latestRun.duration_ms ?? latestRun.durationMs);
  const latestExitCode =
    latestRun && (latestRun.exit_code ?? latestRun.exitCode);
  const latestEnv = latestRun && (latestRun.env || "");
  const latestTags = latestRun && (latestRun.tags || []);
  const latestAgentVersion =
    latestRun && (latestRun.agent_version ?? latestRun.agentVersion);

  const hasAnyLogs = !!(stdoutText || stderrText);

  const publicUrl =
    detailJob && detailJob.publicEnabled && detailJob.publicToken
      ? `${PUBLIC_BASE_URL}/p/${detailJob.publicToken}`
      : "";

  const handleCopyPublic = async () => {
    if (!publicUrl) return;
    try {
      await navigator.clipboard.writeText(publicUrl);
      setCopyFeedback("Copied");
      setTimeout(() => setCopyFeedback(""), 1500);
    } catch {
      setCopyFeedback("Copy failed");
      setTimeout(() => setCopyFeedback(""), 1500);
    }
  };

  const plan = account?.plan || null;
  const maxJobs = plan?.max_jobs ?? null;
  const jobsUsed = jobs.length;
  const emailAllowed = !plan || plan.allow_email_alerts;
  const publicAllowed = !plan || plan.allow_public_links;

  return (
    <div class="max-w-6xl mx-auto px-4 py-8">
      <div class="flex items-start justify-between gap-3 mb-1">
        <h1 class="text-2xl font-semibold text-slate-900">Monitored jobs</h1>
        <button
          type="button"
          onClick={() => setShowNewJobPanel(true)}
          class="shrink-0 px-3 py-1.5 rounded-full text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 shadow-sm"
        >
          + New job
        </button>
      </div>
      <p class="text-xs text-slate-500 mb-1">
        Define jobs, wire their heartbeats, and keep an eye on missed runs. This
        view refreshes automatically every ~15 seconds.
      </p>
      <p class="text-[11px] text-slate-500 mb-2">
        Webhook alerts are included by default. Email alerts and public status
        links depend on your plan.
      </p>
      {plan && (
        <p class="text-[11px] text-slate-500 mb-3">
          Plan: <span class="font-medium">{plan.label}</span>{" "}
          {typeof maxJobs === "number" && maxJobs > 0 && (
            <>
              • Jobs used:{" "}
              <span class="font-medium">
                {jobsUsed}/{maxJobs}
              </span>
            </>
          )}
        </p>
      )}
      {accountError && (
        <p class="text-[11px] text-amber-700 mb-2">
          {accountError} (plan info may be incomplete)
        </p>
      )}

      {/* New job slide-over panel -- opened via the "+ New job" button above
          rather than always taking up the top of the page, so the jobs list
          is the first thing visible on load. Non-blocking: clicking the
          backdrop or Escape dismisses it, same as the rest of the app's
          side-panel/toast pattern. */}
      {showNewJobPanel && (
        <div class="fixed inset-0 z-50 flex justify-end">
          <div
            class="absolute inset-0 bg-slate-900/30"
            onClick={() => setShowNewJobPanel(false)}
          />
          <div class="relative w-full max-w-md h-full bg-white shadow-2xl border-l border-slate-200 flex flex-col">
            <div class="flex items-center justify-between px-4 py-3 border-b border-slate-200 shrink-0">
              <h2 class="text-sm font-semibold text-slate-900">New job</h2>
              <button
                type="button"
                onClick={() => setShowNewJobPanel(false)}
                class="text-slate-400 hover:text-slate-600 text-xl leading-none px-1"
                aria-label="Close"
              >
                &times;
              </button>
            </div>
            <div class="flex-1 overflow-y-auto p-4">
        <form class="space-y-3" onSubmit={handleCreateJob}>
          <div>
            <label class="block text-xs text-slate-600 mb-1">Job name</label>
            <input
              type="text"
              class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={jobName}
              onInput={(e) => setJobName(e.target.value)}
              placeholder="Nightly orders sync"
            />
          </div>
          <div>
            <label class="block text-xs text-slate-600 mb-1">Project</label>
            <select
              class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
            >
              <option value="">Unassigned</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label class="block text-xs text-slate-600 mb-1">Schedule type</label>
            <div class="inline-flex rounded-full bg-slate-100 p-1">
              {[
                { key: "interval", label: "Interval" },
                { key: "cron", label: "Cron expression" },
              ].map((opt) => (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => setScheduleMode(opt.key)}
                  class={
                    "px-3 py-1 rounded-full text-xs font-medium transition-colors " +
                    (scheduleMode === opt.key
                      ? "bg-white shadow-sm text-slate-900"
                      : "text-slate-500 hover:text-slate-700")
                  }
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
          {scheduleMode === "cron" ? (
            <div>
              <label class="block text-xs text-slate-600 mb-1">
                Cron expression (UTC)
              </label>
              <input
                type="text"
                class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm font-mono text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={cronExpr}
                onInput={(e) => setCronExpr(e.target.value)}
                placeholder="0 3 * * *"
              />
              <p class="mt-1 text-[11px] text-slate-500">
                Standard 5-field syntax (minute hour day month weekday), e.g.{" "}
                <code>0 3 * * *</code> = daily at 3am UTC. The job is "late" if
                a heartbeat doesn't arrive by the next scheduled run plus the
                grace period below.
              </p>
            </div>
          ) : (
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
          )}
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
          <button
            type="submit"
            disabled={
              creating ||
              !jobName.trim() ||
              (scheduleMode === "cron"
                ? !cronExpr.trim()
                : !(Number(intervalMinutes) > 0))
            }
            class="w-full mt-1 py-2 rounded-lg text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60 shadow-sm"
          >
            {creating ? "Creating job…" : "Add job"}
          </button>
          {plan && typeof maxJobs === "number" && maxJobs > 0 && (
            <p class="mt-2 text-[11px] text-slate-500 text-center">
              Jobs: {jobsUsed}/{maxJobs} on {plan.label} plan
            </p>
          )}
        </form>
            </div>
          </div>
        </div>
      )}

      {/* Jobs + detail split */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
          <h2 class="text-sm font-semibold text-slate-900">Existing jobs</h2>
          <div class="flex items-center gap-2">
            <div class="inline-flex rounded-full bg-slate-100 p-1">
              {[
                { key: "all", label: "All" },
                { key: "ok", label: "Healthy" },
                { key: "late", label: "Late" },
                { key: "never_seen", label: "No data" },
              ].map((opt) => (
                <button
                  key={opt.key}
                  onClick={() => setStatusFilter(opt.key)}
                  class={
                    "px-2.5 py-1 rounded-full text-[11px] " +
                    (statusFilter === opt.key
                      ? "bg-white text-slate-900 shadow-sm"
                      : "text-slate-500 hover:text-slate-800")
                  }
                >
                  {opt.label}
                </button>
              ))}
            </div>
            {loadingJobs ? (
              <span class="text-[11px] text-slate-500">Loading…</span>
            ) : (
              <button
                onClick={() => loadJobs()}
                class="text-[11px] text-slate-500 hover:text-slate-800"
              >
                Refresh now
              </button>
            )}
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2 mb-3">
          <div class="inline-flex rounded-full bg-slate-100 p-1">
            {[{ key: "all", label: "Any severity" }, ...SEVERITY_OPTIONS].map(
              (opt) => (
                <button
                  key={opt.key}
                  onClick={() => setSeverityFilter(opt.key)}
                  class={
                    "px-2.5 py-1 rounded-full text-[11px] " +
                    (severityFilter === opt.key
                      ? "bg-white text-slate-900 shadow-sm"
                      : "text-slate-500 hover:text-slate-800")
                  }
                >
                  {opt.label}
                </button>
              )
            )}
          </div>
          <button
            onClick={() => setSilencedOnly((prev) => !prev)}
            class={
              "px-2.5 py-1 rounded-full text-[11px] border " +
              (silencedOnly
                ? "bg-amber-50 border-amber-300 text-amber-800"
                : "bg-white border-slate-200 text-slate-500 hover:text-slate-800")
            }
          >
            Silenced only
          </button>
          {(statusFilter !== "all" ||
            severityFilter !== "all" ||
            silencedOnly) && (
            <button
              onClick={() => {
                setStatusFilter("all");
                setSeverityFilter("all");
                setSilencedOnly(false);
              }}
              class="text-[11px] text-slate-400 hover:text-slate-700"
            >
              Clear filters
            </button>
          )}
        </div>

        {jobError && (
          <div class="text-[11px] text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2 mb-3">
            {jobError}
          </div>
        )}

        <div class="grid grid-cols-1 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1.5fr)] gap-4">
          {/* left: jobs table */}
          <div class="max-h-[26rem] overflow-y-auto border border-slate-200 rounded-xl">
            {loadingJobs ? (
              <p class="text-[11px] text-slate-500 p-3">Loading jobs…</p>
            ) : filteredJobs.length === 0 ? (
              <p class="text-[11px] text-slate-500 p-3">
                No jobs match this filter. Adjust your filter, or click "+ New
                job" above to create one.
              </p>
            ) : (
              <table class="w-full table-fixed text-[11px] text-left">
                <thead class="sticky top-0 bg-slate-50 border-b border-slate-200 text-slate-500">
                  <tr>
                    <th class="w-[28%] py-2 px-2 font-medium">Name</th>
                    <th class="w-[18%] py-2 px-2 font-medium">Status</th>
                    <th class="w-[30%] py-2 px-2 font-medium">Last heartbeat</th>
                    <th class="w-[24%] py-2 px-2 font-medium text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredJobs.map((job) => {
                    const isEditing = editingId === job.id;
                    const isDeleting = deletingId === job.id;
                    const selected = selectedJobId === job.id;

                    const jobSilenced = isJobSilenced(job);
                    const rawJobMaintenance =
                      job.maintenance_until || job.maintenanceUntil;

                    return (
                      <tr
                        key={job.id}
                        class={
                          "border-b border-slate-100 hover:bg-slate-50 cursor-pointer " +
                          (selected ? "bg-slate-50" : "")
                        }
                        onClick={() => handleRowClick(job)}
                      >
                        <td class="py-2 px-2 align-top">
                          {isEditing ? (
                            <input
                              type="text"
                              class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                              value={editName}
                              onClick={(e) => e.stopPropagation()}
                              onInput={(e) => setEditName(e.target.value)}
                            />
                          ) : (
                            <div class="flex flex-col">
                              <span class="text-slate-900 font-medium break-words">
                                {job.name}
                              </span>
                              <span class="text-[10px] text-slate-500">
                                {job.project_name || "Unassigned"}
                              </span>
                              <span class="text-[10px] text-slate-500 font-mono truncate max-w-[12rem]">
                                {job.heartbeat_url}
                              </span>
                              {job.runbook_url && (
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    try {
                                      window.open(
                                        job.runbook_url,
                                        "_blank",
                                        "noopener,noreferrer"
                                      );
                                    } catch {
                                      // ignore
                                    }
                                  }}
                                  class="mt-0.5 inline-flex items-center text-[9px] text-blue-600 hover:text-blue-700"
                                >
                                  <span class="w-1.5 h-1.5 rounded-full bg-blue-500 mr-1" />
                                  Runbook
                                </button>
                              )}
                            </div>
                          )}
                        </td>
                        <td class="py-2 px-2 align-top">
                          <div class="flex flex-col items-start gap-1">
                            {renderStatusPill(job.status)}
                            {renderSeverityPill(job.severity)}
                          </div>
                        </td>
                        <td class="py-2 px-2 align-top text-slate-700">
                          {formatTimestamp(job.last_heartbeat_at)}
                          {job.last_alert_sent_at && (
                            <div class="text-[10px] text-amber-700">
                              Last alert:{" "}
                              {formatTimestamp(job.last_alert_sent_at)}
                            </div>
                          )}
                          {jobSilenced && (
                            <div class="text-[10px] text-slate-500">
                              {job.maintenance_mode === "until_success"
                                ? "Silenced until next success"
                                : job.maintenance_mode === "manual"
                                ? "Silenced until unmuted"
                                : rawJobMaintenance && (
                                    <>
                                      Silenced until{" "}
                                      {formatShortDateTime(rawJobMaintenance)}
                                    </>
                                  )}
                            </div>
                          )}
                        </td>
                        <td class="py-2 px-2 align-top text-right">
                          <div class="flex flex-col items-stretch gap-1">
                            {!isEditing && (
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  // Schedule/grace/cron/project editing now
                                  // lives in the detail panel on the right,
                                  // so make sure it's showing this job.
                                  handleRowClick(job);
                                  startEdit(job);
                                }}
                                class="px-2 py-1 rounded-full border border-slate-300 text-[11px] text-slate-700 hover:bg-slate-100"
                              >
                                Edit
                              </button>
                            )}
                            {isEditing && (
                              <div class="flex flex-col gap-1">
                                <button
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    handleEditSave(job.id);
                                  }}
                                  disabled={editingBusy}
                                  class="px-2 py-1 rounded-full bg-blue-600 text-white text-[11px] hover:bg-blue-500 disabled:opacity-60"
                                >
                                  {editingBusy ? "Saving…" : "Save"}
                                </button>
                                <button
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    cancelEdit();
                                  }}
                                  class="px-2 py-1 rounded-full border border-slate-300 text-[11px] text-slate-600 hover:bg-slate-100"
                                >
                                  Cancel
                                </button>
                              </div>
                            )}
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                handleDeleteClick(job);
                              }}
                              disabled={isDeleting}
                              class={
                                "px-2 py-1 rounded-full border text-[11px] " +
                                (confirmDeleteId === job.id
                                  ? "border-red-500 bg-red-600 text-white hover:bg-red-500"
                                  : "border-red-200 text-red-600 hover:bg-red-50") +
                                (isDeleting ? " opacity-60" : "")
                              }
                            >
                              {isDeleting
                                ? "Deleting…"
                                : confirmDeleteId === job.id
                                ? "Confirm delete"
                                : "Delete"}
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>

          {/* right: detail panel */}
          <div class="border border-slate-200 rounded-xl p-3 bg-slate-50/60 min-h-[12rem]">
            {!selectedJobId ? (
              <div class="h-full flex items-center justify-center">
                <p class="text-[11px] text-slate-500 text-center max-w-xs">
                  Select a job from the table to see wiring snippets, recent
                  runs, uptime over the last 7 days, webhook deliveries, and the
                  latest logs from the RunGuard CLI.
                </p>
              </div>
            ) : detailLoading ? (
              <p class="text-[11px] text-slate-500">Loading job details…</p>
            ) : detailError ? (
              <div class="text-[11px] text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
                {detailError}
              </div>
            ) : !detailJob ? (
              <p class="text-[11px] text-slate-500">No job details.</p>
            ) : (
              <div class="space-y-3">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <div class="text-xs font-semibold text-slate-900 mb-0.5">
                      {detailJob.name}
                    </div>
                    <div class="text-[10px] text-slate-500 font-mono break-all">
                      {detailJob.heartbeat_url}
                    </div>
                    <div class="text-[10px] text-slate-500 mt-1">
                      Last heartbeat:{" "}
                      <span class="text-slate-700">
                        {formatTimestamp(detailJob.last_heartbeat_at)}
                      </span>
                    </div>
                    {editingId === detailJob.id ? (
                      <div class="mt-1 space-y-1.5 bg-white border border-slate-200 rounded-lg p-2">
                        <div>
                          <label class="block text-[10px] text-slate-500 mb-0.5">
                            Interval (min)
                          </label>
                          <input
                            type="number"
                            min="1"
                            class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            value={editInterval}
                            onInput={(e) => setEditInterval(e.target.value)}
                          />
                        </div>
                        <div>
                          <label class="block text-[10px] text-slate-500 mb-0.5">
                            Grace (min)
                          </label>
                          <input
                            type="number"
                            min="0"
                            class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            value={editGrace}
                            onInput={(e) => setEditGrace(e.target.value)}
                          />
                        </div>
                        <div>
                          <label class="block text-[10px] text-slate-500 mb-0.5">
                            Cron override (optional)
                          </label>
                          <input
                            type="text"
                            class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] font-mono text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            value={editCronExpr}
                            onInput={(e) => setEditCronExpr(e.target.value)}
                            placeholder="e.g. 0 3 * * * (blank = use interval)"
                          />
                        </div>
                        <div>
                          <label class="block text-[10px] text-slate-500 mb-0.5">
                            Project
                          </label>
                          <select
                            class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            value={editProjectId}
                            onChange={(e) => setEditProjectId(e.target.value)}
                          >
                            <option value="">Unassigned</option>
                            {projects.map((project) => (
                              <option key={project.id} value={project.id}>
                                {project.name}
                              </option>
                            ))}
                          </select>
                        </div>
                        <p class="text-[10px] text-slate-500">
                          Use Save/Cancel in the jobs table row to apply or
                          discard these changes.
                        </p>
                      </div>
                    ) : (
                      <div class="text-[10px] text-slate-500">
                        {detailJob.cron_expr ? (
                          <>
                            Schedule{" "}
                            <span class="font-mono text-slate-700">
                              {detailJob.cron_expr}
                            </span>{" "}
                            (UTC), grace{" "}
                            {Math.round(detailJob.grace_seconds / 60)} min
                          </>
                        ) : (
                          <>
                            Interval ~{" "}
                            {Math.round(
                              detailJob.expected_interval_seconds / 60
                            )}{" "}
                            min, grace{" "}
                            {Math.round(detailJob.grace_seconds / 60)} min
                          </>
                        )}
                      </div>
                    )}
                    <div class="mt-1.5 flex items-center gap-1.5">
                      <span class="text-[10px] text-slate-500">Alerts:</span>
                      <button
                        type="button"
                        onClick={() => {
                          if (!emailAllowed) {
                            const msg =
                              "Email alerts are not available on your current plan.";
                            setJobError(msg);
                            pushToast("info", msg);
                            return;
                          }
                          handleToggleEmail(detailJob);
                        }}
                        disabled={
                          togglingEmailId === detailJob.id || !emailAllowed
                        }
                        class={
                          "px-2 py-0.5 rounded-full text-[10px] border " +
                          (emailAllowed
                            ? detailJob.alert_email_enabled
                              ? "bg-blue-50 border-blue-200 text-blue-700 hover:bg-blue-100"
                              : "bg-slate-50 border-slate-200 text-slate-500 hover:bg-slate-100"
                            : "bg-slate-50 border-slate-200 text-slate-400 cursor-not-allowed")
                        }
                        title={
                          emailAllowed
                            ? ""
                            : "Upgrade your plan to enable email alerts"
                        }
                      >
                        {togglingEmailId === detailJob.id
                          ? "Updating…"
                          : !emailAllowed
                          ? "Email (upgrade)"
                          : detailJob.alert_email_enabled
                          ? "Email on"
                          : "Email off"}
                      </button>
                      <button
                        type="button"
                        onClick={() => handleToggleWebhook(detailJob)}
                        disabled={togglingWebhookId === detailJob.id}
                        class={
                          "px-2 py-0.5 rounded-full text-[10px] border " +
                          (detailJob.alert_webhook_enabled
                            ? "bg-emerald-50 border-emerald-200 text-emerald-700 hover:bg-emerald-100"
                            : "bg-slate-50 border-slate-200 text-slate-500 hover:bg-slate-100")
                        }
                      >
                        {togglingWebhookId === detailJob.id
                          ? "Updating…"
                          : detailJob.alert_webhook_enabled
                          ? "Webhooks on"
                          : "Webhooks off"}
                      </button>
                    </div>
                    <div class="mt-1 flex items-center gap-2">
                      <span class="text-[10px] text-slate-500">Severity:</span>
                      {renderSeverityPill(detailJob.severity)}
                      <div class="inline-flex rounded-full bg-slate-100 p-0.5 ml-1">
                        {SEVERITY_OPTIONS.map((opt) => (
                          <button
                            type="button"
                            key={opt.key}
                            onClick={() =>
                              detailJob.severity !== opt.key &&
                              handleChangeSeverity(detailJob, opt.key)
                            }
                            class={
                              "px-2 py-0.5 rounded-full text-[10px] " +
                              (detailJob.severity === opt.key ||
                              (!detailJob.severity && opt.key === "normal")
                                ? "bg-white text-slate-900 shadow-sm"
                                : "text-slate-500 hover:text-slate-800")
                            }
                          >
                            {opt.label}
                          </button>
                        ))}
                      </div>
                    </div>
                    {detailStats && (
                      <div class="text-[10px] text-slate-500 mt-0.5 space-y-0.5">
                        <div>
                          Last 7 days:&nbsp;
                          <span
                            class={
                              "font-medium " +
                              (detailStats.uptime_percent >= 99
                                ? "text-emerald-700"
                                : detailStats.uptime_percent >= 90
                                ? "text-amber-700"
                                : "text-red-700")
                            }
                          >
                            {detailStats.uptime_percent.toFixed(1)}% on-time
                          </span>
                          {detailStats.total > 0 && (
                            <span class="ml-1 text-[10px] text-slate-500">
                              ({detailStats.ok} ok, {detailStats.late} late,{" "}
                              {detailStats.fail || 0} fail,{" "}
                              {detailStats.total} runs)
                            </span>
                          )}
                        </div>
                        {Array.isArray(detailStats.windows) &&
                          detailStats.windows
                            .filter((w) => w.days !== 7)
                            .map((w) => (
                              <div key={w.days}>
                                Last {w.days} days:&nbsp;
                                <span
                                  class={
                                    "font-medium " +
                                    (w.uptime_percent >= 99
                                      ? "text-emerald-700"
                                      : w.uptime_percent >= 90
                                      ? "text-amber-700"
                                      : "text-red-700")
                                  }
                                >
                                  {w.uptime_percent.toFixed(1)}% on-time
                                </span>
                                {w.total > 0 && (
                                  <span class="ml-1 text-[10px] text-slate-500">
                                    ({w.ok} ok, {w.late} late, {w.fail} fail,{" "}
                                    {w.total} runs)
                                  </span>
                                )}
                              </div>
                            ))}
                      </div>
                    )}
                  </div>
                  <div class="flex flex-col items-end gap-1">
                    {renderStatusPill(detailJob.status)}
                    <button
                      onClick={() => loadDetail(detailJob.id)}
                      class="text-[10px] text-slate-500 hover:text-slate-800"
                    >
                      Refresh
                    </button>
                    <p class="text-[10px] text-slate-500 text-right max-w-[11rem]">
                      Email alerts, public links, exports, and log capture vary
                      by plan. Webhook alerts are included on all tiers.
                    </p>
                  </div>
                </div>

                {/* Maintenance / silence block */}
                {(() => {
                  const now = new Date();
                  const rawMaintenance =
                    detailJob.maintenance_until || detailJob.maintenanceUntil;
                  const mode = detailJob.maintenance_mode || "";

                  let silenced = false;
                  let label = "No active maintenance window.";
                  let activeMinutes = 0;

                  if (mode === "until_success") {
                    silenced = true;
                    label = "Alerts silenced until the next successful heartbeat.";
                  } else if (mode === "manual") {
                    silenced = true;
                    label = "Alerts silenced until manually unmuted.";
                  } else if (rawMaintenance) {
                    const until = new Date(rawMaintenance);
                    if (!isNaN(until.getTime()) && until > now) {
                      silenced = true;
                      const diffMin = Math.round((until - now) / 60000);
                      activeMinutes = diffMin;

                      const untilLabel = until.toLocaleString(undefined, {
                        month: "short",
                        day: "2-digit",
                        hour: "2-digit",
                        minute: "2-digit",
                      });
                      label = `Alerts silenced until ${untilLabel}.`;
                    }
                  }

                  const maintenancePresets = [
                    { key: "off", label: "Off", minutes: 0 },
                    { key: "1h", label: "1 hour", minutes: 60 },
                    { key: "4h", label: "4 hours", minutes: 4 * 60 },
                    { key: "24h", label: "24 hours", minutes: 24 * 60 },
                    { key: "7d", label: "7 days", minutes: 7 * 24 * 60 },
                  ];

                  const isPresetActive = (opt) => {
                    if (mode === "until_success" || mode === "manual") return false;
                    if (opt.minutes === 0) {
                      return !silenced;
                    }
                    if (!silenced) return false;
                    const diff = Math.abs(activeMinutes - opt.minutes);
                    return diff <= 10;
                  };

                  const modeOptions = [
                    { key: "until_success", label: "Until next success" },
                    { key: "manual", label: "Until unmuted" },
                  ];

                  return (
                    <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                      <div class="flex items-center justify-between mb-1.5">
                        <h3 class="text-[11px] font-semibold text-slate-900">
                          Maintenance / silence
                        </h3>
                        <span
                          class={
                            "inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border " +
                            (silenced
                              ? "bg-amber-50 border-amber-200 text-amber-700"
                              : "bg-slate-50 border-slate-200 text-slate-600")
                          }
                        >
                          <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
                          {silenced ? "Active" : "Off"}
                        </span>
                      </div>
                      <p class="text-[10px] text-slate-500 mb-1">
                        Use this to temporarily pause alerts during maintenance
                        or noisy deploys. Heartbeats and stats continue to be
                        recorded.
                      </p>
                      <p class="text-[10px] text-slate-600 mb-2">{label}</p>
                      <div class="flex flex-wrap items-center gap-1.5">
                        <div class="inline-flex rounded-full bg-slate-100 p-1">
                          {maintenancePresets.map((opt) => (
                            <button
                              key={opt.key}
                              type="button"
                              onClick={() =>
                                handleSetMaintenance(detailJob, opt.minutes)
                              }
                              class={
                                "px-2 py-0.5 rounded-full text-[10px] " +
                                (isPresetActive(opt)
                                  ? "bg-white text-slate-900 shadow-sm"
                                  : "text-slate-500 hover:text-slate-800")
                              }
                            >
                              {opt.label}
                            </button>
                          ))}
                        </div>
                        <div class="inline-flex rounded-full bg-slate-100 p-1">
                          {modeOptions.map((opt) => (
                            <button
                              key={opt.key}
                              type="button"
                              onClick={() =>
                                handleSetMaintenanceMode(detailJob, opt.key)
                              }
                              class={
                                "px-2 py-0.5 rounded-full text-[10px] " +
                                (mode === opt.key
                                  ? "bg-white text-slate-900 shadow-sm"
                                  : "text-slate-500 hover:text-slate-800")
                              }
                            >
                              {opt.label}
                            </button>
                          ))}
                        </div>
                        {silenced && (
                          <button
                            type="button"
                            onClick={() => handleSetMaintenance(detailJob, 0)}
                            class="px-2 py-0.5 rounded-full text-[10px] border border-amber-300 text-amber-800 hover:bg-amber-50"
                          >
                            Unmute now
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })()}

                {/* Per-job run history export */}
                {(!plan || plan.allow_exports) && (
                  <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                    <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
                      Export run history
                    </h3>
                    <p class="text-[10px] text-slate-500 mb-2">
                      Download this job's own run history (status, exit code,
                      duration, tags) as CSV or Excel.
                    </p>
                    <div class="flex flex-wrap gap-2">
                      <a
                        href={`/api/jobs/${detailJob.id}/export/runs.csv`}
                        class="px-3 py-1.5 rounded-full text-[11px] bg-slate-900 text-white hover:bg-slate-800"
                      >
                        Download CSV
                      </a>
                      <a
                        href={`/api/jobs/${detailJob.id}/export/runs.xlsx`}
                        class="px-3 py-1.5 rounded-full text-[11px] bg-slate-100 text-slate-900 border border-slate-200 hover:bg-slate-200"
                      >
                        Download Excel
                      </a>
                    </div>
                  </div>
                )}

                {/* Runbook / notes */}
                <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                  <div class="flex items-center justify-between mb-1.5">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Runbook & notes
                    </h3>
                    {detailJob.runbook_url && (
                      <button
                        type="button"
                        onClick={() => {
                          try {
                            window.open(
                              detailJob.runbook_url,
                              "_blank",
                              "noopener,noreferrer"
                            );
                          } catch {
                            // ignore
                          }
                        }}
                        class="text-[10px] text-blue-600 hover:text-blue-700"
                      >
                        Open runbook
                      </button>
                    )}
                  </div>
                  <p class="text-[10px] text-slate-500 mb-2">
                    Short link and notes for future you. Stored only inside
                    RunGuard and never exposed via public links or webhooks.
                  </p>
                  <div class="space-y-2">
                    <div>
                      <label class="block text-[10px] text-slate-600 mb-0.5">
                        Runbook URL
                      </label>
                      <input
                        type="text"
                        class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                        placeholder="https://internal/wiki/job-runbook"
                        value={runbookUrl}
                        onInput={(e) => setRunbookUrl(e.target.value)}
                      />
                    </div>
                    <div>
                      <label class="block text-[10px] text-slate-600 mb-0.5">
                        Notes
                      </label>
                      <textarea
                        rows={3}
                        class="w-full px-2 py-1 rounded bg-white border border-slate-300 text-[11px] text-slate-900 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-y"
                        placeholder="Where the job runs, common failure modes, quick checks…"
                        value={runbookNotes}
                        onInput={(e) => setRunbookNotes(e.target.value)}
                      />
                    </div>
                  </div>
                  <div class="flex items-center justify-between mt-2">
                    <span class="text-[10px] text-slate-500">
                      Good for “how to fix it at 03:00” so you don&apos;t have
                      to dig through old tickets.
                    </span>
                    <button
                      type="button"
                      onClick={handleSaveRunbook}
                      disabled={runbookSaving}
                      class="px-3 py-1 rounded-full text-[10px] bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60"
                    >
                      {runbookSaving ? "Saving…" : "Save"}
                    </button>
                  </div>
                </div>

                {/* Public link block */}
                <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                  <div class="flex items-center justify-between mb-1.5">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Public status link
                    </h3>
                    <button
                      type="button"
                      onClick={() => {
                        if (!publicAllowed) {
                          const msg =
                            "Public status links are not available on your current plan.";
                          setJobError(msg);
                          pushToast("info", msg);
                          return;
                        }
                        handleTogglePublic(detailJob);
                      }}
                      disabled={
                        togglingPublicId === detailJob.id || !publicAllowed
                      }
                      class={
                        "inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border " +
                        (publicAllowed
                          ? detailJob.publicEnabled
                            ? "bg-emerald-50 border-emerald-200 text-emerald-700"
                            : "bg-slate-50 border-slate-200 text-slate-500"
                          : "bg-slate-50 border-slate-200 text-slate-400 cursor-not-allowed")
                      }
                      title={
                        publicAllowed
                          ? ""
                          : "Upgrade your plan to enable public status links"
                      }
                    >
                      {togglingPublicId === detailJob.id
                        ? "Updating…"
                        : !publicAllowed
                        ? "Public (upgrade)"
                        : detailJob.publicEnabled
                        ? "Public on"
                        : "Public off"}
                    </button>
                  </div>
                  <p class="text-[10px] text-slate-500 mb-1.5">
                    When enabled, this job’s high-level health is available at a
                    read-only URL on{" "}
                    <span class="font-mono">
                      {PUBLIC_BASE_URL.replace(/^https?:\/\//, "")}
                    </span>
                    . No credentials, IPs, or internals are exposed.
                  </p>
                  {detailJob.publicEnabled && publicUrl ? (
                    <div class="flex items-center gap-2">
                      <div class="flex-1 bg-slate-50 border border-slate-200 rounded px-2 py-1">
                        <span class="text-[10px] font-mono break-all text-slate-800">
                          {publicUrl}
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={handleCopyPublic}
                        class="px-2 py-1 rounded-lg text-[10px] bg-slate-800 text-white hover:bg-slate-700"
                      >
                        {copyFeedback || "Copy"}
                      </button>
                    </div>
                  ) : (
                    <p class="text-[10px] text-slate-400 italic">
                      Public link is currently disabled.
                    </p>
                  )}
                </div>

                {/* Webhook templates (selection only) */}
                <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                  <div class="flex items-center justify-between mb-1.5">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Webhook endpoints
                    </h3>
                    {detailJob.alert_webhook_enabled ? (
                      <span class="text-[10px] text-emerald-700">
                        Webhooks on
                      </span>
                    ) : (
                      <span class="text-[10px] text-slate-500">
                        Webhooks off
                      </span>
                    )}
                  </div>

                  <p class="text-[10px] text-slate-500 mb-2">
                    Attach one or more saved webhook templates to this job.
                    Manage templates centrally on the Webhooks page; changes
                    there apply to all jobs using them.
                  </p>

                  {webhookTemplatesError && (
                    <p class="text-[10px] text-amber-700 mb-1">
                      {webhookTemplatesError}
                    </p>
                  )}

                  {webhookTemplatesLoading ? (
                    <p class="text-[10px] text-slate-500 mb-2">
                      Loading templates…
                    </p>
                  ) : webhookTemplates.length === 0 ? (
                    <p class="text-[10px] text-slate-500 mb-2">
                      No webhook templates yet. Define them on the Webhooks
                      page, then return here to attach them to this job.
                    </p>
                  ) : (
                    <div class="mb-1 max-h-24 overflow-y-auto border border-slate-200 rounded-md">
                      <table class="w-full text-[10px] text-left">
                        <thead class="bg-slate-50 border-b border-slate-200 text-slate-500">
                          <tr>
                            <th class="py-1 px-2 font-medium">Name</th>
                            <th class="py-1 px-2 font-medium">
                              Use on this job
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {webhookTemplates.map((tmpl) => {
                            const urls = jobWebhookUrls(detailJob);
                            const attached = urls.includes(tmpl.url);
                            const shortUrl =
                              tmpl.url.length > 60
                                ? tmpl.url.slice(0, 57) + "…"
                                : tmpl.url;

                            const typeLabel =
                              tmpl.type === "slack" ? "Slack" : "Generic JSON";
                            const typeClasses =
                              "inline-flex items-center px-1.5 py-0.5 rounded-full text-[9px] border " +
                              (tmpl.type === "slack"
                                ? "bg-purple-50 border-purple-200 text-purple-700"
                                : "bg-slate-50 border-slate-200 text-slate-600");

                            return (
                              <tr
                                key={tmpl.id}
                                class="border-b border-slate-100 last:border-0"
                              >
                                <td class="py-1 px-2">
                                  <div class="flex flex-col gap-0.5">
                                    <div class="flex items-center gap-1">
                                      <span class="text-[10px] text-slate-800 font-medium">
                                        {tmpl.name}
                                      </span>
                                      <span class={typeClasses}>
                                        {typeLabel}
                                      </span>
                                    </div>
                                    <span
                                      class="text-[9px] text-slate-500 font-mono truncate max-w-[12rem]"
                                      title={tmpl.url}
                                    >
                                      {shortUrl}
                                    </span>
                                  </div>
                                </td>
                                <td class="py-1 px-2">
                                  <button
                                    type="button"
                                    onClick={() =>
                                      handleToggleTemplateForJob(
                                        detailJob,
                                        tmpl
                                      )
                                    }
                                    class={
                                      "px-2 py-0.5 rounded-full text-[10px] border " +
                                      (attached
                                        ? "bg-emerald-50 border-emerald-200 text-emerald-700"
                                        : "bg-slate-50 border-slate-200 text-slate-500 hover:bg-slate-100")
                                    }
                                  >
                                    {attached ? "Using" : "Attach"}
                                  </button>
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}

                  <p class="text-[9px] text-slate-500 mt-1">
                    To add, edit, or remove templates, use the central Webhooks
                    configuration page.
                  </p>
                </div>

                {/* snippets */}
                <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                  <div class="flex items-center justify-between mb-1.5">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Wiring snippet
                    </h3>
                    <div class="inline-flex rounded-full bg-slate-100 p-1">
                      {SNIPPET_TYPES.map((type) => (
                        <button
                          key={type}
                          onClick={() => setSnippetType(type)}
                          class={
                            "px-2 py-1 rounded-full text-[10px] capitalize " +
                            (snippetType === type
                              ? "bg-white text-slate-900 shadow-sm"
                              : "text-slate-500 hover:text-slate-800")
                          }
                        >
                          {type.replace("-", " ")}
                        </button>
                      ))}
                    </div>
                  </div>
                  <p class="text-[10px] text-slate-500 mb-2">
                    Copy the snippet that matches your environment. All of them
                    simply ensure this URL is called on success.
                  </p>
                  <div class="bg-slate-50 border border-slate-200 rounded-md p-2 text-[11px] font-mono overflow-x-auto">
                    <pre class="whitespace-pre leading-snug">
                      {renderSnippet()}
                    </pre>
                  </div>
                </div>

                {/* runs as segments */}
                <div>
                  <div class="flex items-center justify-between mb-1">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Recent runs (compressed)
                    </h3>
                    <span class="text-[10px] text-slate-500">
                      {runSegments.length} segment
                      {runSegments.length === 1 ? "" : "s"}
                    </span>
                  </div>
                  {runSegments.length === 0 ? (
                    <p class="text-[10px] text-slate-500">
                      No recorded runs yet. Once the job sends heartbeats or
                      goes late enough to trigger an alert, they will appear
                      here.
                    </p>
                  ) : (
                    <div class="max-h-40 overflow-y-auto border border-slate-200 rounded-md">
                      <table class="w-full text-[10px] text-left">
                        <thead class="bg-slate-50 border-b border-slate-200 text-slate-500">
                          <tr>
                            <th class="py-1 px-2 font-medium">Period</th>
                            <th class="py-1 px-2 font-medium">Status</th>
                            <th class="py-1 px-2 font-medium">Count</th>
                            <th class="py-1 px-2 font-medium">Late by</th>
                          </tr>
                        </thead>
                        <tbody>
                          {runSegments.map((seg, idx) => {
                            const isOk = seg.status === "ok";

                            let period = "";
                            if (seg.startAt === seg.endAt) {
                              period = formatShortDateTime(seg.startAt);
                            } else {
                              const sameDay =
                                new Date(seg.startAt).toDateString() ===
                                new Date(seg.endAt).toDateString();
                              period = sameDay
                                ? `${formatShortTime(
                                    seg.startAt
                                  )}–${formatShortTime(seg.endAt)} (${new Date(
                                    seg.startAt
                                  ).toLocaleDateString(undefined, {
                                    month: "short",
                                    day: "2-digit",
                                  })})`
                                : `${formatShortDateTime(
                                    seg.startAt
                                  )} → ${formatShortDateTime(seg.endAt)}`;
                            }

                            return (
                              <tr
                                key={idx}
                                class="border-b border-slate-100 last:border-0"
                              >
                                <td class="py-1 px-2 text-slate-700">
                                  {period}
                                </td>
                                <td class="py-1 px-2">
                                  {isOk ? (
                                    <span class="inline-flex items-center px-2 py-0.5 rounded-full bg-emerald-50 border border-emerald-200 text-emerald-700">
                                      <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
                                      Healthy
                                    </span>
                                  ) : (
                                    <span class="inline-flex items-center px-2 py-0.5 rounded-full bg-amber-50 border border-amber-200 text-amber-700">
                                      <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
                                      Late
                                    </span>
                                  )}
                                </td>
                                <td class="py-1 px-2 text-slate-700">
                                  {seg.count}
                                </td>
                                <td class="py-1 px-2 text-slate-700">
                                  {isOk
                                    ? "—"
                                    : seg.maxLateSeconds
                                    ? `max ${seg.maxLateSeconds}s`
                                    : "n/a"}
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>

                {/* latest run logs & metadata */}
                <div class="bg-white border border-slate-200 rounded-lg p-2.5">
                  <div class="flex items-center justify-between mb-1.5">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Latest run details & logs
                    </h3>
                    {latestRun ? (
                      <span class="text-[10px] text-slate-500">
                        {formatShortDateTime(latestRun.at)}
                      </span>
                    ) : (
                      <span class="text-[10px] text-slate-500">
                        No runs recorded yet
                      </span>
                    )}
                  </div>
                  {!latestRun ? (
                    <p class="text-[10px] text-slate-500">
                      Once the job has run at least once, the most recent
                      execution will appear here, including logs from the
                      RunGuard CLI if available.
                    </p>
                  ) : (
                    <>
                      <div class="text-[10px] text-slate-600 mb-2 space-y-0.5">
                        <div>
                          Status:{" "}
                          <span class="font-medium text-slate-800">
                            {latestRun.status || "unknown"}
                          </span>
                          {typeof latestExitCode === "number" && (
                            <span class="ml-2">
                              • Exit code:{" "}
                              <span class="font-mono">
                                {latestExitCode}
                              </span>
                            </span>
                          )}
                          {typeof latestDurationMs === "number" && (
                            <span class="ml-2">
                              • Duration:{" "}
                              <span class="font-mono">
                                {latestDurationMs} ms
                              </span>
                            </span>
                          )}
                        </div>
                        {(latestEnv || (latestTags && latestTags.length > 0)) && (
                          <div>
                            {latestEnv && (
                              <>
                                Env:{" "}
                                <span class="font-mono text-slate-800">
                                  {latestEnv}
                                </span>
                              </>
                            )}
                            {latestTags && latestTags.length > 0 && (
                              <>
                                {" "}
                                • Tags:{" "}
                                {latestTags.map((t) => (
                                  <span
                                    key={t}
                                    class="inline-flex items-center px-1.5 py-0.5 rounded-full text-[9px] bg-slate-100 text-slate-700 border border-slate-200 mr-1"
                                  >
                                    {t}
                                  </span>
                                ))}
                              </>
                            )}
                          </div>
                        )}
                        {latestAgentVersion && (
                          <div>
                            Agent:{" "}
                            <span class="font-mono text-slate-800">
                              runguard {latestAgentVersion}
                            </span>
                          </div>
                        )}
                      </div>

                      {hasAnyLogs ? (
                        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                          <div>
                            <div class="flex items-center justify-between mb-1">
                              <span class="text-[10px] font-semibold text-slate-800">
                                stdout tail
                              </span>
                              {stdoutTruncated && (
                                <span class="text-[9px] text-slate-500">
                                  truncated
                                </span>
                              )}
                            </div>
                            <div class="bg-slate-50 border border-slate-200 rounded-md p-1.5 text-[10px] font-mono max-h-32 overflow-auto whitespace-pre-wrap">
                              {stdoutText || (
                                <span class="text-slate-400">
                                  (no stdout captured)
                                </span>
                              )}
                            </div>
                          </div>
                          <div>
                            <div class="flex items-center justify-between mb-1">
                              <span class="text-[10px] font-semibold text-slate-800">
                                stderr tail
                              </span>
                              {stderrTruncated && (
                                <span class="text-[9px] text-slate-500">
                                  truncated
                                </span>
                              )}
                            </div>
                            <div class="bg-slate-50 border border-slate-200 rounded-md p-1.5 text-[10px] font-mono max-h-32 overflow-auto whitespace-pre-wrap">
                              {stderrText || (
                                <span class="text-slate-400">
                                  (no stderr captured)
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      ) : (
                        <p class="text-[10px] text-slate-500">
                          This run did not include captured logs. To see stdout
                          and stderr tails here, invoke your job via the
                          RunGuard CLI wrapper instead of a plain curl, and
                          ensure your plan allows log capture for this job.
                        </p>
                      )}
                    </>
                  )}
                </div>

                {/* webhook deliveries */}
                <div class="mt-3">
                  <div class="flex items-center justify-between mb-1">
                    <h3 class="text-[11px] font-semibold text-slate-900">
                      Webhook deliveries
                    </h3>
                    <div class="flex items-center gap-2">
                      <span class="text-[10px] text-slate-500">
                        {detailWebhookLogs.length} attempt
                        {detailWebhookLogs.length === 1 ? "" : "s"}
                      </span>
                      <button
                        type="button"
                        onClick={handleTestWebhooks}
                        disabled={testWebhookBusy}
                        class={
                          "px-2 py-0.5 rounded-full text-[10px] border " +
                          (testWebhookBusy
                            ? "bg-slate-100 border-slate-200 text-slate-400 cursor-wait"
                            : "bg-white border-slate-300 text-slate-700 hover:bg-slate-50")
                        }
                        title="Send a one-off test event to all configured webhook URLs for this job"
                      >
                        {testWebhookBusy ? "Testing…" : "Test webhook"}
                      </button>
                    </div>
                  </div>

                  {testWebhookFeedback && (
                    <p class="text-[10px] text-slate-600 mb-1">
                      {testWebhookFeedback}
                    </p>
                  )}

                  {detailWebhookError && (
                    <p class="text-[10px] text-amber-700 mb-1">
                      {detailWebhookError}
                    </p>
                  )}

                  {detailWebhookLogs.length === 0 ? (
                    <p class="text-[10px] text-slate-500">
                      No webhook deliveries recorded yet for this job. Once late
                      alerts fire and webhooks are configured, they will appear
                      here.
                    </p>
                  ) : (
                    <div class="max-h-40 overflow-y-auto border border-slate-200 rounded-md">
                      <table class="w-full text-[10px] text-left">
                        <thead class="bg-slate-50 border-b border-slate-200 text-slate-500">
                          <tr>
                            <th class="py-1 px-2 font-medium">When</th>
                            <th class="py-1 px-2 font-medium">Status</th>
                            <th class="py-1 px-2 font-medium">HTTP</th>
                            <th class="py-1 px-2 font-medium">Duration</th>
                            <th class="py-1 px-2 font-medium">URL</th>
                            <th class="py-1 px-2 font-medium">Error</th>
                          </tr>
                        </thead>
                        <tbody>
                          {detailWebhookLogs.map((log) => {
                            const when = formatShortDateTime(log.created_at);
                            const duration = log.duration_ms
                              ? `${log.duration_ms} ms`
                              : "—";

                            let shortUrl = "—";
                            if (log.url) {
                              try {
                                const u = new URL(log.url);
                                const full = u.host + u.pathname;
                                shortUrl =
                                  full.length > 60
                                    ? full.slice(0, 57) + "…"
                                    : full;
                              } catch {
                                shortUrl =
                                  log.url.length > 60
                                    ? log.url.slice(0, 57) + "…"
                                    : log.url;
                              }
                            }

                            const errText = log.error || "";

                            return (
                              <tr
                                key={log.id}
                                class="border-b border-slate-100 last:border-0"
                              >
                                <td class="py-1 px-2 text-slate-700">
                                  {when}
                                </td>
                                <td class="py-1 px-2">
                                  {renderWebhookStatusPill(log.status)}
                                </td>
                                <td class="py-1 px-2 text-slate-700">
                                  {log.code || "—"}
                                </td>
                                <td class="py-1 px-2 text-slate-700">
                                  {duration}
                                </td>
                                <td
                                  class="py-1 px-2 text-slate-700 max-w-[10rem] truncate"
                                  title={log.url}
                                >
                                  {shortUrl}
                                </td>
                                <td
                                  class="py-1 px-2 text-slate-700 max-w-[10rem] truncate"
                                  title={errText}
                                >
                                  {errText || "—"}
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Toast notifications */}
      <div class="fixed bottom-4 right-4 z-50 space-y-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            class={
              "max-w-xs rounded-xl border px-3 py-2 text-xs shadow-lg bg-white " +
              (t.variant === "success"
                ? "border-emerald-200 text-emerald-800"
                : t.variant === "error"
                ? "border-rose-200 text-rose-800"
                : "border-slate-200 text-slate-800")
            }
          >
            {t.message}
          </div>
        ))}
      </div>
    </div>
  );
}
