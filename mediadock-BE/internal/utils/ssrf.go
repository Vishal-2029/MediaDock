package utils

import (
	"context"
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

var (
	// ErrInvalidURL indicates a syntactically invalid or disallowed URL.
	ErrInvalidURL = errors.New("invalid or disallowed URL")
	// ErrSSRFBlocked indicates the URL targets a restricted network or host.
	ErrSSRFBlocked = errors.New("target host or IP address is blocked by security policy")
)

var blockedHostnames = map[string]struct{}{
	"localhost":                {},
	"localhost.localdomain":    {},
	"ip6-localhost":            {},
	"ip6-loopback":             {},
	"metadata.google.internal": {},
	"metadata":                 {},
	"169.254.169.254":          {},
	"100.100.100.200":          {},
}

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// ValidatePublicURL parses rawURL, enforces HTTP/HTTPS scheme, rejects embedded credentials,
// and verifies that the host is not a local, private, link-local, or cloud metadata target.
// If resolveDNS is true, DNS lookup is performed and all resolved IPs are checked.
func ValidatePublicURL(ctx context.Context, rawURL string, resolveDNS bool) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || len(trimmed) > 2048 {
		return nil, fmt.Errorf("%w: URL length out of bounds", ErrInvalidURL)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%w: unsupported scheme %q", ErrInvalidURL, parsed.Scheme)
	}

	if parsed.User != nil {
		return nil, fmt.Errorf("%w: embedded URL credentials are not permitted", ErrInvalidURL)
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return nil, fmt.Errorf("%w: missing hostname", ErrInvalidURL)
	}

	if isBlockedHostname(host) {
		return nil, fmt.Errorf("%w: hostname %q is restricted", ErrSSRFBlocked, host)
	}

	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		if !IsPublicIP(ip) {
			return nil, fmt.Errorf("%w: IP %s is not publicly routable", ErrSSRFBlocked, ip)
		}
		return parsed, nil
	}

	if resolveDNS {
		if err := ValidateHostDNS(ctx, host); err != nil {
			return nil, err
		}
	}

	return parsed, nil
}

// ValidateHostDNS resolves host and verifies all returned IP addresses are public.
func ValidateHostDNS(ctx context.Context, host string) error {
	if isBlockedHostname(strings.ToLower(strings.TrimSuffix(host, "."))) {
		return fmt.Errorf("%w: hostname %q is restricted", ErrSSRFBlocked, host)
	}

	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: DNS resolution failed for %q", ErrInvalidURL, host)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: no IP addresses resolved for %q", ErrInvalidURL, host)
	}
	for _, ip := range ips {
		if !IsPublicIP(ip) {
			return fmt.Errorf("%w: resolved IP %s is restricted", ErrSSRFBlocked, ip)
		}
	}
	return nil
}

// IsPublicIP returns true only if the IP address is globally routable unicast
// and not private, loopback, link-local, CGNAT, multicast, or unspecified.
func IsPublicIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return false
	}
	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return false
	}
	if addr.Is4() && cgnatPrefix.Contains(addr) {
		return false
	}
	// Block IPv4-compatible / 6to4 / documentation / reserved ranges via net.IP checks if needed.
	ip := net.IP(addr.AsSlice())
	if ip == nil {
		return false
	}
	return addr.IsGlobalUnicast()
}

func isBlockedHostname(host string) bool {
	if _, blocked := blockedHostnames[host]; blocked {
		return true
	}
	suffixes := []string{".localhost", ".local", ".internal", ".arpa", ".home"}
	for _, s := range suffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

// NewSafeHTTPClient builds an http.Client that protects against SSRF at both
// TCP dial time (preventing DNS rebinding) and HTTP redirect hops.
func NewSafeHTTPClient(timeout time.Duration, maxRedirects int) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if maxRedirects <= 0 {
		maxRedirects = 5
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: malformed target address", ErrSSRFBlocked)
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return fmt.Errorf("%w: invalid target IP", ErrSSRFBlocked)
			}
			if !IsPublicIP(ip) {
				return fmt.Errorf("%w: blocked connection to non-public IP %s", ErrSSRFBlocked, ip)
			}
			return nil
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("%w: exceeded maximum redirects (%d)", ErrSSRFBlocked, maxRedirects)
			}
			if _, err := ValidatePublicURL(req.Context(), req.URL.String(), true); err != nil {
				return err
			}
			return nil
		},
	}
}

