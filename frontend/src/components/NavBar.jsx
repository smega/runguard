// src/components/Navbar.jsx
import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import { Link } from "preact-router/match";
import { getJson } from "../api.js";

export default function Navbar({ user, onLogout }) {
  const userLabel = user?.email || "Account";
  const [criticalAttnJobs, setCriticalAttnJobs] = useState(0);

  const handleLogoutClick = () => {
    if (onLogout) onLogout();
  };

  // Small badge on the "Jobs" link so anything late/never-seen at high or
  // critical severity is visible without opening the Jobs page. Polls
  // gently in the background — no need for anything fancier here.
  useEffect(() => {
    if (!user) {
      setCriticalAttnJobs(0);
      return;
    }
    let cancelled = false;
    const poll = () => {
      getJson("/api/overview")
        .then((data) => {
          if (!cancelled) setCriticalAttnJobs(data?.criticalAttnJobs || 0);
        })
        .catch(() => {
          // silent — a stale/missing badge isn't worth surfacing an error for
        });
    };
    poll();
    const id = setInterval(poll, 30000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [user]);

  return (
    <header class="border-b border-slate-200 bg-white">
      <div class="max-w-5xl mx-auto px-4 h-14 flex items-center justify-between gap-4">
        <div class="flex items-center gap-2 min-w-0">
          <div class="w-2 h-2 rounded-full bg-teal-500" />
          <Link
            href="/"
            class="font-semibold text-sm tracking-tight text-slate-900"
          >
            RunGuard
          </Link>
          <span class="hidden sm:inline text-[11px] px-2 py-0.5 rounded-full bg-blue-50 border border-blue-100 text-blue-700">
            cron & job reliability
          </span>
        </div>

        <nav class="flex items-center gap-4 text-sm">
          {!user && (
            <>
              <Link href="/docs" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Docs</Link>
              <Link href="/agent" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Agent</Link>
              <Link href="/login" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Login</Link>
              <Link href="/signup" class="px-3 py-1.5 rounded-full bg-blue-600 text-white text-sm font-medium hover:bg-blue-500 shadow-sm">Get started</Link>
            </>
          )}

          {user && (
            <>
              <Link href="/overview" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Overview</Link>
              <Link href="/jobs" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900 inline-flex items-center gap-1">
                Jobs
                {criticalAttnJobs > 0 && (
                  <span
                    class="inline-flex items-center justify-center min-w-[1.1rem] h-[1.1rem] px-1 rounded-full bg-red-600 text-white text-[10px] font-semibold leading-none"
                    title={`${criticalAttnJobs} high/critical severity job${criticalAttnJobs === 1 ? "" : "s"} need attention`}
                  >
                    {criticalAttnJobs}
                  </span>
                )}
              </Link>
              <Link href="/projects" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Projects</Link>
              <Link href="/incidents" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Incidents</Link>
              <Link href="/agent" activeClassName="text-slate-900 font-medium" class="text-slate-600 hover:text-slate-900">Agent</Link>

              <div class="relative group">
                <div class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-slate-300 text-xs text-slate-700 bg-white group-hover:bg-slate-100 cursor-default">
                  <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-slate-900 text-[10px] font-medium text-white">
                    {userLabel.charAt(0).toUpperCase()}
                  </span>
                  <span class="hidden sm:inline max-w-[10rem] truncate text-left">{userLabel}</span>
                  <svg class="h-3 w-3 text-slate-500" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                    <path d="M5.25 7.5L10 12.25L14.75 7.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                </div>

                <div class="absolute right-0 top-full w-44 rounded-xl border border-slate-200 bg-white shadow-lg py-1 text-xs z-20 opacity-0 invisible group-hover:opacity-100 group-hover:visible pointer-events-none group-hover:pointer-events-auto transition ease-out duration-100">
                  <Link href="/account" activeClassName="bg-slate-100 text-slate-900" class="block px-3 py-1.5 text-slate-700 hover:bg-slate-50">Account</Link>
                  <Link href="/webhooks" activeClassName="bg-slate-100 text-slate-900" class="block px-3 py-1.5 text-slate-700 hover:bg-slate-50">Webhooks</Link>
                  <Link href="/docs" activeClassName="bg-slate-100 text-slate-900" class="block px-3 py-1.5 text-slate-700 hover:bg-slate-50">Docs</Link>
                  {user.is_admin && (
                    <Link href="/admin" activeClassName="bg-slate-100 text-slate-900" class="block px-3 py-1.5 text-slate-700 hover:bg-slate-50">Admin</Link>
                  )}
                  <div class="border-t border-slate-200 mt-1 pt-1">
                    <button type="button" onClick={handleLogoutClick} class="w-full text-left px-3 py-1.5 text-slate-700 hover:bg-slate-50">
                      Logout
                    </button>
                  </div>
                </div>
              </div>
            </>
          )}
        </nav>
      </div>
    </header>
  );
}
