// siteConfig.js — single place for values that are specific to *your*
// deployment (domain, download host, contact addresses). Every page should
// import from here instead of hardcoding a literal, so a self-hoster only
// has to fill in frontend/.env (see .env.example) once.
//
// These fallbacks are deliberately obvious placeholders, not a real
// working default — see the equivalent comment on defaultAPIBase in
// backend/cmd/runguard/main.go for why.

export const PUBLIC_BASE_URL =
  import.meta.env.VITE_PUBLIC_BASE_URL || "https://runguard.example.com";

// Separate subdomain/host that serves public read-only job-status pages —
// distinct from PUBLIC_BASE_URL (the app itself).
export const PUBLIC_STATUS_BASE_URL =
  import.meta.env.VITE_PUBLIC_STATUS_BASE_URL || "https://runguard-status.example.com";

export const CLI_DOWNLOAD_BASE =
  import.meta.env.VITE_CLI_DOWNLOAD_BASE || "https://runguard-downloads.example.com";

export const INSTALL_SCRIPT_URL =
  import.meta.env.VITE_INSTALL_SCRIPT_URL || `${PUBLIC_BASE_URL}/install.sh`;

export const PRIVACY_EMAIL = import.meta.env.VITE_PRIVACY_EMAIL || "privacy@example.com";
export const LEGAL_EMAIL = import.meta.env.VITE_LEGAL_EMAIL || "legal@example.com";
export const SECURITY_EMAIL = import.meta.env.VITE_SECURITY_EMAIL || "security@example.com";
