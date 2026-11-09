// src/pages/AgentPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { CLI_DOWNLOAD_BASE, INSTALL_SCRIPT_URL } from "../siteConfig";

export default function AgentPage() {
  const [cliInfo, setCliInfo] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [osArch, setOsArch] = useState({
    os: "linux",
    arch: "amd64",
    label: "Linux x86_64",
  });

  // Detect platform for nicer manual download hints
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

  // Load CLI versions from public endpoint
  useEffect(() => {
    const load = async () => {
      setLoading(true);
      setError("");

      try {
        const res = await fetch("/api/public/cli/versions", {
          method: "GET",
        });

        const text = await res.text();
        let data = null;
        try {
          data = text ? JSON.parse(text) : null;
        } catch {
          // ignore JSON parse error, fall back to raw text
        }

        if (!res.ok) {
          throw new Error(
            (data && (data.error || data.detail)) ||
              text ||
              `HTTP ${res.status}`
          );
        }

        setCliInfo(data || {});
      } catch (err) {
        console.error("load cli versions", err);
        setError(err.message || "Failed to load CLI versions.");
      } finally {
        setLoading(false);
      }
    };

    load();
  }, []);

  const latest = cliInfo?.latest || "";
  const downloadBase = cliInfo?.download_base || CLI_DOWNLOAD_BASE;
  const versions = cliInfo?.versions || [];

  const installScriptURL = INSTALL_SCRIPT_URL;

  // Commands to show
  const quickInstallCmd = `curl -fsSL ${installScriptURL} | bash`;
  const pinnedInstallCmd = latest
    ? `curl -fsSL ${installScriptURL} | bash -s -- ${latest}`
    : quickInstallCmd;

  const manualDownloadCmd =
    latest && downloadBase
      ? `curl -fsSL ${downloadBase.replace(
          /\/+$/,
          ""
        )}/${latest}/runguard-${osArch.os}-${osArch.arch}-${latest} -o runguard && chmod +x runguard`
      : "";

  return (
    <div class="max-w-4xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-semibold text-slate-900 mb-1">
        RunGuard agent & CLI wrapper
      </h1>
      <p class="text-xs text-slate-500 mb-2">
        A small Go binary that wraps your cron/CI/systemd job, captures
        failures, and always sends the right heartbeat back to RunGuard.
      </p>

      {error && (
        <div class="mb-3 text-[11px] text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
          {error}
        </div>
      )}

      {/* Summary card */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm mb-4">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-2">
          <div>
            <h2 class="text-sm font-semibold text-slate-900">
              Current CLI versions
            </h2>
            <p class="text-[11px] text-slate-500">
              The installer script discovers the latest binary from the public
              downloads host. You can also pin to a specific version or download
              manually.
            </p>
          </div>
          <div class="flex flex-col items-end gap-1">
            <span class="text-[11px] text-slate-500">
              Platform detected:{" "}
              <span class="font-medium text-slate-800">{osArch.label}</span>
            </span>
            {loading ? (
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border border-slate-200 text-slate-500">
                Checking versions…
              </span>
            ) : latest ? (
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border border-emerald-200 bg-emerald-50 text-emerald-700">
                <span class="w-1.5 h-1.5 rounded-full bg-current/60 mr-1" />
                Latest: {latest}
              </span>
            ) : (
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border border-amber-200 bg-amber-50 text-amber-700">
                No versions discovered
              </span>
            )}
          </div>
        </div>

        {/* Quick install */}
        <div class="mt-3">
          <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
            1. Quick install (recommended)
          </h3>
          <p class="text-[10px] text-slate-500 mb-1.5">
            This pulls the installer script from the app and auto-detects the
            latest compatible binary from the downloads host.
          </p>
          <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
            <pre class="whitespace-pre-wrap break-all">{quickInstallCmd}</pre>
          </div>
        </div>

        {/* Pinned install */}
        <div class="mt-3">
          <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
            2. Pin to a specific version
          </h3>
          <p class="text-[10px] text-slate-500 mb-1.5">
            Use this if you want to keep staging/prod on a known CLI version.
          </p>
          <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
            <pre class="whitespace-pre-wrap break-all">
              {latest
                ? pinnedInstallCmd
                : "# Waiting for version info… use the quick install above for now"}
            </pre>
          </div>
        </div>

        {/* Manual download */}
        <div class="mt-3">
          <h3 class="text-[11px] font-semibold text-slate-900 mb-1">
            3. Manual download (air-gapped or custom installs)
          </h3>
          <p class="text-[10px] text-slate-500 mb-1.5">
            If you prefer to keep a binary in your own artifact store, download
            it once and redistribute internally.
          </p>
          <div class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 text-[11px] font-mono overflow-x-auto">
            <pre class="whitespace-pre-wrap break-all">
              {manualDownloadCmd ||
                "# Waiting for version info… once available, a per-platform curl command will appear here."}
            </pre>
          </div>
          <p class="mt-1 text-[10px] text-slate-500">
            Binaries are served from{" "}
            <span class="font-mono">
              {downloadBase.replace(/^https?:\/\//, "")}
            </span>
            . Each file has a matching <code>.sha256</code> checksum —
            verified automatically by the install script and self-update,
            and safe to check by hand too.
          </p>
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

      {/* Version list */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
        <h2 class="text-sm font-semibold text-slate-900 mb-2">
          Available CLI versions
        </h2>
        {loading ? (
          <p class="text-[11px] text-slate-500">Loading versions…</p>
        ) : versions.length === 0 ? (
          <p class="text-[11px] text-slate-500">
            No versions discovered. Once the downloads host exposes semver
            directories, they will appear here.
          </p>
        ) : (
          <div class="max-h-40 overflow-y-auto border border-slate-200 rounded-lg">
            <table class="w-full text-[11px] text-left">
              <thead class="bg-slate-50 border-b border-slate-200 text-slate-500">
                <tr>
                  <th class="py-1 px-3 font-medium">Version</th>
                  <th class="py-1 px-3 font-medium">Notes</th>
                </tr>
              </thead>
              <tbody>
                {versions.map((v) => (
                  <tr
                    key={v}
                    class={
                      "border-b border-slate-100 last:border-0" +
                      (v === latest ? " bg-emerald-50/40" : "")
                    }
                  >
                    <td class="py-1 px-3 text-slate-800">
                      <span class="font-mono">{v}</span>
                      {v === latest && (
                        <span class="ml-2 inline-flex items-center px-1.5 py-0.5 rounded-full text-[9px] bg-emerald-600 text-white">
                          latest
                        </span>
                      )}
                    </td>
                    <td class="py-1 px-3 text-slate-500">
                      CLI wrapper binary published under{" "}
                      <span class="font-mono">
                        {downloadBase.replace(/^https?:\/\//, "")}/{v}/
                      </span>
                      .
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
