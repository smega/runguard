// src/pages/AgentPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { CLI_REPO } from "../siteConfig";

export default function AgentPage() {
  const [osArch, setOsArch] = useState({
    os: "linux",
    arch: "amd64",
    label: "Linux x86_64",
  });

  // Detect platform for a nicer one-liner
  useEffect(() => {
    try {
      const ua = (window.navigator && window.navigator.userAgent) || "";
      let os = "linux";
      let arch = "amd64";
      let label = "Linux x86_64";

      if (/Mac OS X/i.test(ua) || /Darwin/i.test(ua)) {
        os = "darwin";
        label = "macOS x86_64";
      }

      if (/arm64|aarch64|Apple M\d/i.test(ua)) {
        arch = "arm64";
        label = os === "darwin" ? "macOS arm64 (Apple Silicon)" : "Linux arm64";
      }

      setOsArch({ os, arch, label });
    } catch {
      // fall back to defaults
    }
  }, []);

  const repoURL = `https://github.com/${CLI_REPO}`;
  const releasesLatestURL = `${repoURL}/releases/latest`;
  const assetName = `runguard-${osArch.os}-${osArch.arch}`;
  // GitHub redirects /releases/download/latest/<asset> to the actual
  // latest tag's asset -- no API call needed just to get the right URL.
  const downloadCmd = `curl -fsSL ${repoURL}/releases/download/latest/${assetName} -o runguard && chmod +x runguard`;
  const buildFromSourceCmd = `git clone ${repoURL}.git\ncd $(basename ${repoURL})\ngo build -o runguard .`;

  return (
    <div class="max-w-4xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-semibold text-slate-900 mb-1">
        RunGuard agent & CLI wrapper
      </h1>
      <p class="text-xs text-slate-500 mb-2">
        A small Go binary that wraps your cron/CI/systemd job, captures
        failures, and always sends the right heartbeat back to RunGuard.
      </p>

      {/* Summary card */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm mb-4">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-2">
          <div>
            <h2 class="text-sm font-semibold text-slate-900">Install the CLI</h2>
            <p class="text-[11px] text-slate-500">
              The CLI is{" "}
              <a href={repoURL} target="_blank" rel="noopener noreferrer" class="text-indigo-600 hover:underline">
                open source
              </a>{" "}
              and distributed via GitHub Releases — checksummed binaries built
              directly from source, no self-hosted download server involved.
            </p>
          </div>
          <span class="text-[11px] text-slate-500 shrink-0">
            Platform detected:{" "}
            <span class="font-medium text-slate-800">{osArch.label}</span>
          </span>
        </div>

        {/* GitHub Release download */}
        <div class="mt-3">
          <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
            1. Download the latest release (recommended)
          </h3>
          <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
            <pre class="whitespace-pre-wrap break-all">{downloadCmd}</pre>
          </div>
          <p class="mt-1 text-[10px] text-slate-500">
            Each release also publishes a matching <code>.sha256</code>{" "}
            checksum file alongside every binary — verified automatically by{" "}
            <code>runguard self-update</code>, and safe to check by hand too.
            See all releases on{" "}
            <a href={releasesLatestURL} target="_blank" rel="noopener noreferrer" class="text-indigo-600 hover:underline">
              GitHub
            </a>.
          </p>
        </div>

        {/* Build from source */}
        <div class="mt-3">
          <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
            2. Or build from source
          </h3>
          <p class="text-[10px] text-slate-500 mb-1.5">
            Stdlib-only, so all you need is Go 1.24+.
          </p>
          <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
            <pre class="whitespace-pre-wrap break-all">{buildFromSourceCmd}</pre>
          </div>
        </div>
      </div>

      {/* How to wire it up */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm mb-4">
        <h2 class="text-sm font-semibold text-slate-900 mb-2">
          Wiring the wrapper into your jobs
        </h2>
        <p class="text-[11px] text-slate-500 mb-2">
          The <code>runguard</code> CLI wraps your existing job, captures
          stdout, stderr, and exit code, and always sends a heartbeat back to
          RunGuard. It also includes enough metadata for debugging without
          leaking credentials.
        </p>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
              Example: cron job
            </h3>
            <p class="text-[10px] text-slate-500 mb-1">
              Replace your direct command with a <code>runguard wrap</code>{" "}
              call:
            </p>
            <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
              <pre class="whitespace-pre leading-snug">
{`# Before:
0 * * * * /opt/scripts/orders_sync.sh

# After:
0 * * * * runguard wrap --key=YOUR_HEARTBEAT_KEY -- /opt/scripts/orders_sync.sh`}
              </pre>
            </div>
          </div>

          <div>
            <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
              Example: CI pipeline step
            </h3>
            <p class="text-[10px] text-slate-500 mb-1">
              Wrap the job step so failures are captured and surfaced:
            </p>
            <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
              <pre class="whitespace-pre leading-snug">
{`runguard wrap \\
  --key=YOUR_HEARTBEAT_KEY \\
  --tag=ci \\
  -- /usr/local/bin/run-report-generation`}
              </pre>
            </div>
          </div>
        </div>

        <p class="mt-3 text-[10px] text-slate-500">
          Heartbeats triggered via <code>runguard</code> look exactly like the
          existing <code>curl</code>-based ones, but you get richer failure
          diagnostics and less boilerplate on your side.
        </p>
      </div>
    </div>
  );
}
