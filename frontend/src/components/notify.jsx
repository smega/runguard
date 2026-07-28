import { createContext } from "preact";
import { useCallback, useContext, useMemo, useState } from "preact/hooks";

const NotifyCtx = createContext(null);

function toneClasses(type) {
  if (type === "success") return "border-emerald-200 bg-emerald-50 text-emerald-900";
  if (type === "error") return "border-rose-200 bg-rose-50 text-rose-900";
  if (type === "warning") return "border-amber-200 bg-amber-50 text-amber-900";
  return "border-slate-200 bg-white text-slate-900";
}

function dotClasses(type) {
  if (type === "success") return "bg-emerald-500";
  if (type === "error") return "bg-rose-500";
  if (type === "warning") return "bg-amber-500";
  return "bg-slate-400";
}

export function NotifyProvider({ children }) {
  const [toasts, setToasts] = useState([]);

  const remove = useCallback((id) => {
    setToasts((t) => t.filter((x) => x.id !== id));
  }, []);

  const notify = useCallback((message, type = "info", ttlMs = 3500) => {
    const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
    setToasts((t) => [...t, { id, message, type }]);

    if (ttlMs > 0) {
      window.setTimeout(() => remove(id), ttlMs);
    }
  }, [remove]);

  const value = useMemo(() => ({ notify }), [notify]);

  return (
    <NotifyCtx.Provider value={value}>
      {children}

      {/* Toast viewport */}
      <div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 w-[min(380px,calc(100vw-2rem))]">
        {toasts.map((t) => (
          <div
            key={t.id}
            class={[
              "rounded-2xl border shadow-sm bg-white/90 backdrop-blur px-3 py-2.5",
              "flex items-start gap-2",
              toneClasses(t.type)
            ].join(" ")}
            role="status"
            aria-live="polite"
          >
            <span class={`mt-1 h-2 w-2 rounded-full ${dotClasses(t.type)}`} />
            <div class="text-sm leading-snug flex-1">
              {t.message}
            </div>
            <button
              type="button"
              class="ml-1 text-slate-500 hover:text-slate-800 text-xs font-mono px-1"
              onClick={() => remove(t.id)}
              aria-label="Dismiss"
              title="Dismiss"
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </NotifyCtx.Provider>
  );
}

export function useNotify() {
  const ctx = useContext(NotifyCtx);
  if (!ctx?.notify) return () => {};
  return ctx.notify;
}
