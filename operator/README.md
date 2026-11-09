# runguard-operator

Watches CronJobs across a Kubernetes cluster and, for any CronJob opted in
with an annotation, automatically:

1. registers a matching RunGuard job (via `/api/agent/register`, using the
   CronJob's own `spec.schedule` as the cron expression — see
   `docs/KUBERNETES.md` in the main repo for why that matters), and
2. patches the CronJob to wrap its container's command with `runguard wrap`,
   so heartbeats are sent with no changes to your own image or script.

This is the automated alternative to the manual `runguard wrap` editing
described in `docs/KUBERNETES.md` — read that doc first if you want to
understand the underlying mechanism this builds on, or if your CronJob's
container has no explicit `command` set (this operator can't auto-wrap that
case; see "Limitations" below).

## Opting in

Add one annotation to the CronJob you want monitored:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-orders-sync
  annotations:
    runguard.io/monitor: "true"
    # Optional:
    # runguard.io/job-name: "custom-name"        # default: <namespace>/<name>
    # runguard.io/severity: "high"                # low|normal|high|critical
spec:
  schedule: "0 3 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: orders-sync
              image: your-registry/orders-sync:latest
              command: ["/app/orders_sync.sh"] # must be explicit — see Limitations
```

That's it — no heartbeat key to copy, no `runguard wrap` to add by hand. The
operator's next reconcile pass (within `--poll-interval`, default 30s) will:

- register a RunGuard job named `<namespace>/nightly-orders-sync` with
  `cron_expr: "0 3 * * *"`
- create a `nightly-orders-sync-runguard-key` Secret in the same namespace
  holding the heartbeat key
- patch the CronJob to add an init container that downloads the `runguard`
  binary into a shared volume, and rewrite the container's command to
  `/runguard-bin/runguard wrap --key=$(RUNGUARD_KEY) -- /app/orders_sync.sh`
- annotate the CronJob `runguard.io/injected: "true"` so it's left alone on
  future passes

## Install

`.github/workflows/operator-image.yml` in this repo builds a multi-arch
operator image on every push — fork it and point it at your own container
registry (GHCR, Docker Hub, a self-hosted registry, whatever you use).

If that registry is private — including a GHCR package you own, which
inherits its repo's visibility — you'll need a pull secret even to pull your
own image:

```bash
kubectl create namespace runguard --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret docker-registry ghcr-pull-secret \
  --namespace=runguard \
  --docker-server=<your-registry-host> \
  --docker-username=<your-username> \
  --docker-password=<a-read-only-token-or-password>

kubectl apply -f deploy/rbac.yaml
# edit deploy/deployment.yaml: set the real image, agent token, --api, and
# any --namespace / --cluster-name / RUNGUARD_INSTALL_URL you want
kubectl apply -f deploy/deployment.yaml
```

If you'd rather not manage a pull secret, make the registry package public
instead and drop the commented `imagePullSecrets` entry in
`deploy/deployment.yaml`.

## Flags

```
--api             RunGuard API base URL (or RUNGUARD_API env var) — required
--agent-token     Agent token (or RUNGUARD_AGENT_TOKEN env var) — required
--cluster-name    Prefix for registered job names, to disambiguate the same
                  namespace/name across multiple clusters on one account
--namespace       Restrict to one namespace (default: all — needs the
                  ClusterRole in deploy/rbac.yaml; swap for a namespaced
                  Role if you only ever pass --namespace)
--poll-interval   How often to re-scan for CronJobs (default 30s)
--install-url     URL the init container downloads the runguard binary from
                  (or RUNGUARD_INSTALL_URL env var) — required for
                  auto-wrapping; without it, CronJobs still get registered
                  with RunGuard but are annotated as injection-skipped
--kubeconfig      For local/out-of-cluster testing; uses in-cluster config
                  by default
```

## Limitations

- **Requires an explicit `command` on the container.** If your container
  relies entirely on its image's own `ENTRYPOINT`/`CMD`, the Kubernetes API
  has no way to tell the operator what that is, so there's nothing safe to
  prepend `runguard wrap` to. The CronJob still gets registered with
  RunGuard (so you get a job + heartbeat key), but it's annotated
  `runguard.io/injection-skipped-reason: ...` instead of
  `runguard.io/injected: "true"`, and you fall back to the manual
  `runguard wrap` approach in `docs/KUBERNETES.md`.
- **Poll-based, not event-driven.** A newly-annotated CronJob can take up to
  `--poll-interval` to be picked up. Fine for a CronJob fleet (which changes
  rarely); not built for anything latency-sensitive.
- **One agent token per operator instance.** Every CronJob this operator
  manages registers under the same RunGuard account. Run one operator
  deployment per account/cluster pairing if you need more than one.
- **Does not un-wrap jobs you stop annotating.** Removing
  `runguard.io/monitor` stops further reconciliation but doesn't revert an
  already-injected CronJob back to its original command — do that by hand
  if you want to fully undo it.
