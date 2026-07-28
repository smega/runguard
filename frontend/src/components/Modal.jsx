import { useEffect, useRef } from "preact/hooks";
import { createPortal } from "preact/compat";

export default function Modal({
  open,
  title,
  onClose,
  children,
  maxWidthClass = "max-w-5xl",
}) {
  const panelRef = useRef(null);

  useEffect(() => {
    if (!open) return;

    const onKeyDown = (e) => {
      if (e.key === "Escape") onClose?.();
    };

    document.addEventListener("keydown", onKeyDown);

    // prevent background scroll
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    // focus the panel for keyboard users
    setTimeout(() => panelRef.current?.focus?.(), 0);

    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = prevOverflow;
    };
  }, [open, onClose]);

  if (!open) return null;

  const modal = (
    <div class="fixed inset-0 z-50">
      {/* Backdrop */}
      <div
        class="absolute inset-0 bg-black/60"
        onClick={() => onClose?.()}
      />

      {/* Panel */}
      <div class="absolute inset-0 flex items-center justify-center p-3 sm:p-6">
        <div
          ref={panelRef}
          tabIndex={-1}
          role="dialog"
          aria-modal="true"
          class={`w-[min(95vw,1100px)] ${maxWidthClass} max-h-[85vh] rounded-2xl bg-slate-900 text-slate-100 shadow-2xl ring-1 ring-white/10 flex flex-col`}
          onClick={(e) => e.stopPropagation()}
        >
          <div class="flex items-center justify-between gap-3 px-4 py-3 border-b border-white/10">
            <div class="font-semibold text-base sm:text-lg">{title}</div>
            <button
              class="rounded-lg px-3 py-1.5 text-sm bg-white/10 hover:bg-white/15"
              onClick={() => onClose?.()}
              type="button"
            >
              Close
            </button>
          </div>

          {/* Body scrolls */}
          <div class="p-4 overflow-auto">
            {children}
          </div>
        </div>
      </div>
    </div>
  );

  return createPortal(modal, document.body);
}
