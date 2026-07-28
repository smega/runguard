# RunGuard idea backlog

Merged from the old `cli_next_steps.txt` and `_aaanextones` (both deleted —
this replaces them). Tags:

- `[DONE]` — already built.
- `[BUILDING NOW]` — in progress this round.
- `[WORTH DOING]` — good idea, not urgent, revisit later.
- `[NOT WORTH IT]` — skip given the actual goal (a few thousand pounds/month,
  solo-maintained, self-hosted/homelab niche) — usually because it's
  enterprise/team-plan scope, an ongoing external-integration maintenance
  burden, or complexity disproportionate to the payoff.

## CLI / agent

- `[DONE]` Log capture & replay — stdout/stderr tail captured by `runguard
  wrap`, shown in the job run history UI.
- `[DONE]` Retry on failure — `runguard wrap --retry=N`.
- `[DONE]` Self-registration — `runguard register --name="..."`.
- `[DONE]` Auto-update channel — `runguard self-update`, SHA256-verified.
- `[NOT WORTH IT]` OpenTelemetry/Honeycomb tracing (`span(name="orders_sync")`)
  — heavy infra commitment for a heartbeat monitor at this scale; the kind of
  thing to reconsider only if paying users specifically ask for it.
- `[NOT WORTH IT]` Dedicated `runguard jobs list` / `runguard jobs test`
  subcommands — low priority against the core wrap/register loop; revisit
  only alongside multi-user orgs (which is itself not worth it yet).

## Scheduling & silence

- `[DONE]` Schedule-aware silence (maintenance windows) — per-job
  `maintenance_until`, respected by the alert loop.
- `[DONE]` Cron-expression scheduling — jobs can be defined by a 5-field cron
  expression instead of a plain interval; lateness is computed against the
  actual next scheduled run.
- `[BUILDING NOW]` Freeze until first success / freeze until manually
  unmuted — two new silence modes alongside the existing fixed-duration
  window, useful for recovery storms and noisy deploys.
- `[WORTH DOING]` Scheduled/recurring silence profiles ("mute every day
  00:00-01:00", "Mondays 06:00-08:00 for deploys") — real improvement over
  one-off silence windows, needs its own small scheduler. Good next-round
  candidate.
- `[NOT WORTH IT]` Follow-the-parent silence (auto-silence downstream jobs
  when an upstream job is in maintenance) — requires a job-dependency graph;
  clever but real scope for a narrow benefit.
- `[NOT WORTH IT]` Deployment-aware auto-silence (GitHub/GitLab/Argo/Tekton/
  Jenkins/Drone/CircleCI integrations that auto-mute during a deploy) — a
  maintenance burden across half a dozen external platforms.

## Alerts & webhooks

- `[DONE]` Multi-URL webhooks per job.
- `[DONE]` Webhook delivery retries with backoff (3 attempts).
- `[DONE]` Slack template type (`{"text": ...}`).
- `[DONE]` Discord template type (`{"content": ...}`, 2000-char trim).
- `[DONE]` PagerDuty (Events API v2, routing key via a URL query param,
  severity mapped, dedup_key per job — no resolve event yet) and Microsoft
  Teams (classic MessageCard) template types.
- `[WORTH DOING]` Custom webhook payload templates (Go `text/template`) —
  lets power users hit anything (OpsGenie, custom internal tools, etc.)
  without RunGuard building a dedicated formatter for each one. Worth doing
  once there are a couple more fixed formats already in place, as the
  "escape hatch" rather than the first thing built.
- `[NOT WORTH IT]` Webhook DLQ + redrive — enterprise reliability-engineering
  scope; the existing retry + visible delivery log is proportionate here.
- `[NOT WORTH IT]` SMS / phone-call alerting (Twilio, Nexmo) — real cost and
  compliance surface (carrier registration, opt-outs) for a feature that's
  easy to defer until specifically requested.
- `[NOT WORTH IT]` Full alert-routing rules engine (route by severity / time
  of day / retry count / environment to different channels) — real product
  scope; the common cases are already approximable today by pointing
  different jobs at different webhook templates.

## Jobs, severity & history

- `[DONE]` Per-job severity (low/normal/high/critical), used in the daily
  summary and shown on the public link.
- `[DONE]` Per-job history export (CSV/XLSX) — reuses the existing
  export machinery, scoped to one job instead of the whole account.
- `[DONE]` Jobs list/overview UX niceties — badge for late/critical
  jobs, filter by severity + "silenced only", remember the last-viewed job.

## Summaries & reporting

- `[DONE]` Daily summary email.
- `[DONE]` Weekly summary email — uptime per job, noisiest jobs,
  alert counts by severity, jobs created/deleted. Reuses the daily-summary
  infrastructure. Good retention/"stickiness" feature and a shareable
  screenshot for anyone showing RunGuard to a colleague.

## Billing & plan enforcement

- `[DONE]` Stripe billing, trial expiry.
- `[WORTH DOING]` Plan-based throttling — rate-limit heartbeats/exports per
  plan tier, block ingestion once genuinely over quota. Matters once you're
  not the only user; do this before any real marketing push, not before.

## Multi-user / teams / infra integrations (the "second wave")

Everything in this section shares one reason for `[NOT WORTH IT]`: it's
scope aimed at competing on team-plan / enterprise features, most of it
requires ongoing maintenance of external integration surfaces, and none of
it is validated by actual demand yet. Revisit only if paying users ask.

- `[NOT WORTH IT]` Multi-user teams/orgs (roles, SSO, per-member routing,
  audit log) — a real rework of the auth model.
- `[NOT WORTH IT]` Terraform provider — nice infra-as-code story, but a
  separately versioned artifact to maintain forever.
- `[NOT WORTH IT]` GitHub Action / GitLab CI template / Argo Workflow hook —
  "curl the install script, run `runguard wrap`" already works in all of
  these without RunGuard publishing anything extra (same reasoning as the
  Kubernetes docs-not-operator decision — see `docs/KUBERNETES.md`).
- `[NOT WORTH IT]` PagerDuty/OpsGenie as first-class dedicated integrations
  beyond a webhook template type — the webhook template covers the
  reasonable version of this already.

## Already resolved / superseded

- `[DONE]` "Show logs in UI" — this was the original ask; resolved via the
  CLI payload fix + `JobsPage.jsx`'s existing log-tail display.
