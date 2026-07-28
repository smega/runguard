// src/pages/MagicPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { consumeMagicLink } from "../api.js";
import { route } from "preact-router";

export default function MagicPage({ onLogin }) {
  const [status, setStatus] = useState("loading"); // loading | ok | error
  const [message, setMessage] = useState("");

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get("token");
    if (!token) {
      setStatus("error");
      setMessage("Missing token.");
      return;
    }

    consumeMagicLink(token)
      .then((res) => {
        onLogin(res.user);
        setStatus("ok");
        setMessage("Magic link accepted. Redirecting to your profile…");
        setTimeout(() => route("/profile", true), 800);
      })
      .catch((err) => {
        console.error(err);
        setStatus("error");
        setMessage(err.message || "Magic link could not be used.");
      });
  }, [onLogin]);

  return (
    <div class="max-w-md mx-auto px-4 py-10">
      <div class="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm text-sm text-slate-700">
        <h1 class="text-lg font-semibold text-slate-900 mb-2">Magic link</h1>
        {status === "loading" && (
          <p class="text-xs text-slate-600">
            Validating your one-time link. This may take a moment…
          </p>
        )}
        {status !== "loading" && (
          <p class="text-xs text-slate-700">{message}</p>
        )}
      </div>
    </div>
  );
}
