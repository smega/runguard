// src/components/VerifyEmailBanner.jsx
import { h } from "preact";
import { useState } from "preact/hooks";
import { resendVerificationEmail } from "../api.js";

// Shown while the account's address isn't confirmed: no email alerts or
// summaries go out until it is (webhooks work regardless).
export default function VerifyEmailBanner({ account }) {
  const [state, setState] = useState("idle"); // idle | sending | sent | error
  if (!account?.email_unverified) return null;

  const resend = () => {
    setState("sending");
    resendVerificationEmail()
      .then(() => setState("sent"))
      .catch(() => setState("error"));
  };

  return (
    <div class="mb-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-900 flex flex-wrap items-center gap-x-3 gap-y-1">
      <span>
        Confirm <strong>{account.email}</strong> to get email alerts and summaries. We sent you a link when you signed
        up. Webhook alerts already work.
      </span>
      {state === "sent" ? (
        <span class="font-medium">New link sent.</span>
      ) : (
        <button
          type="button"
          onClick={resend}
          disabled={state === "sending"}
          class="font-medium underline hover:no-underline disabled:opacity-60"
        >
          {state === "sending" ? "Sending…" : state === "error" ? "Couldn't send, try again" : "Resend the link"}
        </button>
      )}
    </div>
  );
}
