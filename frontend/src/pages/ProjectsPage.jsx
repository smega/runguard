import { useEffect, useState } from "preact/hooks";
import { route } from "preact-router";
import { createProject, deleteProject, listProjects } from "../api.js";
import { showToast } from "../components/Toast.jsx";

export default function ProjectsPage({ user }) {
  const [projects, setProjects] = useState([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [environment, setEnvironment] = useState("production");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => { if (!user) route("/", true); }, [user]);
  useEffect(() => { if (user) reload(); }, [user]);

  async function reload() {
    try { setError(""); const res = await listProjects(); setProjects(res.projects || []); } catch (err) { setError(err.message || "Failed to load projects"); }
  }

  async function onCreate(e) {
    e.preventDefault();
    setBusy(true);
    try {
      await createProject({ name, description, environment });
      setName(""); setDescription(""); setEnvironment("production");
      await reload();
    } catch (err) { setError(err.message || "Failed to create project"); }
    finally { setBusy(false); }
  }

  async function onDelete(project) {
    try {
      await deleteProject(project.id);
      await reload();
      showToast(`Project "${project.name}" deleted.`, { type: "success" });
    } catch (err) {
      setError(err.message || "Failed to delete project");
    }
  }

  if (!user) return null;

  return (
    <div class="max-w-5xl mx-auto px-4 py-8 space-y-6">
      <div>
        <h1 class="text-xl font-semibold text-slate-900">Projects</h1>
        <p class="text-sm text-slate-500">Group jobs by service, environment, or team so incidents stay easier to reason about.</p>
      </div>
      {error && <div class="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>}
      <div class="grid gap-4 lg:grid-cols-[1.1fr,1.7fr]">
        <form onSubmit={onCreate} class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm space-y-3">
          <h2 class="text-sm font-semibold text-slate-900">New project</h2>
          <input value={name} onInput={(e) => setName(e.target.value)} placeholder="Payments API" class="w-full rounded-xl border border-slate-300 px-3 py-2 text-sm" />
          <input value={description} onInput={(e) => setDescription(e.target.value)} placeholder="Optional description" class="w-full rounded-xl border border-slate-300 px-3 py-2 text-sm" />
          <select value={environment} onChange={(e) => setEnvironment(e.target.value)} class="w-full rounded-xl border border-slate-300 px-3 py-2 text-sm">
            <option value="production">production</option>
            <option value="staging">staging</option>
            <option value="development">development</option>
          </select>
          <button disabled={busy || !name.trim()} class="rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50">{busy ? "Creating…" : "Create project"}</button>
        </form>
        <div class="rounded-2xl border border-slate-200 bg-white shadow-sm overflow-hidden">
          <div class="border-b border-slate-200 px-4 py-3"><h2 class="text-sm font-semibold text-slate-900">Current projects</h2></div>
          {projects.length === 0 ? <div class="px-4 py-6 text-sm text-slate-500">No projects yet. Start with one group per service or environment.</div> : (
            <table class="min-w-full text-sm">
              <thead class="bg-slate-50 text-xs uppercase tracking-wide text-slate-500"><tr><th class="px-4 py-2 text-left">Project</th><th class="px-4 py-2 text-left">Environment</th><th class="px-4 py-2 text-left">Jobs</th><th class="px-4 py-2 text-left">Open incidents</th><th class="px-4 py-2 text-right">Actions</th></tr></thead>
              <tbody class="divide-y divide-slate-100">
                {projects.map((project) => (
                  <tr key={project.id}>
                    <td class="px-4 py-3"><div class="font-medium text-slate-900">{project.name}</div><div class="text-xs text-slate-500">{project.description || project.slug}</div></td>
                    <td class="px-4 py-3 text-slate-600">{project.environment || "—"}</td>
                    <td class="px-4 py-3 text-slate-600">{project.job_count}</td>
                    <td class="px-4 py-3 text-slate-600">{project.open_incidents}</td>
                    <td class="px-4 py-3 text-right"><button onClick={() => onDelete(project)} title="Deletes this project immediately. Blocked if it still has jobs assigned to it." class="rounded-full border border-slate-300 px-3 py-1 text-xs text-slate-700 hover:bg-slate-100">Delete</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
