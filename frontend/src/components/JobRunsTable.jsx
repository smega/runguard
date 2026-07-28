// frontend/src/components/JobRunsTable.jsx
import { Fragment } from "preact";
import { useState } from "preact/hooks";

function LogBlock({ title, tail, truncated }) {
  if (!tail) return null;
  return (
    <div class="border border-slate-800 rounded-md bg-slate-900/70 text-slate-100 text-xs p-3 relative">
      <div class="flex items-center justify-between mb-1">
        <span class="font-mono uppercase tracking-wide text-[0.65rem] text-slate-400">
          {title}
        </span>
        <button
          type="button"
          class="text-[0.65rem] text-slate-300 hover:text-white"
          onClick={() => tail && navigator.clipboard.writeText(tail)}
        >
          Copy
        </button>
      </div>
      <pre class="whitespace-pre-wrap break-words max-h-64 overflow-auto font-mono text-[0.7rem]">
        {tail}
      </pre>
      {truncated && (
        <div class="mt-1 text-[0.65rem] text-amber-400">
          Truncated to last captured bytes.
        </div>
      )}
    </div>
  );
}

export default function JobRunsTable({ jobId, runs }) {
  const [expandedId, setExpandedId] = useState(null);

  return (
    <div class="mt-4 border border-slate-800 rounded-xl overflow-hidden">
      <table class="min-w-full text-sm">
        <thead class="bg-slate-900/80">
          <tr>
            <th class="px-3 py-2 text-left text-xs font-semibold text-slate-300">
              Status
            </th>
            <th class="px-3 py-2 text-left text-xs font-semibold text-slate-300">
              Time
            </th>
            <th class="px-3 py-2 text-left text-xs font-semibold text-slate-300">
              Duration
            </th>
            <th class="px-3 py-2 text-left text-xs font-semibold text-slate-300">
              Exit
            </th>
            <th class="px-3 py-2 text-right text-xs font-semibold text-slate-300">
              Logs
            </th>
          </tr>
        </thead>
        <tbody class="bg-slate-950">
          {runs.map((run) => {
            const expanded = expandedId === run.id;
            const hasStdout = !!run.stdout_tail?.tail;
            const hasStderr = !!run.stderr_tail?.tail;
            const canExpand = hasStdout || hasStderr;

            return (
              <Fragment key={run.id}>
                <tr class="border-t border-slate-800">
                  <td class="px-3 py-2">
                    <span
                      class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs ${
                        run.status === "ok"
                          ? "bg-emerald-500/10 text-emerald-400"
                          : "bg-rose-500/10 text-rose-400"
                      }`}
                    >
                      {run.status}
                    </span>
                  </td>
                  <td class="px-3 py-2 text-xs text-slate-300">
                    {new Date(run.ts).toLocaleString()}
                  </td>
                  <td class="px-3 py-2 text-xs text-slate-300">
                    {run.duration_ms} ms
                  </td>
                  <td class="px-3 py-2 text-xs text-slate-300">
                    {run.exit_code ?? "—"}
                  </td>
                  <td class="px-3 py-2 text-right">
                    {canExpand && (
                      <button
                        type="button"
                        class="text-xs text-slate-300 hover:text-white underline underline-offset-2"
                        onClick={() =>
                          setExpandedId(expanded ? null : run.id)
                        }
                      >
                        {expanded ? "Hide logs" : "Show logs"}
                      </button>
                    )}
                  </td>
                </tr>

                {expanded && (
                  <tr class="border-t border-slate-800">
                    <td colSpan={5} class="px-3 py-3 bg-slate-950/80">
                      <div class="grid gap-3 md:grid-cols-2">
                        <LogBlock
                          title="stderr"
                          tail={run.stderr_tail?.tail}
                          truncated={run.stderr_tail?.truncated}
                        />
                        <LogBlock
                          title="stdout"
                          tail={run.stdout_tail?.tail}
                          truncated={run.stdout_tail?.truncated}
                        />
                      </div>

                      {/* optional webhook log viewer */}
                      <div class="mt-4">
                        <WebhookLogs jobId={jobId} runId={run.id} />
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
