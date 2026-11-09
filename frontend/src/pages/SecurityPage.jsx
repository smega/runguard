import { SECURITY_EMAIL } from "../siteConfig";

export default function SecurityPage() {
  return (
    <div class="max-w-4xl mx-auto px-4 py-10 space-y-6 text-sm text-slate-700 leading-relaxed">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Security</h1>
        <p class="mt-2 text-xs text-slate-500">Last updated: 29 July 2026</p>
      </div>

      <p class="text-sm text-slate-600">
        RunGuard is built and operated by a small team, not a large security organization — so
        rather than a vague "we take security seriously" statement, here's a specific account of
        what's actually in place today.
      </p>

      <section class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">Authentication</h2>
        <p>Passwords are hashed with bcrypt before storage — we never store or log plain-text passwords.</p>
        <p>
          Sessions use a signed, HTTP-only cookie (secure/HTTPS-only in production). Every session
          carries a token version tied to your account; changing your password or using "log out
          everywhere" immediately invalidates all existing sessions rather than waiting for them to
          expire naturally.
        </p>
        <p>Agent tokens (used by the CLI's <code>register</code> command) are a separate, narrowly-scoped credential — they can create/look up jobs but cannot access the web UI or account settings, and can be regenerated at any time, which instantly revokes the old value.</p>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">Rate limiting &amp; abuse prevention</h2>
        <p>Authentication endpoints and heartbeat endpoints are independently rate-limited — both per source IP and per heartbeat key — so a single leaked key or a brute-force attempt can't be used to flood the service or hide a real signal in noise.</p>
        <p>Reverse proxy configuration is explicit: RunGuard only trusts <code>X-Forwarded-For</code> from proxy addresses we've configured, so client IPs used for rate limiting can't be spoofed by a request header.</p>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">Outbound webhook delivery (SSRF protection)</h2>
        <p>
          When you configure a webhook URL (generic, Slack, Discord, PagerDuty, Teams), RunGuard
          validates the URL scheme and blocks delivery to loopback, private, and link-local
          addresses before making the request. This stops a webhook field from being used to make
          RunGuard's server issue requests into internal networks it shouldn't be able to reach.
        </p>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">The CLI agent</h2>
        <p>
          The agent is designed to be outbound-only: it sends a heartbeat and exits. It does not
          open a listening port, does not accept inbound remote commands, and does not require
          elevated privileges to run. Its <code>self-update</code> command verifies a SHA256
          checksum before replacing the running binary, and refuses to install if no checksum is
          published or it doesn't match.
        </p>
        <p>
          The CLI is fully open source at{" "}
          <a href="https://github.com/smega/runguard-cli" target="_blank" rel="noopener noreferrer" class="text-indigo-600 hover:underline">
            github.com/smega/runguard-cli
          </a>{" "}
          — every line of code that runs on your machine is public and auditable, rather than
          something you have to trust on our word.
        </p>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">Data handling</h2>
        <p>Stdout/stderr tails are only captured if you choose to send them (via the CLI's wrap command), are capped in size by both the CLI and your plan tier, and are stored as-is — nothing is scanned or redacted automatically. Avoid printing secrets in a wrapped command's output, and rotate any webhook URL or key you believe has been exposed; both can be regenerated at any time from your account.</p>
        <p>Exports (CSV/XLSX job history) are limited in row count and size to prevent excessive load, and are generated on demand rather than cached indefinitely.</p>
        <p>Database backups are taken on a regular schedule and stored separately from the primary database.</p>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm space-y-2">
        <h2 class="text-lg font-semibold text-slate-900">Reporting a vulnerability</h2>
        <p>
          If you find a security issue, please email{" "}
          <a href={`mailto:${SECURITY_EMAIL}`} class="text-indigo-600 hover:underline">{SECURITY_EMAIL}</a>{" "}
          with details and, if possible, steps to reproduce. We'll acknowledge reports promptly and
          keep you updated as we investigate and fix. Please give us a reasonable window to address
          an issue before any public disclosure — as a small team, our response time won't match a
          large company's, but we do take reports seriously and will follow up.
        </p>
      </section>
    </div>
  );
}
