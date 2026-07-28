// src/pages/ResetPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { confirmPasswordReset } from "../api.js";
import { route } from "preact-router";

export default function ResetPage({ onLogin }) {
  const [token, setToken] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [status, setStatus] = useState("idle"); // idle | ok | error

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const t = params.get("token");
    if (!t) {
      setStatus("error");
      setMessage("Missing or invalid reset token.");
      return;
    }
    setToken(t);
  }, []);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!token) return;
    setBusy(true);
    setMessage("");
    setStatus("idle");
    try {
      const res = await confirmPasswordReset(token, password);
      if (res.user) {
        onLogin(res.user);
        setStatus("ok");
        setMessage("Password updated. Redirecting to your profile…");
        setTimeout(() => route("/profile", true), 800);
      } else {
        setStatus("ok");
        setMessage("Password updated. You can now log in.");
        setTimeout(() => route("/login", true), 800);
      }
    } catch (err) {
      console.error(err);
      setStatus("error");
      setMessage(err.message || "Reset failed.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="max-w-md mx-auto px-4 py-10">
      <div class="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm text-sm text-slate-700">
        <h1 class="text-lg font-semibold text-slate-900 mb-2">
          Set a new password
        </h1>
        {message && (
          <div
            class={`text-xs px-3 py-2 mb-3 rounded-lg border ${
              status === "error"
                ? "bg-red-50 border-red-200 text-red-700"
                : "bg-slate-50 border-slate-200 text-slate-700"
            }`}
          >
            {message}
          </div>
        )}
        {token && (
          <form class="space-y-3" onSubmit={handleSubmit}>
            <div>
              <label class="block text-xs text-slate-600 mb-1">
                New password
              </label>
              <input
                type="password"
                class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={password}
                onInput={(e) => setPassword(e.target.value)}
              />
            </div>
                        <div class="text-xs text-slate-600 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2">
              Password must be 8-128 characters and include at least 1 letter, 1 number, and 1 special character.
            </div>
            <button
              type="submit"
              disabled={busy}
              class="w-full mt-2 py-2 rounded-lg text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60 shadow-sm"
            >
              {busy ? "Updating…" : "Update password"}
            </button>
          </form>
        )}
        {!token && !message && (
          <p class="text-xs text-slate-600">
            Missing or invalid reset token. Please request a new link.
          </p>
        )}
      </div>
    </div>
  );
}
