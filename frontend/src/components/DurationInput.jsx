// src/components/DurationInput.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";

// A number + unit (minutes / hours / days) input for job intervals and grace
// periods. The API still takes minutes, so the parent keeps minutes in its
// state and gets minutes back from onChange; only the display is in units.

const FACTORS = { minutes: 1, hours: 60, days: 1440 };

// Show a stored value in the largest unit it divides into evenly:
// 10080 -> 7 days, 360 -> 6 hours, 90 -> 90 minutes.
function pickUnit(minutes) {
  if (minutes > 0 && minutes % FACTORS.days === 0) return "days";
  if (minutes > 0 && minutes % FACTORS.hours === 0) return "hours";
  return "minutes";
}

function toMinutes(text, unit) {
  if (text === "" || text == null) return "";
  return Math.round(Number(text) * FACTORS[unit]);
}

export default function DurationInput({ minutes, onChange, min = 0, size = "md" }) {
  const initialUnit = pickUnit(Number(minutes));
  const [unit, setUnit] = useState(initialUnit);
  const [text, setText] = useState(
    minutes === "" || minutes == null ? "" : String(Number(minutes) / FACTORS[initialUnit])
  );

  // Follow changes made by the parent (e.g. the form resetting after a
  // create), but not the echo of our own onChange.
  useEffect(() => {
    if (minutes === "" || minutes == null) return;
    if (toMinutes(text, unit) === Number(minutes)) return;
    const u = pickUnit(Number(minutes));
    setUnit(u);
    setText(String(Number(minutes) / FACTORS[u]));
  }, [minutes]);

  const small = size === "sm";
  const field = small
    ? "px-2 py-1 rounded text-[11px] focus:ring-1"
    : "px-3 py-2 rounded-lg text-sm focus:ring-2";
  const total = toMinutes(text, unit);

  return (
    <div>
      <div class="flex gap-2">
        <input
          type="number"
          min={min}
          step="any"
          class={`flex-1 min-w-0 bg-white border border-slate-300 text-slate-900 focus:outline-none focus:ring-blue-500 ${field}`}
          value={text}
          onInput={(e) => {
            setText(e.target.value);
            onChange(toMinutes(e.target.value, unit));
          }}
        />
        <select
          aria-label="Unit"
          class={`bg-white border border-slate-300 text-slate-900 focus:outline-none focus:ring-blue-500 ${field}`}
          value={unit}
          onChange={(e) => {
            setUnit(e.target.value);
            onChange(toMinutes(text, e.target.value));
          }}
        >
          <option value="minutes">minutes</option>
          <option value="hours">hours</option>
          <option value="days">days</option>
        </select>
      </div>
      {unit !== "minutes" && total !== "" && !small && (
        <p class="mt-1 text-[11px] text-slate-500">= {total} minutes</p>
      )}
    </div>
  );
}

// "7 days", "6 hours", "1 day 12 hours" style display for an interval in
// seconds (the API's unit).
export function formatDuration(seconds) {
  let m = Math.round(Number(seconds) / 60);
  if (!(m > 0)) return "0 min";
  const parts = [];
  const days = Math.floor(m / 1440);
  m -= days * 1440;
  const hours = Math.floor(m / 60);
  m -= hours * 60;
  if (days) parts.push(`${days} day${days === 1 ? "" : "s"}`);
  if (hours) parts.push(`${hours} hour${hours === 1 ? "" : "s"}`);
  if (m) parts.push(`${m} min`);
  return parts.join(" ");
}

// A cron expression that runs about every `minutes`. Cron can only express
// steps that divide the hour/day evenly, so other values get the nearest
// sensible schedule (the job's own interval + grace still decides lateness).
export function cronForInterval(minutes) {
  const m = Math.max(1, Math.round(Number(minutes)));
  if (m < 60) return `*/${m} * * * *`;
  if (m < 1440) {
    const h = Math.max(1, Math.round(m / 60));
    return h === 1 ? "0 * * * *" : `0 */${h} * * *`;
  }
  if (m === 10080) return "0 3 * * 0";
  const d = Math.max(1, Math.round(m / 1440));
  return d === 1 ? "0 3 * * *" : `0 3 */${d} * *`;
}
