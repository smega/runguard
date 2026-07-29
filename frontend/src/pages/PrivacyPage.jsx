export default function PrivacyPage() {
  return (
    <div class="max-w-4xl mx-auto px-4 py-10 space-y-6 text-sm text-slate-700 leading-relaxed">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Privacy Policy</h1>
        <p class="mt-2 text-xs text-slate-500">Last updated: 29 July 2026</p>
      </div>

      <p class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
        RunGuard is currently in beta. The operating entity (data controller) name below is a
        placeholder pending company formation and will be updated before general availability. The
        data practices described here reflect what the service actually does today.
      </p>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">1. Who controls your data</h2>
        <p>
          <strong>[Placeholder Legal Entity Name]</strong> is the data controller for personal data
          processed through RunGuard, for the purposes of UK GDPR and the Data Protection Act 2018.
          We're a small, self-funded team — this policy describes exactly what we collect and why,
          not a boilerplate list.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">2. What we collect</h2>
        <p><strong>Account data:</strong> your email address, a hashed password (we never store passwords in plain text), and admin/plan status.</p>
        <p><strong>Job configuration:</strong> job names, schedules (interval or cron expression), severity, grace periods, project groupings, and any webhook/email alert settings you configure.</p>
        <p><strong>Run and heartbeat data:</strong> timestamps, status (ok/fail/late), exit codes, and — only if you choose to send them — short tails of your job's stdout/stderr output (capped by your plan and by the CLI's own <code>--truncate</code> flag).</p>
        <p><strong>Delivery logs:</strong> records of alerts we've sent (email, webhook, PagerDuty, Teams) and whether delivery succeeded, kept so you can audit what was sent and when.</p>
        <p><strong>Billing data:</strong> we don't store card details ourselves — Stripe handles payment processing and stores card data on our behalf (see sub-processors below). We keep a record of your plan, subscription status, and invoice history as returned by Stripe.</p>
        <p><strong>Technical data:</strong> IP address and User-Agent are visible to us as part of ordinary HTTP requests (including CLI heartbeats), and may be used for rate limiting, abuse prevention, and debugging. We don't run analytics trackers or advertising pixels on the app or landing page.</p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">3. What we don't collect</h2>
        <p>
          The RunGuard CLI agent does not send your hostname, environment variables (other than an
          optional label you explicitly pass via <code>--env</code>), file paths, full command lines,
          or OS/user information. See the{" "}
          <a href="https://github.com/smega/runguard-cli" target="_blank" rel="noopener noreferrer" class="text-indigo-600 hover:underline">
            open-source CLI repository
          </a>{" "}
          for the exact, auditable code that decides what gets sent on every heartbeat — we'd rather
          you verify this than take our word for it.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">4. Why we process it (legal basis)</h2>
        <p>Account, job, and billing data: processed under contract — it's necessary to provide the service you've signed up for.</p>
        <p>Technical/rate-limiting data: processed under legitimate interest, to keep the service secure and available.</p>
        <p>Marketing emails: we don't send any beyond service-related notices (alerts, billing, security notices) unless you explicitly opt in to something else in the future — none of that exists today.</p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">5. Sub-processors</h2>
        <p>We use a small number of third-party services to run RunGuard. We choose providers with their own GDPR commitments and don't sell or share your data beyond what's needed to operate the service:</p>
        <ul class="list-disc pl-5 space-y-1">
          <li><strong>MongoDB Atlas</strong> — primary database, hosting job/run/account data.</li>
          <li><strong>Stripe</strong> — payment processing and card storage for paid plans.</li>
          <li><strong>Mailgun</strong> — outbound transactional email (alerts, weekly summaries, account notices).</li>
          <li>Our infrastructure provider, for hosting the application server itself.</li>
        </ul>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">6. Data retention</h2>
        <p>
          Account and job configuration data is kept for as long as your account is active. Run
          history, heartbeat logs, and delivery logs are retained according to your plan's history
          window; older records may be aggregated or pruned automatically. If you delete a job or
          close your account, we remove the associated data within a reasonable operational window,
          except where we're required to retain billing records for tax/accounting purposes (typically
          up to 6 years under UK law).
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">7. Your rights</h2>
        <p>Under UK GDPR, you have the right to:</p>
        <ul class="list-disc pl-5 space-y-1">
          <li>Access the personal data we hold about you.</li>
          <li>Correct inaccurate data (most account/job fields are editable directly in the app).</li>
          <li>Request deletion of your account and associated data.</li>
          <li>Export your data (per-job history export is built into the product; a full account export can be requested).</li>
          <li>Object to or restrict certain processing.</li>
          <li>Lodge a complaint with the UK Information Commissioner's Office (ICO) if you believe we've mishandled your data.</li>
        </ul>
        <p>To exercise any of these, contact us using the details below — we aim to respond within 30 days.</p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">8. International transfers</h2>
        <p>
          Some of our sub-processors (Stripe, MongoDB Atlas, Mailgun) may process data outside the UK.
          Where that happens, it's done under a recognised safeguard — such as the provider's UK/EU
          data residency options, Standard Contractual Clauses, or an adequacy decision — as offered by
          that provider.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">9. Security</h2>
        <p>
          See our <a href="/security" class="text-indigo-600 hover:underline">Security page</a> for
          details on how we protect stored data, including rate limiting, SSRF-safe outbound webhook
          delivery, and password hashing.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">10. Changes to this policy</h2>
        <p>
          We may update this policy as the service evolves. Material changes will be announced by
          email or an in-app notice before they take effect.
        </p>
      </section>

      <section class="space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">11. Contact</h2>
        <p>
          Privacy questions or rights requests: <a href="mailto:privacy@runguard.smega.eu" class="text-indigo-600 hover:underline">privacy@runguard.smega.eu</a>{" "}
          (placeholder contact — will be updated alongside the legal entity name above).
        </p>
      </section>
    </div>
  );
}
