import { route } from "preact-router";
import { useEffect, useState } from "preact/hooks";
import { createCheckoutLink, createPortalLink, getAccount, getAgentToken, getBilling, logoutAllDevices, regenerateAgentToken, updateAccount } from "../api.js";
import { showToast } from "../components/Toast.jsx";

export default function AccountPage({ user, onLogout }) {
  const [account, setAccount] = useState(null);
  const [billing, setBilling] = useState(null);
  const [loadingAccount, setLoadingAccount] = useState(true);
  const [accountError, setAccountError] = useState("");
  const [savingSummary, setSavingSummary] = useState(false);
  const [billingBusy, setBillingBusy] = useState("");
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
        const [accountData, billingData] = await Promise.all([getAccount(), getBilling()]);
        setAccount(accountData);
        setBilling(billingData);
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

  async function goCheckout(planId) {
    setBillingBusy(planId);
    try {
      const res = await createCheckoutLink(planId);
      window.location.href = res.url;
    } catch (err) {
      setAccountError(err.message || "Failed to open checkout");
    } finally {
      setBillingBusy("");
    }
  }

  async function goPortal() {
    setBillingBusy("portal");
    try {
      const res = await createPortalLink();
      window.location.href = res.url;
    } catch (err) {
      setAccountError(err.message || "Billing portal is not configured yet");
    } finally {
      setBillingBusy("");
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
  const plan = a.plan || {};
  const isAdmin = !!a.is_admin;
  const billingPlans = billing?.billing || [];
  const currentPlanId = billing?.current_plan_id || a.plan_id;

  return (
    <div class="max-w-6xl mx-auto px-4 py-8 space-y-6">
      <div>
        <h1 class="text-xl font-semibold text-slate-900">Account & billing</h1>
        <p class="text-sm text-slate-500">Plan details, billing routes, exports, and summary email settings for your RunGuard workspace.</p>
      </div>
      {accountError && <div class="text-xs text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">{accountError}</div>}

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
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Current plan</p>
              <p class="mt-1 text-sm font-semibold text-slate-900">{plan.label || "Free"}</p>
              <p class="mt-1 text-xs text-slate-500">Up to {plan.max_jobs ?? 1} jobs and {plan.max_projects ?? 1} projects.</p>
            </div>
            <div class="text-right text-[11px] text-slate-500 max-w-[12rem]">
              <div>Email alerts: <span class="font-medium">{plan.allow_email_alerts ? "Included" : "Not included"}</span></div>
              <div>Public links: <span class="font-medium">{plan.allow_public_links ? "Included" : "Not included"}</span></div>
              <div>Exports: <span class="font-medium">{plan.allow_exports ? "Included" : "Not included"}</span></div>
            </div>
          </div>
          {Array.isArray(plan.bullets) && plan.bullets.length > 0 && <ul class="space-y-1 text-[11px] text-slate-600">{plan.bullets.map((b) => <li key={b} class="flex items-start gap-1.5"><span class="mt-[3px] h-1.5 w-1.5 rounded-full bg-slate-300" /><span>{b}</span></li>)}</ul>}
          <div class="flex flex-wrap gap-2 pt-2">
            {billingPlans.filter((bp) => bp.plan_id !== currentPlanId && bp.price_monthly_gbp > 0).map((bp) => (
              <button key={bp.plan_id} onClick={() => goCheckout(bp.plan_id)} disabled={billingBusy === bp.plan_id} class="px-3 py-1.5 rounded-full bg-slate-900 text-white text-xs hover:bg-slate-800 disabled:opacity-50">{billingBusy === bp.plan_id ? "Opening…" : `Upgrade to ${bp.plan_id}`}</button>
            ))}
            <button onClick={goPortal} disabled={billingBusy === "portal"} class="px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 hover:bg-slate-100 disabled:opacity-50">{billingBusy === "portal" ? "Opening…" : "Open billing portal"}</button>
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

      {(plan.allow_exports || isAdmin) && (
        <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm space-y-3">
          <p class="text-xs font-medium text-slate-500 uppercase tracking-wide">Exports</p>
          <p class="text-sm text-slate-700">Download a snapshot of your jobs and their current status as CSV, Excel or PDF.</p>
          <div class="flex flex-wrap gap-2">
            <button type="button" onClick={() => handleExport("csv")} class="px-3 py-1.5 rounded-full text-[11px] bg-slate-900 text-white hover:bg-slate-800">Download CSV</button>
            <button type="button" onClick={() => handleExport("xlsx")} class="px-3 py-1.5 rounded-full text-[11px] bg-slate-100 text-slate-900 border border-slate-200 hover:bg-slate-200">Download Excel</button>
            <button type="button" onClick={() => handleExport("pdf")} class="px-3 py-1.5 rounded-full text-[11px] bg-slate-100 text-slate-900 border border-slate-200 hover:bg-slate-200">Download PDF</button>
          </div>
        </div>
      )}
    </div>
  );
}
