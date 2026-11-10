# Getting started (self-hosting)

Prerequisites: Go 1.26+, Node 18+ (to build the frontend), and a MongoDB
instance (Atlas or self-hosted). A Mailgun account is optional (needed only
for email alerts/summaries).

**Backend**

```sh
cd backend
cp config.sample.yml config.yml
# edit config.yml: mongo.uri, public_base_url, auth.jwt_secret,
# admin.bootstrap_emails, mailgun (optional) -- every field is commented.
go build -o runguard-server .
./runguard-server
```

Listens on `server.addr` (default `:8888`). `config.yml` is gitignored —
never commit it, it holds live secrets.

**Frontend**

```sh
cd frontend
cp .env.example .env
# edit .env -- see src/siteConfig.js for what each VITE_* var controls.
npm install
npm run build
```

This produces a static `dist/` you serve with any web server/reverse proxy
(nginx, Caddy, etc). The backend is API-only — point your proxy's `/api` and
`/hb` paths at the backend process and everything else at `dist/`.
[`deploy/nginx/runguard.conf.example`](deploy/nginx/runguard.conf.example) is
a sample nginx vhost doing exactly that (TLS, static frontend, proxied
`/api`+`/hb`, optional Basic Auth in front of the web UI only).

**Running it**: [`deploy/systemd/runguard.service`](deploy/systemd/runguard.service)
is a sample systemd unit for the backend (see the comment at its top for the
install steps). `_build/build_go.sh` and `build_preact.sh` are the author's
own SSH-based deploy scripts on top of that (they assume the systemd service
and nginx vhost already exist, and your own SSH key) — adapt them, replace
them with your own process manager, or just run the binary directly.

**First admin account**: sign up normally through the web UI, then see
"Admin access" below.


# Agent (CLI)

`runguard wrap -- your-command` captures stdout/stderr/exit code/duration and
posts a heartbeat; `runguard register` self-registers a job using an account
agent token instead of a pre-existing heartbeat key (view/regenerate the
token from the account page). See [`docs/AGENT_DATA.md`](docs/AGENT_DATA.md)
for exactly what data the CLI sends and when, and
[`docs/KUBERNETES.md`](docs/KUBERNETES.md) for monitoring Kubernetes
CronJobs specifically.

**The CLI is open source**, in its own standalone repo:
[`github.com/smega/runguard-cli`](https://github.com/smega/runguard-cli)
(stdlib-only, no dependency on this monorepo). That repo is the canonical
public source for users who want to read or build the client themselves —
its README is the public-facing version of `docs/AGENT_DATA.md`. The copy
under `backend/cmd/runguard/` in this repo stays in sync by hand. Releases
are published from the standalone repo's own `release.yml` (tag push →
checksummed GitHub Release binaries) — there's no self-hosted install
script or download server to maintain.


# Admin access

Add an email to `admin.bootstrap_emails` in `backend/config.yml` and restart
the backend — that account is promoted to admin automatically (idempotent,
safe to leave configured permanently). From there, admins can promote/demote
other users from the in-app Admin page (`/admin`, only visible to admins),
which also keeps an audit log of who granted/revoked admin access and when.
The last remaining admin can't be demoted, so this can't lock you out.

(The old approach of running a `db.users.updateOne(...)` command by hand in
mongosh still works if you ever need it, but shouldn't be necessary anymore.)


## Limits

RunGuard isn't monetized — there's no plan/tier system, just an optional
per-user cap. Set `limits.max_jobs_per_user` / `limits.max_projects_per_user`
in `backend/config.yml` (0 or omitted = unlimited, the right value for a
self-hosted instance). The author's own hosted instance sets these low
purely to keep it small, not as a paid-tier upsell.


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
