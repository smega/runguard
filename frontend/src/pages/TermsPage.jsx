export default function TermsPage() {
  return (
    <div class="max-w-4xl mx-auto px-4 py-10 space-y-6 text-sm text-slate-700 leading-relaxed">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Terms of Service</h1>
        <p class="mt-2 text-xs text-slate-500">Last updated: 29 July 2026</p>
      </div>

      <p class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
        RunGuard is currently in beta. The operating entity name below is a placeholder pending
        company formation and will be updated with our registered legal name and address before
        general availability. These terms otherwise apply as written from the date above.
      </p>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">1. Who we are</h2>
        <p>
          RunGuard ("we", "us", "our") is provided by <strong>[Placeholder Legal Entity Name]</strong>,
          operating under the laws of England and Wales. These terms are governed by the laws of the
          United Kingdom, and any dispute not resolved informally is subject to the exclusive
          jurisdiction of the courts of England and Wales.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">2. The service</h2>
        <p>
          RunGuard is a cron/heartbeat job monitoring service. You configure jobs (scheduled tasks,
          cron entries, background workers) to send a heartbeat to RunGuard on each run, using either
          the RunGuard CLI agent or a plain HTTP request. We track whether heartbeats arrive on
          schedule and notify you by email, webhook, PagerDuty, or Microsoft Teams when a job is late,
          fails, or recovers.
        </p>
        <p>
          RunGuard is a monitoring and alerting tool. It does not run, schedule, retry, or take
          corrective action on your jobs — it only observes and reports. You remain solely
          responsible for the jobs themselves and for having appropriate fallback processes if
          RunGuard is unavailable or an alert doesn't reach you.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">3. Accounts</h2>
        <p>
          You must provide accurate information when creating an account and keep your login
          credentials confidential. You're responsible for all activity under your account,
          including actions taken through API keys, agent tokens, or heartbeat keys you generate.
          If you believe a credential has been compromised, rotate it immediately from your account
          settings — this doesn't require contacting us first.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">4. Plans, billing, and refunds</h2>
        <p>
          Paid plans are billed monthly in advance via Stripe, in GBP, at the prices shown on our{" "}
          <a href="/pricing" class="text-indigo-600 hover:underline">pricing page</a> at the time of
          purchase. We may change prices for future billing periods with at least 30 days' notice by
          email; continuing to use a paid plan after a price change takes effect constitutes
          acceptance of the new price.
        </p>
        <p>
          You can cancel a paid plan at any time from your account page; cancellation takes effect at
          the end of the current billing period and you retain access until then. We don't offer
          prorated refunds for partial billing periods, but if you believe you were charged in error,
          contact us and we'll review it in good faith. Where we offer a free trial, no payment is
          taken until the trial ends and you've chosen to continue on a paid plan.
        </p>
        <p>
          If a job count or feature limit tied to your plan is exceeded (for example, you've reached
          your plan's monitored-job limit), we may prevent new jobs from being created until you
          upgrade or remove existing ones; we won't delete your data solely for being over a limit.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">5. Acceptable use</h2>
        <p>You agree not to use RunGuard to:</p>
        <ul class="list-disc pl-5 space-y-1">
          <li>Monitor or interact with systems you don't have the right to monitor.</li>
          <li>Configure webhook or callback URLs that target internal/private network addresses, or otherwise attempt to use RunGuard's outbound requests as a proxy into infrastructure that isn't yours.</li>
          <li>Send unlawful, infringing, or abusive content through job names, tags, stdout/stderr captures, or any other field.</li>
          <li>Attempt to circumvent plan limits, rate limits, or authentication controls, or interfere with the service's availability for other users.</li>
          <li>Reverse engineer the hosted service (as distinct from the open-source CLI, which you're welcome to inspect and modify under its own license).</li>
        </ul>
        <p>We may suspend or terminate accounts that violate this section, with notice where practicable.</p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">6. Your data and content</h2>
        <p>
          You retain ownership of the job configuration, run history, and any log output you send to
          RunGuard. You grant us a limited license to store and process that data solely to provide
          the service to you. See our{" "}
          <a href="/privacy" class="text-indigo-600 hover:underline">Privacy Policy</a> for details on
          what we store and for how long.
        </p>
        <p>
          Avoid sending secrets (passwords, API keys, tokens) in job names or in stdout/stderr your
          jobs print, since these may be captured as part of a heartbeat. This is your responsibility
          to control, not something RunGuard filters on your behalf.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">7. Service availability</h2>
        <p>
          We aim for high availability but do not currently offer a formal SLA or service credits.
          RunGuard is built and operated by a small team, and while we take reliability seriously
          (see our{" "}
          <a href="/security" class="text-indigo-600 hover:underline">Security</a> page), we can't
          guarantee uninterrupted service. If RunGuard itself is down or delayed in sending an alert,
          we won't be liable for any resulting downstream impact from your monitored jobs failing
          silently during that window — this is why we recommend RunGuard as one layer of monitoring,
          not your only safeguard for critical systems.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">8. Liability</h2>
        <p>
          To the fullest extent permitted by law, RunGuard is provided "as is" without warranties of
          any kind. Our total liability to you for any claim arising from your use of the service is
          limited to the amount you paid us in the 12 months before the claim arose. Nothing in these
          terms excludes or limits liability that cannot lawfully be excluded or limited under UK law,
          including liability for death or personal injury caused by negligence, or for fraud.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">9. Termination</h2>
        <p>
          You may close your account at any time from your account settings. We may suspend or
          terminate your account for breach of these terms, non-payment, or if required by law, and
          will make reasonable efforts to notify you first except where immediate action is
          necessary. Sections that by their nature should survive termination (billing already owed,
          liability limits) continue to apply.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">10. Changes to these terms</h2>
        <p>
          We may update these terms as the service evolves. For material changes, we'll notify you by
          email or an in-app notice before they take effect. Continued use of RunGuard after changes
          take effect constitutes acceptance.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">11. Contact</h2>
        <p>
          Questions about these terms: <a href="mailto:legal@runguard.smega.eu" class="text-indigo-600 hover:underline">legal@runguard.smega.eu</a>{" "}
          (placeholder contact — will be updated alongside the legal entity name above).
        </p>
      </section>
    </div>
  );
}
