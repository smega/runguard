# What the `runguard` CLI sends

This is a plain description of exactly what data the CLI agent transmits to
RunGuard, and when. It exists because "install a binary that watches your
cron jobs" is a reasonable thing to want documented before you point it at
anything you care about — this is aimed at that self-hosted/homelab audience,
not written as after-the-fact legal cover.

The CLI itself is open source: [`github.com/smega/runguard-cli`](https://github.com/smega/runguard-cli).
Everything described below is verifiable directly in that source rather than
just asserted here.

There are two commands that talk to the network: `wrap` and `register`.
`self-update` also makes network calls (to fetch a new binary); that's
covered separately at the bottom.

## `runguard wrap -- your-command`

Every time a wrapped command finishes, the CLI sends one HTTP POST to your
job's heartbeat URL (`https://<your-runguard-host>/hb/<key>`, or wherever
`--endpoint`/`RUNGUARD_ENDPOINT` points). No authentication beyond the
heartbeat key itself is required for this request — that's unchanged from
plain `curl`-based heartbeats.

The request body contains:

- `status` — `"ok"` or `"fail"`, derived from the command's exit code.
- `exit_code` — the wrapped command's actual exit code.
- `duration_ms` — how long the command (including retries, if any) took to run.
- `env` — only if you passed `--env=`. An arbitrary label you chose (e.g. `"prod"`); not auto-detected.
- `tags` — only if you passed `--tag=`, plus an automatic `retried:N` tag if `--retry` caused extra attempts.
- `agent_version` — the CLI's own version string (e.g. `"0.1.4"`), so RunGuard can flag jobs on old/unsupported agent versions.
- `stdout_tail` / `stderr_tail` — the **last `--truncate` bytes** (default 4000) of the command's stdout and stderr, captured separately. This is genuinely the tail end of whatever your command printed — nothing is added, redacted, or filtered by the CLI itself. If your command prints secrets, those secrets end up in this field. The server additionally caps how much of this it stores (`defaultCaptureLimits` in the backend); the CLI-side `--truncate` is a client-side cap on top of that.
- `ts` — a UTC timestamp taken right after the command finishes.

What it does **not** send: no hostname, no environment variables (other than the one string you explicitly pass via `--env`), no file paths, no command line/arguments, no network configuration, no OS/user info. The server does see your request's source IP as part of ordinary HTTP, the same as any web request — that's not something the CLI adds, it's just how HTTP works.

If the heartbeat POST itself fails (network error, non-2xx response), the CLI logs that failure to its own stderr and exits with the wrapped command's real exit code regardless — a broken connection to RunGuard never changes whether your job is considered to have succeeded or failed for the purposes of its own exit status.

## `runguard register`

Sends one HTTP POST to `<api-base>/api/agent/register`, authenticated with
`Authorization: Bearer <your-agent-token>` (the token from the account
page — see below). Body:

- `name` — the `--name` you passed.
- `interval_minutes`, `grace_minutes`, `severity` — the values you passed (or their defaults).

That's the entire payload. It does not send the command you intend to wrap, your token is never echoed back in the response, and repeated calls with the same `--name` return the existing job instead of creating a new one (so re-running a setup script is safe).

## Agent tokens

The agent token (visible/regenerable from the account page) is a bearer credential scoped to one thing: creating/looking-up jobs via `register`. It is not your login password and doesn't grant access to the web UI. Regenerating it immediately invalidates the old value — any script still using the old token will start getting 401s from `register` (existing heartbeats using job keys are unaffected, since those don't use the agent token at all).

## `runguard self-update`

Fetches release metadata and a binary from GitHub's public Releases API (`api.github.com`) over plain HTTPS GET requests — no data about your machine is sent as part of this beyond what's inherent in making an HTTP request (IP, User-Agent). The downloaded binary's SHA256 checksum is verified before it replaces the currently running binary; if no checksum is available or it doesn't match, the update is refused rather than installed anyway.
