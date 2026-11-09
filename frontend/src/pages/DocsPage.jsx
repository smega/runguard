import { PUBLIC_BASE_URL } from "../siteConfig";

export default function DocsPage() {
  return (
    <div class="max-w-4xl mx-auto px-4 py-10 space-y-8 text-sm text-slate-700 leading-relaxed">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Getting started</h1>
        <p class="mt-2 text-slate-600">
          Create a project, add one job, send a heartbeat, then test both a failure and a recovery
          alert before rolling RunGuard out more widely.
        </p>
      </div>

      <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <p>1. Create a project for a service or environment.</p>
        <p>2. Add a job with a realistic schedule (a fixed interval, or a cron expression) and grace window.</p>
        <p>3. Send a test heartbeat — with a plain <code>curl</code> or the CLI — and confirm it appears in the dashboard.</p>
        <p>4. Add an email and/or webhook alert destination (Slack, Discord, PagerDuty, or Teams).</p>
        <p>5. Simulate a failure and then a healthy recovery to see both alert paths.</p>
      </div>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">Sending heartbeats</h2>
        <p>
          Every job gets a heartbeat URL. Hit it after each successful run — from cron, a systemd
          timer, a CI job, or anywhere else a scheduled task runs:
        </p>
        <pre class="bg-slate-50 border border-slate-200 rounded-xl p-3 overflow-x-auto text-[12px] leading-snug font-mono">
{`0 * * * * /opt/scripts/sync_orders.sh && \\
  curl -fsS ${PUBLIC_BASE_URL}/hb/ORDERS_SYNC >/dev/null`}
        </pre>
        <p>
          That's the entire integration if you just want an "it ran" signal. If you also want exit
          codes, stdout/stderr tails, retries, or automatic failure heartbeats, use the CLI instead.
        </p>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">The CLI agent</h2>
        <p>
          <code>runguard</code> is a small, dependency-free, <strong>open-source</strong> Go
          binary — the full source is at{" "}
          <a href="https://github.com/smega/runguard-cli" target="_blank" rel="noopener noreferrer" class="text-indigo-600 hover:underline">
            github.com/smega/runguard-cli
          </a>
          . It wraps a command, captures the result, and reports a heartbeat automatically —
          whether the command succeeds or fails.
        </p>
        <pre class="bg-slate-50 border border-slate-200 rounded-xl p-3 overflow-x-auto text-[12px] leading-snug font-mono">
{`# Wrap any command. Success or failure, a heartbeat is sent either way.
runguard wrap --key=ORDERS_SYNC -- /opt/scripts/sync_orders.sh

# Retry a flaky job up to 2 extra times before reporting failure.
runguard wrap --key=FLAKY_SYNC --retry=2 -- /opt/scripts/flaky_sync.sh

# Register a job from a provisioning script instead of the UI.
runguard register --token=$RUNGUARD_AGENT_TOKEN --name="orders sync" --interval=60

# Update the CLI in place.
runguard self-update`}
        </pre>
        <p>
          For exactly what data each subcommand sends (and, just as importantly, what it doesn't),
          see the CLI repo's README — it's written as a plain accounting, not marketing copy,
          because that's what we'd want to read before pointing a binary at our own infrastructure.
        </p>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">Scheduling: intervals or cron expressions</h2>
        <p>
          Jobs can be monitored on a fixed interval (e.g. "every 60 minutes") or with a real cron
          expression (e.g. <code>0 */6 * * *</code>). Cron-aware scheduling means RunGuard knows
          exactly when a job is next due, so grace periods are measured against your actual
          schedule instead of an approximate interval.
        </p>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">Severity and silencing</h2>
        <p>
          Set a job's severity (low/normal/high/critical) so alerts reflect how much a miss
          actually matters. When you need planned downtime, freeze a job until it next succeeds or
          mute it manually — monitoring for every other job keeps running normally.
        </p>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">Alert destinations</h2>
        <p>
          Add email alerts, or a webhook pointed at Slack, Discord, PagerDuty, or Microsoft Teams —
          RunGuard formats the payload for whichever type you choose. You can also enable a weekly
          summary email covering what ran, what failed, and current uptime per job.
        </p>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">Exporting history</h2>
        <p>
          Per-job run history can be exported as CSV or XLSX from the job detail panel — useful for
          a postmortem, an audit, or just keeping your own records outside RunGuard.
        </p>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold text-slate-900">More</h2>
        <p>
          See the <a href="/security" class="text-indigo-600 hover:underline">Security</a> page for
          how the service and CLI are built, or the{" "}
          <a href="/privacy" class="text-indigo-600 hover:underline">Privacy Policy</a> for exactly
          what account/job data is stored and why.
        </p>
      </section>
    </div>
  );
}
