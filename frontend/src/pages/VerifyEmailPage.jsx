// src/pages/VerifyEmailPage.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { verifyEmail } from "../api.js";

// Landing page for the link in the "Confirm your email" message. Works
// without being logged in: the token alone identifies the account.
export default function VerifyEmailPage({ user }) {
  const [status, setStatus] = useState("loading"); // loading | ok | error
  const [message, setMessage] = useState("");

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token");
    if (!token) {
      setStatus("error");
      setMessage("This link is missing its token.");
      return;
    }
    verifyEmail(token)
      .then(() => {
        setStatus("ok");
        setMessage("Your email address is confirmed. RunGuard will now send your alerts and summaries there.");
      })
      .catch((err) => {
        setStatus("error");
        setMessage(
          (err.message || "This link could not be used.") +
            " You can request a new one from your account page."
        );
      });
  }, []);

  return (
    <div class="max-w-md mx-auto px-4 py-10">
      <div class="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm text-sm text-slate-700">
        <h1 class="text-lg font-semibold text-slate-900 mb-2">Confirm email</h1>
        {status === "loading" ? (
          <p class="text-xs text-slate-600">Checking your link…</p>
        ) : (
          <p class={"text-xs " + (status === "ok" ? "text-emerald-700" : "text-slate-700")}>{message}</p>
        )}
        {status !== "loading" && (
          <a href={user ? "/jobs" : "/login"} class="inline-block mt-4 text-xs font-medium text-blue-600 hover:underline">
            {user ? "Go to your jobs" : "Log in"}
          </a>
        )}
      </div>
    </div>
  );
}
