// components/Toast.jsx
//
// A small, non-blocking notification stack — replaces native
// window.confirm()/alert() popups, which interrupt the whole page and
// can't be styled or positioned. Mount <ToastHost /> once near the root
// of the app; call showToast(...) from anywhere to post a message.
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";

let idCounter = 0;
let listeners = [];

function notify(updater) {
  listeners.forEach((fn) => fn(updater));
}

/**
 * Show a toast. Returns the toast id (useful if you want to dismiss it early).
 * opts.type: "info" | "success" | "error" (default "info")
 * opts.duration: ms before auto-dismiss, 0 disables auto-dismiss (default 4000)
 */
export function showToast(message, opts = {}) {
  const { type = "info", duration = 4000 } = opts;
  const id = ++idCounter;
  notify((prev) => [...prev, { id, message, type }]);
  if (duration > 0) {
    setTimeout(() => dismissToast(id), duration);
  }
  return id;
}

export function dismissToast(id) {
  notify((prev) => prev.filter((t) => t.id !== id));
}

const typeStyles = {
  info: "bg-white border-slate-200 text-slate-700",
  success: "bg-emerald-50 border-emerald-200 text-emerald-700",
  error: "bg-red-50 border-red-200 text-red-700",
};

// Renders the toast stack, anchored to the top-right of the viewport.
// Mount exactly once (in App.jsx) — every showToast() call anywhere in
// the app reaches this single instance via the module-level listener list.
export function ToastHost() {
  const [toasts, setToasts] = useState([]);

  useEffect(() => {
    listeners.push(setToasts);
    return () => {
      listeners = listeners.filter((fn) => fn !== setToasts);
    };
  }, []);

  if (toasts.length === 0) return null;

  return (
    <div class="fixed top-4 right-4 z-50 flex flex-col gap-2 w-72 max-w-[calc(100vw-2rem)] pointer-events-none">
      {toasts.map((t) => (
        <div
          key={t.id}
          class={
            "pointer-events-auto rounded-xl border shadow-lg px-3 py-2.5 text-xs flex items-start gap-2 transition-opacity " +
            (typeStyles[t.type] || typeStyles.info)
          }
        >
          <span class="flex-1 leading-snug">{t.message}</span>
          <button
            type="button"
            onClick={() => dismissToast(t.id)}
            class="shrink-0 text-slate-400 hover:text-slate-600 leading-none text-sm"
            aria-label="Dismiss"
          >
            &times;
          </button>
        </div>
      ))}
    </div>
  );
}
