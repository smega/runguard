// webhook_http.go
//
// Shared, SSRF-safe HTTP client used for every outbound webhook delivery
// (alert webhooks and the "send test webhook" button). Job webhook URLs are
// supplied by users, so without this a job could be configured to point at
// internal infrastructure (e.g. 10.0.0.0/8, 169.254.169.254 cloud metadata,
// localhost services) and the backend would happily make that request on
// the user's behalf.
//
// Defense happens at the actual dial, not just at URL-save time: the
// resolved IP is checked immediately before connecting, and the connection
// is made to that exact IP (not re-resolved), which also closes the classic
// DNS-rebinding gap (URL resolves to a safe IP at validation time, then to
// an internal IP at request time).
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// validateWebhookURLScheme does a cheap, synchronous check at the point a
// user saves a webhook URL (job alert URL or webhook template), so they get
// immediate feedback instead of a silently-failing background alert later.
// It intentionally does NOT do DNS resolution here (that's enforced at send
// time in safeWebhookDialContext, where it actually matters).
func validateWebhookURLScheme(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("webhook url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook url must be http or https")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("webhook url must include a host")
	}
	return nil
}

// isDisallowedWebhookIP blocks loopback, private, link-local (which covers
// the 169.254.169.254 cloud metadata address), and other non-routable
// ranges that a public webhook has no legitimate reason to target.
func isDisallowedWebhookIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

func safeWebhookDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("split host/port: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for host %q", host)
	}
	for _, ip := range ips {
		if isDisallowedWebhookIP(ip) {
			return nil, fmt.Errorf("refusing to send webhook to disallowed address %s", ip)
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	var lastErr error
	for _, ip := range ips {
		// Dial the already-validated IP directly, not the original host,
		// so a second DNS lookup at connect time can't return a different
		// (unsafe) address.
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}

// webhookHTTPClient is the only HTTP client that should be used to deliver
// user-configured webhook URLs.
var webhookHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext:         safeWebhookDialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	},
	// Don't follow redirects blindly — a redirect to an internal address
	// would otherwise bypass the checks above on the *next* hop only via
	// DialContext (which does still apply, so this is belt-and-suspenders,
	// but capping redirects also avoids surprising behavior).
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}
