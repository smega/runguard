// src/pages/WebhooksPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import {
  listWebhookTemplates,
  createWebhookTemplate,
  deleteWebhookTemplate,
} from "../api.js";

export default function WebhooksPage({ user }) {
  const [templates, setTemplates] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [name, setName] = useState("");
  const [url, setURL] = useState("");
  const [type, setType] = useState("generic");
  const [saving, setSaving] = useState(false);

  const [deletingId, setDeletingId] = useState(null);

  const [inlineMessage, setInlineMessage] = useState("");

  useEffect(() => {
    if (!user) return;
    loadTemplates();
  }, [user]);

  const loadTemplates = async (silent = false) => {
    if (!silent) {
      setLoading(true);
      setError("");
    }
    try {
      const res = await listWebhookTemplates();
      setTemplates(res.templates || []);
    } catch (err) {
      console.error("list templates", err);
      if (!silent) {
        setError(err.message || "Failed to load webhook templates.");
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!name.trim() || !url.trim()) return;
    setSaving(true);
    setError("");
    setInlineMessage("");
    try {
      const res = await createWebhookTemplate(name.trim(), url.trim(), type);
      setTemplates((prev) => [...prev, res.template]);
      setName("");
      setURL("");
      setType("generic");
      setInlineMessage("Template created.");
      setTimeout(() => setInlineMessage(""), 2000);
    } catch (err) {
      console.error(err);
      setError(err.message || "Failed to create template.");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (tmpl) => {
    setDeletingId(tmpl.id);
    setError("");
    setInlineMessage("");
    try {
      await deleteWebhookTemplate(tmpl.id);
      setTemplates((prev) => prev.filter((t) => t.id !== tmpl.id));
      setInlineMessage(`Deleted template "${tmpl.name}".`);
      setTimeout(() => setInlineMessage(""), 2000);
    } catch (err) {
      console.error(err);
      setError(err.message || "Failed to delete template.");
    } finally {
      setDeletingId(null);
    }
  };

  if (!user) return null;

  return (
    <div class="max-w-4xl mx-auto px-4 py-8">
      <h1 class="text-2xl font-semibold text-slate-900 mb-1">
        Webhook templates
      </h1>
      <p class="text-xs text-slate-500 mb-3">
        Define reusable webhook endpoints once, then attach them to individual
        jobs on the Jobs page. Templates can target Slack, Discord, PagerDuty,
        Microsoft Teams, or general JSON receivers.
      </p>

      {/* Create template */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm mb-4">
        <h2 class="text-sm font-semibold text-slate-900 mb-2">
          New webhook template
        </h2>
        <form class="space-y-2" onSubmit={handleSubmit}>
          <div class="grid grid-cols-1 sm:grid-cols-[minmax(0,1.1fr)_minmax(0,2fr)_minmax(0,1.2fr)] gap-2">
            <div>
              <label class="block text-[11px] text-slate-600 mb-1">
                Label
              </label>
              <input
                type="text"
                class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="Slack incidents channel"
                value={name}
                onInput={(e) => setName(e.target.value)}
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-600 mb-1">
                URL
              </label>
              <input
                type="text"
                class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="https://hooks.slack.com/services/…"
                value={url}
                onInput={(e) => setURL(e.target.value)}
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-600 mb-1">
                Type
              </label>
              <select
                class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={type}
                onInput={(e) => setType(e.target.value)}
              >
                <option value="generic">Generic JSON</option>
                <option value="slack">Slack (text payload)</option>
                <option value="discord">Discord (content payload)</option>
                <option value="pagerduty">PagerDuty (Events API v2)</option>
                <option value="teams">Microsoft Teams (MessageCard)</option>
              </select>
            </div>
          </div>

          {type === "slack" && (
            <div class="mt-2 text-[10px] text-slate-600 bg-slate-50 border border-slate-200 rounded-lg p-2">
              <p class="mb-1">
                Slack incoming webhooks expect a simple payload with a{" "}
                <code class="font-mono text-[9px] bg-white border border-slate-200 rounded px-1 py-0.5">
                  text
                </code>{" "}
                field, for example:
              </p>
              <pre class="bg-white border border-slate-200 rounded px-2 py-1 font-mono text-[9px] whitespace-pre overflow-x-auto">
{`{ "text": "RunGuard: job 'nightly-report' is late (critical)." }`}
              </pre>
              <p class="mt-1">
                Slack templates here automatically send a{" "}
                <code class="font-mono text-[9px] bg-white border border-slate-200 rounded px-1 py-0.5">
                  text
                </code>{" "}
                message that includes the job name, event type, and severity.
              </p>
            </div>
          )}

          {type === "discord" && (
            <div class="mt-2 text-[10px] text-slate-600 bg-slate-50 border border-slate-200 rounded-lg p-2">
              <p class="mb-1">
                Discord incoming webhooks expect a payload with a{" "}
                <code class="font-mono text-[9px] bg-white border border-slate-200 rounded px-1 py-0.5">
                  content
                </code>{" "}
                field, for example:
              </p>
              <pre class="bg-white border border-slate-200 rounded px-2 py-1 font-mono text-[9px] whitespace-pre overflow-x-auto">
{`{ "content": "RunGuard: job 'nightly-report' is late (critical)." }`}
              </pre>
              <p class="mt-1">
                Discord templates here automatically send a{" "}
                <code class="font-mono text-[9px] bg-white border border-slate-200 rounded px-1 py-0.5">
                  content
                </code>{" "}
                message the same way Slack templates do, trimmed to Discord's
                2000-character limit if needed.
              </p>
            </div>
          )}

          {type === "pagerduty" && (
            <div class="mt-2 text-[10px] text-slate-600 bg-slate-50 border border-slate-200 rounded-lg p-2">
              <p class="mb-1">
                PagerDuty's Events API v2 has one fixed endpoint and expects
                the destination "integration key" (routing key) in the
                request body rather than the URL. Since templates here only
                have one URL field, append it as a query parameter — paste:
              </p>
              <pre class="bg-white border border-slate-200 rounded px-2 py-1 font-mono text-[9px] whitespace-pre overflow-x-auto">
{`https://events.pagerduty.com/v2/enqueue?routing_key=YOUR_INTEGRATION_KEY`}
              </pre>
              <p class="mt-1">
                RunGuard reads the routing key back out of that query
                parameter and sends an <code class="font-mono text-[9px] bg-white border border-slate-200 rounded px-1 py-0.5">event_action: "trigger"</code>{" "}
                with the job's severity mapped onto PagerDuty's
                critical/error/warning/info scale, and a{" "}
                <code class="font-mono text-[9px] bg-white border border-slate-200 rounded px-1 py-0.5">dedup_key</code>{" "}
                per job so repeat alerts update the same incident instead of
                opening a new one each time. There's no separate "resolve"
                event yet — incidents need to be acknowledged/resolved in
                PagerDuty itself.
              </p>
            </div>
          )}

          {type === "teams" && (
            <div class="mt-2 text-[10px] text-slate-600 bg-slate-50 border border-slate-200 rounded-lg p-2">
              <p class="mb-1">
                Uses the classic Microsoft Teams "Incoming Webhook" connector
                card format:
              </p>
              <pre class="bg-white border border-slate-200 rounded px-2 py-1 font-mono text-[9px] whitespace-pre overflow-x-auto">
{`{ "@type": "MessageCard", "text": "RunGuard: job 'nightly-report' is late." }`}
              </pre>
              <p class="mt-1">
                Note: Microsoft has been migrating newer Teams channels
                towards Power Automate workflow webhooks instead of classic
                connectors — if a channel's webhook URL doesn't accept this
                format, check whether it needs a workflow-based webhook
                instead.
              </p>
            </div>
          )}

          <div class="flex items-center justify-between mt-2">
            <button
              type="submit"
              disabled={saving || !name.trim() || !url.trim()}
              class="px-4 py-2 rounded-lg text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60 shadow-sm"
            >
              {saving ? "Saving…" : "Add template"}
            </button>
            <div class="text-[11px] text-slate-500">
              Generic JSON templates receive the full RunGuard payload; Slack,
              Discord, PagerDuty, and Teams templates each get a payload
              shaped for that receiver.
            </div>
          </div>
          {inlineMessage && (
            <p class="text-[11px] text-emerald-700 mt-1">{inlineMessage}</p>
          )}
          {error && (
            <p class="text-[11px] text-amber-700 mt-1">{error}</p>
          )}
        </form>
      </div>

      {/* Existing templates */}
      <div class="bg-white border border-slate-200 rounded-2xl p-4 shadow-sm">
        <div class="flex items-center justify-between mb-2">
          <h2 class="text-sm font-semibold text-slate-900">
            Existing templates
          </h2>
          <button
            type="button"
            onClick={() => loadTemplates()}
            class="text-[11px] text-slate-500 hover:text-slate-800"
          >
            Refresh
          </button>
        </div>

        {loading ? (
          <p class="text-[11px] text-slate-500">Loading templates…</p>
        ) : templates.length === 0 ? (
          <p class="text-[11px] text-slate-500">
            You don&apos;t have any webhook templates yet. Create one above and
            then attach it to jobs from the Jobs page.
          </p>
        ) : (
          <div class="max-h-[22rem] overflow-y-auto border border-slate-200 rounded-xl">
            <table class="w-full text-[11px] text-left">
              <thead class="sticky top-0 bg-slate-50 border-b border-slate-200 text-slate-500">
                <tr>
                  <th class="py-2 px-3 font-medium">Name</th>
                  <th class="py-2 px-3 font-medium">Type</th>
                  <th class="py-2 px-3 font-medium">URL</th>
                  <th class="py-2 px-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {templates.map((tmpl) => {
                  const shortUrl =
                    tmpl.url && tmpl.url.length > 80
                      ? tmpl.url.slice(0, 77) + "…"
                      : tmpl.url || "—";

                  const typeLabel =
                    tmpl.type === "slack"
                      ? "Slack"
                      : tmpl.type === "discord"
                      ? "Discord"
                      : tmpl.type === "pagerduty"
                      ? "PagerDuty"
                      : tmpl.type === "teams"
                      ? "Teams"
                      : "Generic JSON";
                  const typeClasses =
                    "inline-flex items-center px-2 py-0.5 rounded-full text-[10px] border " +
                    (tmpl.type === "slack"
                      ? "bg-purple-50 border-purple-200 text-purple-700"
                      : tmpl.type === "discord"
                      ? "bg-indigo-50 border-indigo-200 text-indigo-700"
                      : tmpl.type === "pagerduty"
                      ? "bg-emerald-50 border-emerald-200 text-emerald-700"
                      : tmpl.type === "teams"
                      ? "bg-blue-50 border-blue-200 text-blue-700"
                      : "bg-slate-50 border-slate-200 text-slate-600");

                  return (
                    <tr
                      key={tmpl.id}
                      class="border-b border-slate-100 last:border-0"
                    >
                      <td class="py-2 px-3 text-slate-900">
                        {tmpl.name || "—"}
                      </td>
                      <td class="py-2 px-3">
                        <span class={typeClasses}>{typeLabel}</span>
                      </td>
                      <td
                        class="py-2 px-3 text-slate-700 font-mono max-w-[16rem] truncate"
                        title={tmpl.url}
                      >
                        {shortUrl}
                      </td>
                      <td class="py-2 px-3 text-right">
                        <button
                          type="button"
                          onClick={() => handleDelete(tmpl)}
                          disabled={deletingId === tmpl.id}
                          class="px-3 py-1 rounded-full border border-red-200 text-[11px] text-red-600 hover:bg-red-50 disabled:opacity-60"
                        >
                          {deletingId === tmpl.id ? "Deleting…" : "Delete"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        <p class="mt-2 text-[10px] text-slate-500">
          Deleting a template will stop future webhooks from being sent to that
          URL, even if jobs still list it. Jobs will not be able to re-add it
          until you create a template with that URL again.
        </p>
      </div>
    </div>
  );
}
