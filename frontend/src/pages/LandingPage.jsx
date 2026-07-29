// src/pages/LandingPage.jsx
import { h } from "preact";
import { Link } from "preact-router/match";

export default function LandingPage({ user }) {
  return (
    <div class="bg-slate-50">
      <div class="max-w-5xl mx-auto px-4 py-12">
        <section class="grid gap-10 md:grid-cols-[1.4fr,1fr] items-start">
          <div>
            <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-50 border border-blue-100 text-[11px] font-medium text-blue-800 mb-4">
              <span class="w-1.5 h-1.5 rounded-full bg-teal-500" />
              RunGuard · cron &amp; heartbeat monitoring, open-source CLI
            </div>

            <h1 class="text-3xl md:text-4xl font-semibold tracking-tight text-slate-900 mb-3">
              Know the moment a job
              <span class="text-blue-700"> silently stops running.</span>
            </h1>

            <p class="text-sm text-slate-600 mb-5 max-w-xl">
              RunGuard watches your cron jobs, background workers, and scheduled scripts. When one
              goes quiet, fails, or runs late, you hear about it — by email, webhook, Slack,
              Discord, PagerDuty, or Teams — instead of finding out from a customer.
            </p>

            <div class="flex flex-wrap items-center gap-3 mb-4">
              {!user && (
                <>
                  <Link
                    href="/signup"
                    class="px-4 py-2 rounded-full bg-blue-600 text-sm font-medium text-white shadow-sm hover:bg-blue-500"
                  >
                    Get started
                  </Link>
                  <Link
                    href="/pricing"
                    class="text-sm text-slate-600 hover:text-slate-900"
                  >
                    Pricing
                  </Link>
                </>
              )}
              {user && (
                <Link
                  href="/account"
                  class="px-4 py-2 rounded-full border border-slate-300 text-sm text-slate-700 hover:bg-slate-100"
                >
                  Go to your profile
                </Link>
              )}
            </div>

            <div class="flex flex-wrap gap-4 text-[11px] text-slate-500">
              <div class="flex items-center gap-2">
                <span class="w-1.5 h-1.5 rounded-full bg-teal-500" />
                <span>Simple heartbeat URL or the open-source CLI agent. No inbound access or VPN required.</span>
              </div>
              <div class="flex items-center gap-2">
                <span class="w-1.5 h-1.5 rounded-full bg-blue-500" />
                <span>Cron expressions, not just fixed intervals — RunGuard knows when your job is actually due.</span>
              </div>
            </div>
          </div>

          <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm text-xs text-slate-700">
            <div class="flex items-center justify-between mb-2">
              <div class="font-medium text-slate-800">Example heartbeat</div>
              <span class="text-[10px] px-2 py-0.5 rounded-full bg-slate-50 border border-slate-200 text-slate-500">
                cron · systemd · CI
              </span>
            </div>
            <pre class="bg-slate-50 border border-slate-200 rounded-xl p-3 overflow-x-auto text-[11px] leading-snug font-mono">
{`# plain curl, or the open-source CLI:
runguard wrap --key=ORDERS_SYNC --retry=2 \\
  -- /opt/scripts/sync_orders.sh`}
            </pre>
            <p class="mt-3 text-[11px] text-slate-500">
              <code>wrap</code> runs your command, captures the exit code and a
              tail of stdout/stderr, retries on failure if you ask it to, then
              reports back. If the job never checks in at all, RunGuard
              notices that too.
            </p>
          </div>
        </section>

        <section class="mt-14">
          <h2 class="text-xl font-semibold text-slate-900 mb-1">Built for jobs that actually run on a schedule</h2>
          <p class="text-sm text-slate-600 mb-6 max-w-2xl">
            Not every job runs every 5 minutes. RunGuard is built around how cron jobs really
            behave — cron expressions, grace periods, and severity that reflects how much a
            missed run actually matters.
          </p>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <FeatureCard
              title="Cron-aware scheduling"
              body="Give RunGuard the same cron expression your crontab uses. It knows exactly when a job is due next, so grace windows are based on your real schedule, not a rough interval guess."
            />
            <FeatureCard
              title="Severity that matches the stakes"
              body="Mark jobs low, normal, high, or critical. A stale low-priority report and a stale billing sync shouldn't page you the same way."
            />
            <FeatureCard
              title="Silence maintenance windows, not alerts"
              body="Freeze a job until it next succeeds, or mute it manually for planned maintenance — without turning off monitoring for everything else."
            />
            <FeatureCard
              title="Alerts where your team already is"
              body="Email, generic webhooks, Slack, Discord, PagerDuty, or Microsoft Teams. Route incidents to the channel your team actually watches."
            />
            <FeatureCard
              title="Weekly summaries, not just fire alarms"
              body="A digest of what ran, what failed, and current uptime — so you're not relying on memory to know how reliable a job has been."
            />
            <FeatureCard
              title="Exportable history"
              body="Pull per-job run history as CSV or XLSX when you need it for a postmortem, an audit, or just your own records."
            />
          </div>
        </section>

        <section class="mt-14 bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <div class="grid gap-6 md:grid-cols-[1.2fr,1fr] items-center">
            <div>
              <h2 class="text-xl font-semibold text-slate-900 mb-2">The client is open source. Read it before you trust it.</h2>
              <p class="text-sm text-slate-600 mb-3">
                The RunGuard CLI is a small, dependency-free Go binary — no hidden telemetry, no
                bundled analytics. It's public so you can read exactly what it sends before you
                point it at anything you care about, rather than taking a vendor's word for it.
              </p>
              <a
                href="https://github.com/smega/runguard-cli"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1.5 text-sm font-medium text-blue-700 hover:text-blue-900"
              >
                View the CLI source on GitHub &rarr;
              </a>
            </div>
            <pre class="bg-slate-50 border border-slate-200 rounded-xl p-3 overflow-x-auto text-[11px] leading-snug font-mono text-slate-700">
{`$ go build -o runguard .
$ runguard wrap --key=KEY -- your-job.sh
$ runguard self-update`}
            </pre>
          </div>
        </section>

        <section class="mt-14 flex flex-wrap items-center justify-between gap-4 bg-slate-900 text-white rounded-2xl p-6">
          <div>
            <h2 class="text-lg font-semibold mb-1">Start with your most important job.</h2>
            <p class="text-sm text-slate-300 max-w-md">Free plan included — add one job in a couple of minutes and see your first heartbeat land.</p>
          </div>
          <div class="flex items-center gap-3">
            {!user && (
              <Link href="/signup" class="px-4 py-2 rounded-full bg-white text-sm font-medium text-slate-900 hover:bg-slate-100">
                Get started
              </Link>
            )}
            <Link href="/pricing" class="text-sm text-slate-300 hover:text-white">
              See pricing
            </Link>
          </div>
        </section>
      </div>
    </div>
  );
}

function FeatureCard({ title, body }) {
  return (
    <div class="bg-white border border-slate-200 rounded-xl p-4 shadow-sm">
      <h3 class="text-sm font-semibold text-slate-900 mb-1">{title}</h3>
      <p class="text-xs text-slate-600 leading-relaxed">{body}</p>
    </div>
  );
}
