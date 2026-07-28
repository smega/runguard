# Agent (CLI)

`runguard wrap -- your-command` captures stdout/stderr/exit code/duration and
posts a heartbeat; `runguard register` self-registers a job using an account
agent token instead of a pre-existing heartbeat key (view/regenerate the
token from the account page). See [`docs/AGENT_DATA.md`](docs/AGENT_DATA.md)
for exactly what data the CLI sends and when, and
[`docs/KUBERNETES.md`](docs/KUBERNETES.md) for monitoring Kubernetes
CronJobs specifically.


# Admin access

Add an email to `admin.bootstrap_emails` in `backend/config.yml` and restart
the backend — that account is promoted to admin automatically (idempotent,
safe to leave configured permanently). From there, admins can promote/demote
other users from the in-app Admin page (`/admin`, only visible to admins),
which also keeps an audit log of who granted/revoked admin access and when.
The last remaining admin can't be demoted, so this can't lock you out.

(The old approach of running a `db.users.updateOne(...)` command by hand in
mongosh still works if you ever need it, but shouldn't be necessary anymore.)


## Stripe billing

RunGuard now supports long-term Stripe integration using:
- Checkout Sessions for subscription signup
- Billing Portal sessions for customer self-service
- verified Stripe webhooks for subscription lifecycle updates

Configure these fields in `backend/config.yml`:
- `billing.stripe_secret_key`
- `billing.webhook_secret`
- `billing.success_url`
- `billing.cancel_url`
- `billing.portal_return_url`
- `billing.plans[].stripe_price_id`

Recommended Stripe webhook events:
- `checkout.session.completed`
- `customer.subscription.created`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `invoice.payment_failed`
- `invoice.paid`


## Backups

The real safety net should be Atlas's own continuous backups — turn those on
in the Atlas dashboard for the production cluster. `scripts/backup_mongo.sh`
is a secondary, ad-hoc snapshot tool (good before a risky migration, or as a
cron job for an extra off-Atlas copy):

```
./scripts/backup_mongo.sh                       # reads mongo.uri from backend/config.yml
MONGO_URI="mongodb+srv://..." ./scripts/backup_mongo.sh
```


## CI/CD

`.github/workflows/ci.yml` runs a build/format/vet check on every push and
PR (no automated tests yet — tracked separately). `.github/workflows/deploy.yml`
is a manual (`workflow_dispatch`) deploy that reuses `_build/build_go.sh` and
`_build/build_preact.sh` — the same scripts you'd run locally — so there's
one definition of "deploy" either way. It's disabled until the required repo
secrets are added; see the comment at the top of that workflow file for
exactly which ones and what they should contain.
