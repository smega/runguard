// src/pages/SignupPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { signup } from "../api.js";
import { route } from "preact-router";

export default function SignupPage({ onSignup, user }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (user) {
      route("/profile", true);
    }
  }, [user]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const res = await signup(email, password);
      onSignup(res.user);
    } catch (err) {
      console.error(err);
      setError(err.message || "Signup failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="max-w-md mx-auto px-4 py-10">
      <div class="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
        <h1 class="text-xl font-semibold text-slate-900 mb-1">
          Create a RunGuard account
        </h1>
        <p class="text-xs text-slate-500 mb-2">
          One account for all your monitored jobs.
        </p>
        <div class="text-xs text-slate-600 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 mb-4">
          Password must be 8-128 characters and include at least 1 letter, 1 number, and 1 special character.
        </div>
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
            {busy ? "Creating account…" : "Sign up"}
          </button>
        </form>
      </div>
    </div>
  );
}
