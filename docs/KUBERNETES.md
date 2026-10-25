# Monitoring Kubernetes CronJobs with RunGuard

RunGuard doesn't need any Kubernetes-specific integration to monitor a
CronJob — the `runguard` CLI is a static binary that works the same way
inside a container as it does on a bare VPS. Wrap the container's actual
command with `runguard wrap` and heartbeats work exactly as they would
anywhere else.

## Minimal example

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-orders-sync
spec:
  schedule: "0 3 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: orders-sync
              image: your-registry/orders-sync:latest
              command:
                - /bin/sh
                - -c
                - |
                  curl -fsSL https://runguard.smega.eu/install.sh | sh
                  runguard wrap --key=YOUR_HEARTBEAT_KEY -- /app/orders_sync.sh
          restartPolicy: OnFailure
```

If your own image doesn't have `curl`, download the binary in an init
container instead and mount it into the main container, or bake it into your
image at build time — the CLI is a single static binary with no runtime
dependencies.

## Matching the cron schedule

Since a CronJob's `schedule` field is itself a cron expression, create the
matching RunGuard job with a cron expression rather than a plain interval —
that way RunGuard's lateness detection lines up with when Kubernetes actually
intends to run the job, instead of approximating it as a fixed interval. Use
the "Cron expression" schedule type on the job creation form, pass
`cron_expr` to the raw API, or pass `--cron-expr` to `runguard register`:

```yaml
KEY=$(runguard register --token=$RUNGUARD_AGENT_TOKEN --name="nightly-orders-sync" --cron-expr="0 3 * * *" | grep 'Heartbeat key' | awk '{print $3}')
```

Reuse the exact same string as the CronJob's own `spec.schedule` so the two
never drift apart.

## Self-registration instead of a pre-shared key

If you're rolling this out to many CronJobs and don't want to create a job
and copy a heartbeat key by hand for each one, use `runguard register`
instead (see the account page for your agent token):

```yaml
command:
  - /bin/sh
  - -c
  - |
    curl -fsSL https://runguard.smega.eu/install.sh | sh
    KEY=$(runguard register --token=$RUNGUARD_AGENT_TOKEN --name="nightly-orders-sync" --interval=1440 | grep 'Heartbeat key' | awk '{print $3}')
    runguard wrap --key="$KEY" -- /app/orders_sync.sh
```

Pass `RUNGUARD_AGENT_TOKEN` via a Kubernetes Secret rather than hardcoding it
in the manifest.

## What this deliberately isn't

There's no RunGuard admission webhook or auto-injection sidecar — a plain
controller (not a webhook) turned out to cover the same ground with a lot
less to maintain, see below.

## Automatic auto-discovery and wrapping (`operator/`)

If editing every CronJob's command by hand doesn't scale, `operator/` in
this repo is a small controller that watches CronJobs across a cluster and,
for any CronJob annotated `runguard.io/monitor: "true"`, automatically
registers it (using its own `spec.schedule`) and patches its command to
route through `runguard wrap` — no heartbeat key to copy, no manifest
editing beyond the one annotation. See `operator/README.md` for setup and
its one real limitation: it can only wrap a container that already sets an
explicit `command` (it has no way to see what an image's own
`ENTRYPOINT`/`CMD` would run). If that doesn't fit your CronJob, the manual
approach above still works exactly the same.
