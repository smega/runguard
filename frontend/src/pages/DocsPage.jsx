export default function DocsPage() {
  return (
    <div class="max-w-4xl mx-auto px-4 py-10 space-y-4">
      <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Getting started</h1>
      <p class="text-sm text-slate-600">Create a project, add one job, send a heartbeat, then test both a failure and a recovery alert before rolling RunGuard out more widely.</p>
      <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm text-sm text-slate-700 space-y-2">
        <p>1. Create a project for a service or environment.</p>
        <p>2. Add a job with a realistic interval and grace window.</p>
        <p>3. Send a test heartbeat and confirm it appears in the dashboard.</p>
        <p>4. Add email and Slack webhook destinations.</p>
        <p>5. Simulate a failure and then a healthy recovery.</p>
      </div>
    </div>
  );
}
