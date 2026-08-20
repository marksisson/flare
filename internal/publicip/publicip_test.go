package publicip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchCloudflareTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("fl=123\nip=192.0.2.44\nts=12345\n"))
	}))
	defer server.Close()

	address, err := Fetch(context.Background(), server.Client(), server.URL, "A")
	if err != nil {
		t.Fatal(err)
	}
	if address != "192.0.2.44" {
		t.Fatalf("address = %q", address)
	}
}

func TestFetchPlainIPv6(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("2001:db8::1\n"))
	}))
	defer server.Close()

	address, err := Fetch(context.Background(), server.Client(), server.URL, "AAAA")
	if err != nil {
		t.Fatal(err)
	}
	if address != "2001:db8::1" {
		t.Fatalf("address = %q", address)
	}
}

func TestFetchRejectsWrongFamily(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("2001:db8::1"))
	}))
	defer server.Close()

	_, err := Fetch(context.Background(), server.Client(), server.URL, "A")
	if err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Fatalf("unexpected error: %v", err)
	}
}
