import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import Router, { route } from "preact-router";
import { Link } from "preact-router/match";
import { me, logout } from "./api.js";

import Navbar from "./components/NavBar.jsx";
import { ToastHost } from "./components/Toast.jsx";
import LandingPage from "./pages/LandingPage.jsx";
import LoginPage from "./pages/LoginPage.jsx";
import SignupPage from "./pages/SignupPage.jsx";
import ProfilePage from "./pages/ProfilePage.jsx";
import MagicPage from "./pages/MagicPage.jsx";
import VerifyEmailPage from "./pages/VerifyEmailPage.jsx";
import ResetRequestPage from "./pages/ResetRequestPage.jsx";
import ResetPage from "./pages/ResetPage.jsx";
import JobsPage from "./pages/JobsPage.jsx";
import OverviewPage from "./pages/OverViewPage.jsx";
import PublicJobPage from "./pages/PublicJobPage.jsx";
import AccountPage from "./pages/AccountPage.jsx";
import WebhooksPage from "./pages/WebHooksPage.jsx";
import AgentPage from "./pages/AgentPage.jsx";
import ProjectsPage from "./pages/ProjectsPage.jsx";
import IncidentsPage from "./pages/IncidentsPage.jsx";
import DocsPage from "./pages/DocsPage.jsx";
import SecurityPage from "./pages/SecurityPage.jsx";
import PrivacyPage from "./pages/PrivacyPage.jsx";
import TermsPage from "./pages/TermsPage.jsx";
import AdminPage from "./pages/AdminPage.jsx";
import StatusPage from "./pages/StatusPage.jsx";

export default function App() {
  const [user, setUser] = useState(null);
  const [authChecked, setAuthChecked] = useState(false);

  useEffect(() => {
    me().then((res) => setUser(res.user)).catch(() => setUser(null)).finally(() => setAuthChecked(true));
  }, []);

  const handleAuth = (u) => { setUser(u); route("/overview", true); };
  const handleLogout = async () => { try { await logout(); } finally { setUser(null); route("/", true); } };

  if (!authChecked) {
    return <div class="min-h-screen flex items-center justify-center bg-slate-50 text-slate-700"><div class="text-sm">Loading…</div></div>;
  }

  return (
    <div class="min-h-screen bg-slate-50 text-slate-800 flex flex-col">
      <ToastHost />
      <Navbar user={user} onLogout={handleLogout} />
      <main class="flex-1">
        <Router>
          <LandingPage path="/" user={user} />
          <LoginPage path="/login" onLogin={handleAuth} user={user} />
          <SignupPage path="/signup" onSignup={handleAuth} user={user} />
          <ProfilePage path="/profile" user={user} onLogout={handleLogout} />
          <MagicPage path="/magic" onLogin={handleAuth} />
          <VerifyEmailPage path="/verify-email" user={user} />
          <ResetRequestPage path="/reset-request" user={user} />
          <ResetPage path="/reset" onLogin={handleAuth} />
          <JobsPage path="/jobs" user={user} />
          <OverviewPage path="/overview" user={user} />
          <ProjectsPage path="/projects" user={user} />
          <IncidentsPage path="/incidents" user={user} />
          <DocsPage path="/docs" />
          <SecurityPage path="/security" />
          <PrivacyPage path="/privacy" />
          <TermsPage path="/terms" />
          <PublicJobPage path="/p/:token" />
          <AccountPage path="/account" user={user} onUserUpdate={setUser} onLogout={handleLogout} />
          <WebhooksPage path="/webhooks" user={user} />
          <AgentPage path="/agent" />
          <AdminPage path="/admin" user={user} />
          <StatusPage path="/status" />
        </Router>
      </main>
      <footer class="border-t border-slate-200 text-xs text-slate-500 py-4 text-center bg-white">
        <div class="max-w-5xl mx-auto px-4 flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
          <span>RunGuard</span>
          <span class="text-slate-300">·</span>
          <span>quietly making sure your jobs actually run</span>
          <span class="text-slate-300">·</span>
          <Link href="/docs" class="hover:text-slate-900">Docs</Link>
          <Link href="/security" class="hover:text-slate-900">Security</Link>
          <Link href="/privacy" class="hover:text-slate-900">Privacy</Link>
          <Link href="/terms" class="hover:text-slate-900">Terms</Link>
          <Link href="/status" class="hover:text-slate-900">Status</Link>
        </div>
      </footer>
    </div>
  );
}
