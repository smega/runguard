// src/pages/LoginPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { login, requestMagicLink } from "../api.js";
import { route } from "preact-router";

export default function LoginPage({ onLogin, user }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [magicMessage, setMagicMessage] = useState("");
  const [magicBusy, setMagicBusy] = useState(false);

  useEffect(() => {
    if (user) {
      route("/account", true);
    }
  }, [user]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const res = await login(email, password);
      onLogin(res.user);
    } catch (err) {
      console.error(err);
      setError(err.message || "Login failed");
    } finally {
      setBusy(false);
    }
  };

  const handleMagic = async (e) => {
    e.preventDefault();
    setMagicMessage("");
    setMagicBusy(true);
    try {
      const res = await requestMagicLink(email);
      setMagicMessage(
        res.message ||
          "If this email exists, a magic link has been sent to your inbox."
      );
    } catch (err) {
      console.error(err);
      setMagicMessage(err.message || "Could not create magic link.");
    } finally {
      setMagicBusy(false);
    }
  };

  return (
    <div class="max-w-md mx-auto px-4 py-10">
      <div class="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
        <h1 class="text-xl font-semibold text-slate-900 mb-1">
          Sign in to RunGuard
        </h1>
        <p class="text-xs text-slate-500 mb-4">
          Use your password or request a one-time magic link.
        </p>
        {error && (
          <div class="text-xs text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2 mb-3">
            {error}
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
          <div>
            <label class="block text-xs text-slate-600 mb-1">Password</label>
            <input
              type="password"
              class="w-full px-3 py-2 rounded-lg bg-white border border-slate-300 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={password}
              onInput={(e) => setPassword(e.target.value)}
            />
          </div>
          <button
            type="submit"
            disabled={busy}
            class="w-full mt-2 py-2 rounded-lg text-sm font-medium bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-60 shadow-sm"
          >
            {busy ? "Signing in…" : "Sign in"}
          </button>
        </form>

        <div class="mt-5 border-t border-slate-200 pt-4 space-y-2">
          <button
            onClick={handleMagic}
            disabled={magicBusy || !email}
            class="w-full py-2 rounded-lg text-xs font-medium border border-slate-300 text-slate-700 hover:bg-slate-50 disabled:opacity-60"
          >
            {magicBusy ? "Creating magic link…" : "Email me a magic link"}
          </button>
          {magicMessage && (
            <div class="text-[11px] text-slate-600 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2">
              {magicMessage}
            </div>
          )}
          <button
            onClick={() => route("/reset-request")}
            class="block w-full text-[11px] text-slate-500 hover:text-slate-800 text-left mt-1"
          >
            Forgot password?
          </button>
        </div>
      </div>
    </div>
  );
}
