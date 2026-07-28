import { forwardRef } from 'preact/compat';

// **Placeholder** – use your existing ui.jsx.
// Ensure it exports the components used below or adapt imports accordingly.
export const Card = (p) => <div class="rounded-2xl border p-4 bg-white text-black shadow" {...p} />
export const CardHeader = (p) => <div class="mb-2" {...p} />
export const CardTitle = (p) => <h2 class="text-xl font-semibold" {...p} />
export const CardContent = (p) => <div {...p} />
export const Button = ({ class:cls='', ...p }) => <button class={`px-3 py-2 rounded-xl border shadow ${cls}`} {...p} />
export const Badge = ({ class:cls='', children }) => <span class={`inline-block px-2 py-1 rounded-lg text-xs border ${cls}`}>{children}</span>
// export const Modal = ({ open, onClose, children }) => open ? (
// <div class="fixed inset-0 z-50 flex items-center justify-center">
// <div class="absolute inset-0 bg-black/40" onClick={onClose} />
// <div class="relative w-full max-w-3xl rounded-2xl bg-white text-black shadow-xl p-6">{children}</div>
// </div>
// ) : null

export const Modal = ({ open, onClose, children }) =>
  open ? (
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div class="absolute inset-0 bg-black/40" onClick={onClose} />

      {/* Dialog */}
      <div class="relative w-full max-w-3xl rounded-2xl bg-white text-black shadow-xl border border-slate-200 overflow-hidden">
        {/* Always-visible close button */}
        {/* <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          class="absolute right-3 top-3 inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-700 shadow-sm hover:bg-slate-50"
        >
          ×
        </button> */}

        {/* Scrollable content */}
        <div class="max-h-[85vh] overflow-y-auto p-6">
          {children}
        </div>
      </div>
    </div>
  ) : null;

export const Checkbox = (p) => <input type="checkbox" {...p} />
export const Table = (p) => <table class="w-full text-sm" {...p} />
export const TableHead = (p) => <thead class="text-left text-slate-500" {...p} />
export const TableBody = (p) => <tbody {...p} />
export const TableRow = (p) => <tr class="border-b last:border-b-0" {...p} />
export const TableCell = (p) => <td class="py-2 pr-3 align-top" {...p} />


export function Label({ children, forId, class: cls = '' }) {
  return (
    <label htmlFor={forId} class={`block text-sm text-slate-600 mb-1 ${cls}`}>
      {children}
    </label>
  );
}

export const Input = forwardRef((props, ref) => (
  <input
    ref={ref}
    class={`border rounded-xl px-3 py-2 w-full focus:outline-none focus:ring-2 focus:ring-indigo-500 ${props.class || ''}`}
    {...props}
  />
));



export function Textarea({ class: cls = '', ...props }) {
  return (
    <textarea
      {...props}
      class={`w-full rounded-2xl border border-gray-300 bg-white px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 ${cls}`}
    />
  );
}

// ui.jsx (add these exports)

export function Select({ value, onChange, children, class: cls = '', ...rest }) {
  return (
    <select
      value={value}
      onChange={onChange}
      class={`h-10 px-3 rounded-2xl border border-gray-300 bg-white text-black shadow-sm outline-none focus:ring-2 focus:ring-indigo-200 ${cls}`}
      {...rest}
    >
      {children}
    </select>
  );
}

export function SelectItem({ value, children }) {
  return <option value={value}>{children}</option>;
}

export function Chip({ active, onClick, children, class: cls = '' }) {
  return (
    <button
      type="button"
      onClick={onClick}
      class={`px-3 h-8 rounded-full border text-sm ${
        active
          ? 'bg-indigo-600 text-white border-indigo-600'
          : 'bg-white text-black border-gray-300 hover:bg-gray-50'
      } ${cls}`}
    >
      {children}
    </button>
  );
}
export function Spinner({ size = 'md', class: className = '' }) {
  let sizeClasses = 'h-5 w-5';
  if (size === 'sm') sizeClasses = 'h-4 w-4';
  if (size === 'lg') sizeClasses = 'h-6 w-6';

  return (
    <svg
      class={`${sizeClasses} animate-spin text-slate-400 ${className}`}
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <circle
        class="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        stroke-width="4"
      />
      <path
        class="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
      />
    </svg>
  );
}