package publicip

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Fetch retrieves and validates an address. The response may be either a plain
// address or Cloudflare's cdn-cgi/trace format.
func Fetch(ctx context.Context, client *http.Client, sourceURL, recordType string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("create public IP request: %w", err)
	}
	request.Header.Set("Accept", "text/plain")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch public IP: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("public IP service returned %s", response.Status)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read public IP response: %w", err)
	}
	candidate := parse(string(payload))
	address := net.ParseIP(candidate)
	if address == nil {
		return "", fmt.Errorf("public IP service returned an invalid address %q", candidate)
	}

	switch strings.ToUpper(recordType) {
	case "A":
		if address.To4() == nil {
			return "", fmt.Errorf("public IP service returned IPv6 address %q for an A record", candidate)
		}
		return address.To4().String(), nil
	case "AAAA":
		if address.To4() != nil {
			return "", fmt.Errorf("public IP service returned IPv4 address %q for an AAAA record", candidate)
		}
		return address.String(), nil
	default:
		return "", fmt.Errorf("unsupported DNS record type %q", recordType)
	}
}

func parse(payload string) string {
	trimmed := strings.TrimSpace(payload)
	for _, line := range strings.Split(trimmed, "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "ip="); found {
			return strings.TrimSpace(value)
		}
	}
	return trimmed
}
