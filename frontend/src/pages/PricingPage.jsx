import { useEffect, useState } from "preact/hooks";
import { Link } from "preact-router/match";
import { getPricing } from "../api.js";

export default function PricingPage({ user }) {
  const [data, setData] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    getPricing().then(setData).catch((err) => setError(err.message || "Failed to load pricing"));
  }, []);

  const plans = data?.plans || [];
  const billing = data?.billing || [];
  const trialDays = data?.trial_days || 14;

  const priceFor = (planId) => billing.find((b) => b.plan_id === planId)?.price_monthly_gbp ?? 0;
  const highlightFor = (planId) => billing.find((b) => b.plan_id === planId)?.highlight;

  return (
    <div class="max-w-6xl mx-auto px-4 py-10 space-y-6">
      <div class="max-w-2xl">
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Pricing</h1>
        <p class="mt-2 text-sm text-slate-600">Pick the plan that matches how many jobs and projects you need. Each paid plan is designed around fast incident visibility, cleaner routing, and longer retention.</p>
        {error && <p class="mt-2 text-sm text-red-600">{error}</p>}
      </div>

      <div class="grid gap-4 md:grid-cols-3">
        {plans.map((plan) => (
          <section key={plan.id} class={(highlightFor(plan.id) ? "border-blue-300 shadow-md " : "") + "rounded-2xl border border-slate-200 bg-white p-5"}>
            <div class="flex items-start justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold text-slate-900">{plan.label}</h2>
                <p class="mt-1 text-3xl font-semibold text-slate-900">£{priceFor(plan.id)}<span class="text-sm font-normal text-slate-500">/month</span></p>
              </div>
              {highlightFor(plan.id) && <span class="rounded-full bg-blue-50 border border-blue-200 px-2 py-1 text-[11px] font-medium text-blue-700">Popular</span>}
            </div>
            <p class="mt-2 text-xs text-slate-500">{plan.max_jobs} jobs · {plan.max_projects || 1} project{(plan.max_projects || 1) === 1 ? "" : "s"} · {trialDays}-day trial</p>
            <ul class="mt-4 space-y-2 text-sm text-slate-700">
              {(plan.bullets || []).map((bullet) => (
                <li key={bullet} class="flex gap-2"><span class="mt-1 h-1.5 w-1.5 rounded-full bg-slate-300" /> <span>{bullet}</span></li>
              ))}
            </ul>
            <div class="mt-5">
              {user ? (
                <Link href="/account" class="inline-flex items-center rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800">Manage plan</Link>
              ) : (
                <Link href="/signup" class="inline-flex items-center rounded-full bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500">Start free trial</Link>
              )}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
