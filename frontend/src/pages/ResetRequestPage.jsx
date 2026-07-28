// src/pages/ResetRequestPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { requestPasswordReset } from "../api.js";
import { route } from "preact-router";

export default function ResetRequestPage({ user }) {
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (user) {
      route("/profile", true);
    }
  }, [user]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setBusy(true);
    setMessage("");
    try {
      const res = await requestPasswordReset(email);
      setMessage(
        res.message ||
          "If this email exists, a reset link has been sent to your inbox."
      );
    } catch (err) {
      console.error(err);
      setMessage(err.message || "Something went wrong.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="max-w-md mx-auto px-4 py-10">
      <div class="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm text-sm text-slate-700">
        <h1 class="text-lg font-semibold text-slate-900 mb-1">
          Reset your password
        </h1>
        <p class="text-xs text-slate-500 mb-4">
          We&apos;ll email you a one-time link to set a new password if this
          address exists in RunGuard.
        </p>
        {message && (
          <div class="text-xs text-slate-700 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 mb-3">
            {message}
          </div>
        )}
        <form class="space-y-3" onSubmit={handleSubmit}>
          <div>
            <label class="block text-xs text-slate-600 mb-1">Email</label>
            <input
              type="email"
              class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={email}
              onInput={(e) => setEmail(e.target.value)}
            />
          </div>
          <button
            type="submit"
            disabled={busy}
            class="w-full mt-2 py-2 rounded-lg text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60 shadow-sm"
          >
            {busy ? "Requesting link…" : "Send reset link"}
          </button>
        </form>
      </div>
    </div>
  );
}
