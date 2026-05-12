package publicip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

var endpoints = []string{
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
}

func ResolveCIDR(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 128))
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s returned HTTP %d", endpoint, resp.StatusCode)
			continue
		}
		cidr, err := CIDRFromIP(strings.TrimSpace(string(body)))
		if err != nil {
			lastErr = err
			continue
		}
		return cidr, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no public IP resolver endpoints configured")
	}
	return "", fmt.Errorf("failed to resolve public IP: %w", lastErr)
}

func CIDRFromIP(value string) (string, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("invalid public IP %q: %w", value, err)
	}
	if addr.Is4() {
		return addr.String() + "/32", nil
	}
	return addr.String() + "/128", nil
}
