export default function SecurityPage() {
  return (
    <div class="max-w-4xl mx-auto px-4 py-10 space-y-4">
      <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Security</h1>
      <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm text-sm text-slate-700 space-y-2">
        <p>RunGuard stores job configuration, heartbeat metadata, incident state, and optional short log tails you choose to send.</p>
        <p>The wrapper/agent is designed for outbound heartbeats and does not require inbound remote execution.</p>
        <p>Avoid sending secrets in stdout/stderr tails and rotate webhook URLs if exposed.</p>
      </div>
    </div>
  );
}
