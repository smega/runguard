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
              RunGuard · production-grade cron monitoring
            </div>

            <h1 class="text-3xl md:text-4xl font-semibold tracking-tight text-slate-900 mb-3">
              Quiet, dependable monitoring
              <span class="text-blue-700"> for critical jobs.</span>
            </h1>

            <p class="text-sm text-slate-600 mb-5 max-w-xl">
              RunGuard gives you a clear, auditable view of every scheduled run.
              When a job silently fails or never checks in, you know about it
              before your customers do.
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
                <span>Designed for production workloads and on-call teams.</span>
              </div>
              <div class="flex items-center gap-2">
                <span class="w-1.5 h-1.5 rounded-full bg-blue-500" />
                <span>Simple heartbeat URL or wrapper-based agent support. No inbound access or VPN requirements.</span>
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
{`0 * * * * /opt/scripts/sync_orders.sh && \\
  curl -fsS https://app.runguard.app/hb/ORDERS_SYNC >/dev/null`}
            </pre>
            <p class="mt-3 text-[11px] text-slate-500">
              Each successful run reports back via a simple heartbeat. If a run
              fails or never reaches RunGuard, it is flagged and surfaced in
              your dashboards and alerts.
            </p>
          </div>
        </section>
      </div>
    </div>
  );
}
