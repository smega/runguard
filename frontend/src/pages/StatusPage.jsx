import { useEffect, useState } from "preact/hooks";
import { getPublicStatus } from "../api.js";

export default function StatusPage() {
  const [state, setState] = useState({ loading: true, ok: null, data: null, error: "" });

  async function load() {
    setState((s) => ({ ...s, loading: true }));
    try {
      const { ok, data } = await getPublicStatus();
      setState({ loading: false, ok, data, error: "" });
    } catch (err) {
      setState({ loading: false, ok: false, data: null, error: err.message || "Failed to reach RunGuard" });
    }
  }

  useEffect(() => { load(); }, []);

  const healthy = state.ok && state.data?.ok;

  return (
    <div class="max-w-2xl mx-auto px-4 py-12">
      <h1 class="text-xl font-semibold text-slate-900 mb-1">RunGuard status</h1>
      <p class="text-sm text-slate-500 mb-6">A monitoring tool ought to be able to tell you when it's the one having a bad day.</p>

      {state.loading && (
        <div class="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-500">Checking…</div>
      )}

      {!state.loading && (
        <div class={
          "rounded-2xl border p-6 " +
          (healthy ? "border-emerald-200 bg-emerald-50" : "border-red-200 bg-red-50")
        }>
          <div class="flex items-center gap-2">
            <span class={"h-2.5 w-2.5 rounded-full " + (healthy ? "bg-emerald-500" : "bg-red-500")} />
            <span class={"text-sm font-medium " + (healthy ? "text-emerald-800" : "text-red-800")}>
              {healthy ? "All systems operational" : "Something's not right"}
            </span>
          </div>

          {state.data && (
            <dl class="mt-4 grid grid-cols-2 gap-y-2 text-xs text-slate-600">
              <dt class="text-slate-400">Database</dt>
              <dd>{state.data.database}</dd>
              <dt class="text-slate-400">Checked at</dt>
              <dd>{state.data.time ? new Date(state.data.time).toLocaleString() : "—"}</dd>
            </dl>
          )}
          {state.error && <p class="mt-4 text-xs text-red-700">{state.error}</p>}
        </div>
      )}

      <button
        type="button"
        onClick={load}
        class="mt-4 text-xs px-3 py-1.5 rounded-full border border-slate-300 text-slate-700 hover:bg-slate-100"
      >
        Refresh
      </button>
    </div>
  );
}
