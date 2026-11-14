import { route } from "preact-router";
import { useEffect, useState } from "preact/hooks";
import VerifyEmailBanner from "../components/VerifyEmailBanner.jsx";
import { getAccount, getAgentToken, logoutAllDevices, regenerateAgentToken, updateAccount } from "../api.js";
import { showToast } from "../components/Toast.jsx";

export default function AccountPage({ user, onLogout }) {
  const [account, setAccount] = useState(null);
  const [loadingAccount, setLoadingAccount] = useState(true);
  const [accountError, setAccountError] = useState("");
  const [savingSummary, setSavingSummary] = useState(false);
  const [savingWeeklySummary, setSavingWeeklySummary] = useState(false);
  const [loggingOutAll, setLoggingOutAll] = useState(false);
  const [logoutAllMessage, setLogoutAllMessage] = useState("");

  const [agentToken, setAgentToken] = useState("");
  const [agentTokenVisible, setAgentTokenVisible] = useState(false);
  const [agentTokenLoading, setAgentTokenLoading] = useState(false);
  const [agentTokenError, setAgentTokenError] = useState("");
  const [agentTokenCopied, setAgentTokenCopied] = useState(false);

  useEffect(() => { if (!user) route("/", true); }, [user]);
  useEffect(() => {
    async function loadAccount() {
      try {
        setAccountError("");
        setAccount(await getAccount());
      } catch (err) {
        console.error("load account", err);
        setAccountError(err.message || "Failed to load account");
      } finally {
        setLoadingAccount(false);
      }
    }
    loadAccount();
  }, []);

  async function handleToggleSummary() {
    if (!account) return;
    setSavingSummary(true);
    setAccountError("");
    try {
      const updated = await updateAccount({ summary_email_enabled: !account.summary_email_enabled });
      setAccount(updated);
    } catch (err) {
      setAccountError(err.message || "Failed to update account");
    } finally {
      setSavingSummary(false);
    }
  }

  async function handleToggleWeeklySummary() {
    if (!account) return;
    setSavingWeeklySummary(true);
    setAccountError("");
    try {
      const updated = await updateAccount({ weekly_summary_enabled: !account.weekly_summary_enabled });
      setAccount(updated);
    } catch (err) {
      setAccountError(err.message || "Failed to update account");
    } finally {
      setSavingWeeklySummary(false);
    }
  }

  const handleExport = (kind) => {
    window.location.href = `/api/exports/jobs.${kind}`;
  };

  async function handleLogoutAll() {
    setLoggingOutAll(true);
    setLogoutAllMessage("");
    try {
      await logoutAllDevices();
      setLogoutAllMessage("Done — every other signed-in session/device has been logged out.");
    } catch (err) {
      setAccountError(err.message || "Failed to log out other sessions");
    } finally {
      setLoggingOutAll(false);
    }
  }

  async function handleShowAgentToken() {
    if (agentToken) {
      setAgentTokenVisible((v) => !v);
      return;
    }
    setAgentTokenLoading(true);
    setAgentTokenError("");
    try {
      const res = await getAgentToken();
      setAgentToken(res.agent_token || "");
      setAgentTokenVisible(true);
    } catch (err) {
      setAgentTokenError(err.message || "Failed to load agent token");
    } finally {
      setAgentTokenLoading(false);
    }
  }

  async function handleRegenerateAgentToken() {
    setAgentTokenLoading(true);
    setAgentTokenError("");
    try {
      const res = await regenerateAgentToken();
      setAgentToken(res.agent_token || "");
      setAgentTokenVisible(true);
      showToast("Agent token regenerated. Any CLI still using the previous token will need updating.", { type: "success" });
    } catch (err) {
      setAgentTokenError(err.message || "Failed to regenerate agent token");
    } finally {
      setAgentTokenLoading(false);
    }
  }

  async function handleCopyAgentToken() {
    try {
      await navigator.clipboard.writeText(agentToken);
      setAgentTokenCopied(true);
      setTimeout(() => setAgentTokenCopied(false), 1500);
    } catch {
      // clipboard API unavailable — token is already visible on screen to copy manually
    }
  }

  if (!user) return null;
  if (loadingAccount && !account) return <div class="p-6"><p class="text-sm text-slate-500">Loading account…</p></div>;
  if (accountError && !account) return <div class="p-6"><p class="text-sm text-red-600">{accountError}</p></div>;

  const a = account || {};
  const isAdmin = !!a.is_admin;
  const maxJobs = a.max_jobs ?? 0; // 0 = unlimited
  const maxProjects = a.max_projects ?? 0;

  return (
    <div class="max-w-6xl mx-auto px-4 py-8 space-y-6">
      <div>
        <h1 class="text-xl font-semibold text-slate-900">Account</h1>
        <p class="text-sm text-slate-500">Usage limits, exports, and summary email settings for your RunGuard workspace.</p>
      </div>
      {accountError && <div class="text-xs text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">{accountError}</div>}
      <VerifyEmailBanner account={account} />

      <div class="grid gap-4 md:grid-cols-2">
        <section class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
          <h2 class="text-sm font-semibold text-slate-900 mb-2">Profile</h2>
          <div class="text-sm text-slate-700 space-y-1">
            <div><span class="text-slate-500">Email: </span><span>{user.email}</span></div>
            {user.created_at && <div><span class="text-slate-500">Member since: </span><span>{new Date(user.created_at).toLocaleDateString()}</span></div>}
            {isAdmin && <div class="mt-2"><span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] bg-slate-900 text-slate-50">Admin workspace</span></div>}
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-2">
            {onLogout && <button onClick={onLogout} class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100">Logout</button>}
            <button
              type="button"
              onClick={handleLogoutAll}
              disabled={loggingOutAll}
              class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100 disabled:opacity-50"
              title="Invalidates every other signed-in session immediately, without logging you out here"
            >
              {loggingOutAll ? "Logging out other sessions…" : "Log out of all other devices"}
            </button>
          </div>
          {logoutAllMessage && <p class="mt-2 text-xs text-emerald-700">{logoutAllMessage}</p>}
        </section>

        <section class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm space-y-3">
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Usage</p>
            <p class="mt-1 text-sm font-semibold text-slate-900">
              {maxJobs > 0 ? `Up to ${maxJobs} jobs` : "Unlimited jobs"} · {maxProjects > 0 ? `up to ${maxProjects} projects` : "unlimited projects"}
            </p>
            <p class="mt-1 text-xs text-slate-500">RunGuard is free — this is just a per-instance cap, not a paid tier.</p>
          </div>
        </section>
      </div>

      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm space-y-3">
        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Daily summary email</p>
            <p class="mt-1 text-sm text-slate-700">One email per day listing late jobs, recent incidents, and anything that never reported.</p>
            <p class="mt-1 text-xs text-slate-500">Sent around {typeof a.summary_hour_utc === "number" ? `${a.summary_hour_utc}:00 UTC` : "08:00 UTC"} to {a.email}.</p>
          </div>
          <button type="button" onClick={handleToggleSummary} disabled={savingSummary} class={(a.summary_email_enabled ? "bg-emerald-50 border-emerald-200 text-emerald-700" : "bg-slate-50 border-slate-200 text-slate-600") + " inline-flex items-center px-3 py-1.5 rounded-full text-[11px] border"}>{savingSummary ? "Saving…" : a.summary_email_enabled ? "Summary email on" : "Summary email off"}</button>
        </div>
      </div>

      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm space-y-3">
        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Weekly summary email</p>
            <p class="mt-1 text-sm text-slate-700">One email per week: uptime per job, the noisiest jobs, alert counts, and jobs created/deleted.</p>
            <p class="mt-1 text-xs text-slate-500">Sent Mondays around {typeof a.summary_hour_utc === "number" ? `${a.summary_hour_utc}:00 UTC` : "08:00 UTC"} to {a.email}.</p>
          </div>
          <button type="button" onClick={handleToggleWeeklySummary} disabled={savingWeeklySummary} class={(a.weekly_summary_enabled ? "bg-emerald-50 border-emerald-200 text-emerald-700" : "bg-slate-50 border-slate-200 text-slate-600") + " inline-flex items-center px-3 py-1.5 rounded-full text-[11px] border"}>{savingWeeklySummary ? "Saving…" : a.weekly_summary_enabled ? "Weekly summary on" : "Weekly summary off"}</button>
        </div>
      </div>

      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm space-y-3">
        <div>
          <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Agent token</p>
          <p class="mt-1 text-sm text-slate-700">
            Used by <code>runguard register</code> to create jobs from the CLI without copying a heartbeat key by hand first.
            Keep it secret — anyone with this token can create jobs on your account.
          </p>
        </div>

        {agentTokenError && <p class="text-xs text-red-600">{agentTokenError}</p>}

        {agentTokenVisible && agentToken ? (
          <div class="flex flex-wrap items-center gap-2">
            <code class="px-3 py-1.5 rounded-lg bg-slate-50 border border-slate-200 text-[11px] break-all">{agentToken}</code>
            <button type="button" onClick={handleCopyAgentToken} class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100">
              {agentTokenCopied ? "Copied!" : "Copy"}
            </button>
          </div>
        ) : null}

        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={handleShowAgentToken}
            disabled={agentTokenLoading}
            class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100 disabled:opacity-50"
          >
            {agentTokenLoading ? "Loading…" : agentTokenVisible ? "Hide token" : agentToken ? "Show token" : "Reveal token"}
          </button>
          <button
            type="button"
            onClick={handleRegenerateAgentToken}
            disabled={agentTokenLoading}
            title="Immediately invalidates the current token — any CLI still using it will need the new one."
            class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100 disabled:opacity-50"
          >
            Regenerate
          </button>
        </div>
        <p class="text-[11px] text-slate-500">
          Regenerating invalidates the current token immediately — any CLI still using it will start getting 401s from <code>register</code> until updated.
        </p>

        <p class="text-[11px] text-slate-500">
          Example: <code>runguard register --token=YOUR_AGENT_TOKEN --name="orders sync" --interval=60</code>
        </p>
      </div>

      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm space-y-3">
        <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Exports</p>
        <p class="text-sm text-slate-700">Download a snapshot of your jobs and their current status as CSV, Excel or PDF.</p>
        <div class="flex flex-wrap gap-2">
          <button type="button" onClick={() => handleExport("csv")} class="px-3 py-1.5 rounded-full text-[11px] bg-slate-900 text-white hover:bg-slate-800">Download CSV</button>
          <button type="button" onClick={() => handleExport("xlsx")} class="px-3 py-1.5 rounded-full text-[11px] bg-slate-100 text-slate-900 border border-slate-200 hover:bg-slate-200">Download Excel</button>
          <button type="button" onClick={() => handleExport("pdf")} class="px-3 py-1.5 rounded-full text-[11px] bg-slate-100 text-slate-900 border border-slate-200 hover:bg-slate-200">Download PDF</button>
        </div>
      </div>
    </div>
  );
}
