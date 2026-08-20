package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindAndUpdateRecord(t *testing.T) {
	proxied := true
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")

		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/zones/zone/dns_records":
			if request.URL.Query().Get("type") != "A" || request.URL.Query().Get("name") != "home.example.com" {
				t.Errorf("unexpected query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"success":true,"errors":[],"result":[{"id":"record","type":"A","name":"home.example.com","content":"192.0.2.1","ttl":1,"proxied":true,"comment":"dynamic"}]}`))
		case request.Method == http.MethodPut && request.URL.Path == "/zones/zone/dns_records/record":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if _, found := body["id"]; found {
				t.Error("update body must not contain the immutable record ID")
			}
			if body["content"] != "192.0.2.2" || body["comment"] != "dynamic" || body["proxied"] != proxied {
				t.Errorf("update body did not preserve fields: %#v", body)
			}
			_, _ = writer.Write([]byte(`{"success":true,"errors":[],"result":{"id":"record","type":"A","name":"home.example.com","content":"192.0.2.2","ttl":1,"proxied":true,"comment":"dynamic"}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client(), "secret")
	client.BaseURL = server.URL
	record, err := client.FindRecord(context.Background(), "zone", "A", "home.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "record" || record.Proxied == nil || *record.Proxied != proxied {
		t.Fatalf("unexpected record: %#v", record)
	}

	record.Content = "192.0.2.2"
	updated, err := client.UpdateRecord(context.Background(), "zone", record)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "192.0.2.2" {
		t.Fatalf("updated content = %q", updated.Content)
	}
}

func TestCloudflareError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), "bad")
	client.BaseURL = server.URL
	_, err := client.GetRecord(context.Background(), "zone", "record")
	if err == nil || err.Error() != "Cloudflare API returned 403 Forbidden: 9109: Invalid access token" {
		t.Fatalf("unexpected error: %v", err)
	}
}
