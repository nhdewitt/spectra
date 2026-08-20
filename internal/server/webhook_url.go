package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// maxWebhookRedirects caps redirect following. Each hop is re-checked by the
// dialer's Control hook below, so this is a loop guard, not the security
// boundary.
const maxWebhookRedirects = 3

// errWebhookTargetBlocked is returned by the webhook dialer's Control hook when
// a connection would land on an address alerting is not allowed to reach.
var errWebhookTargetBlocked = errors.New("webhook target address is not permitted")

// webhookAddrAllowed reports whether alerting may open a connection to ip.
//
// RFC1918 and ULA addresses are deliberately allowed: Spectra is deployed on a LAN
// and the realistic webhook targets live there. Blocking them would break the primary
// deployment to buy very little, since only admins can create channels.
//
// What is blocked is the set of addresses that are never a legitimate webhook
// target: loopback, link-local, the unspecified address, multicast, and the IPv4
// broadcast address.
func webhookAddrAllowed(ip netip.Addr) bool {
	// Unmap first: ::ffff:127.0.0.1 must not read as "not loopback".
	ip = ip.Unmap()
	if !ip.IsValid() {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	if ip.Is4() && ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return false
	}
	return true
}

// validateWebhookURL is the create/update-time check: it rejects URLs that are
// obviously unusable or obviously hostile so an admin gets a 400 immediately
// rather than a silent notification failure later.
//
// It is not the security boundary. A hostname resolving to a blocked address
// (including DNS rebinding) is caught by newWebhookClient's Control hook, which
// sees the concrete IP being dialed.
func validateWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("webhook url is not a valid URL: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("webhook url scheme %q is not permitted (want http or https)", u.Scheme)
	}

	if u.User != nil {
		return errors.New("webhook url must not embed credentials")
	}

	host := u.Hostname()
	if host == "" {
		return errors.New("webhook url must include a host")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !webhookAddrAllowed(ip) {
		return fmt.Errorf("webhook url host %s is not a permitted target", host)
	}
	return nil
}

// newWebhookClient builds the HTTP client used for outbound alert webhooks.
//
// The Control hook runs after DNS resolution with the concrete addresds about to
// be dialed, so it covers hostnames, redirect targets, and DNS rebinding in one
// place; checking the URL string alone would miss all three.
//
// Proxy is deliberately left nil: a proxy would terminate the connection at the
// proxy's address, making every Control check pass regardless of the real target.
func newWebhookClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: unparseable address %q", errWebhookTargetBlocked, address)
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return fmt.Errorf("%w: unparseable address %q", errWebhookTargetBlocked, host)
			}
			if !webhookAddrAllowed(ip) {
				return fmt.Errorf("%w: %s", errWebhookTargetBlocked, ip)
			}
			return nil
		},
	}

	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          4,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       60 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxWebhookRedirects {
				return fmt.Errorf("stopped after %d redirects", maxWebhookRedirects)
			}
			switch req.URL.Scheme {
			case "http", "https":
				return nil
			default:
				return fmt.Errorf("redirect to scheme %q is not permitted", req.URL.Scheme)
			}
		},
	}
}
