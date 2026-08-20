package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/marksisson/flare/internal/cloudflare"
	"github.com/marksisson/flare/internal/publicip"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "flare:", err)
		os.Exit(1)
	}
}

func run() error {
	zoneID := flag.String("zone-id", env("CLOUDFLARE_ZONE_ID", ""), "Cloudflare zone ID")
	recordID := flag.String("record-id", env("CLOUDFLARE_DNS_RECORD_ID", ""), "Cloudflare DNS record ID (optional when -name is set)")
	name := flag.String("name", env("FLARE_RECORD_NAME", ""), "fully qualified DNS record name")
	recordType := flag.String("type", env("FLARE_RECORD_TYPE", "A"), "DNS record type: A or AAAA")
	ipSource := flag.String("ip-source", env("FLARE_IP_SOURCE", ""), "public IP service URL (defaults to a Cloudflare trace endpoint)")
	timeout := flag.Duration("timeout", 20*time.Second, "overall request timeout")
	dryRun := flag.Bool("dry-run", false, "show the proposed change without updating Cloudflare")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flag.Args(), " "))
	}

	token, err := apiToken()
	if err != nil {
		return err
	}
	if strings.TrimSpace(*zoneID) == "" {
		return errors.New("zone ID is required; set CLOUDFLARE_ZONE_ID or use -zone-id")
	}
	*recordType = strings.ToUpper(strings.TrimSpace(*recordType))
	if *recordType != "A" && *recordType != "AAAA" {
		return fmt.Errorf("record type must be A or AAAA, not %q", *recordType)
	}
	if strings.TrimSpace(*ipSource) == "" {
		if *recordType == "AAAA" {
			*ipSource = "https://[2606:4700:4700::1111]/cdn-cgi/trace"
		} else {
			*ipSource = "https://1.1.1.1/cdn-cgi/trace"
		}
	}
	if strings.TrimSpace(*recordID) == "" && strings.TrimSpace(*name) == "" {
		return errors.New("record name is required when no record ID is supplied; set FLARE_RECORD_NAME or use -name")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	httpClient := &http.Client{}
	client := cloudflare.NewClient(httpClient, token)

	var record cloudflare.Record
	if strings.TrimSpace(*recordID) != "" {
		record, err = client.GetRecord(ctx, strings.TrimSpace(*zoneID), strings.TrimSpace(*recordID))
	} else {
		record, err = client.FindRecord(ctx, strings.TrimSpace(*zoneID), *recordType, strings.TrimSpace(*name))
	}
	if err != nil {
		return fmt.Errorf("resolve DNS record: %w", err)
	}
	if record.Type != *recordType {
		return fmt.Errorf("record %q has type %s, not requested type %s", record.Name, record.Type, *recordType)
	}
	if strings.TrimSpace(*name) != "" && !strings.EqualFold(record.Name, strings.TrimSuffix(strings.TrimSpace(*name), ".")) {
		return fmt.Errorf("record ID belongs to %q, not requested name %q", record.Name, *name)
	}

	address, err := publicip.Fetch(ctx, httpClient, *ipSource, *recordType)
	if err != nil {
		return err
	}
	if record.Content == address {
		fmt.Printf("%s %s is unchanged at %s\n", record.Type, record.Name, address)
		return nil
	}
	if *dryRun {
		fmt.Printf("would update %s %s from %s to %s\n", record.Type, record.Name, record.Content, address)
		return nil
	}

	previous := record.Content
	record.Content = address
	updated, err := client.UpdateRecord(ctx, strings.TrimSpace(*zoneID), record)
	if err != nil {
		return fmt.Errorf("update DNS record: %w", err)
	}
	fmt.Printf("updated %s %s from %s to %s\n", updated.Type, updated.Name, previous, updated.Content)
	return nil
}

func apiToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")); token != "" {
		return token, nil
	}
	if path := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN_FILE")); path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read CLOUDFLARE_API_TOKEN_FILE: %w", err)
		}
		if token := strings.TrimSpace(string(contents)); token != "" {
			return token, nil
		}
		return "", errors.New("CLOUDFLARE_API_TOKEN_FILE is empty")
	}
	return "", errors.New("API token is required; set CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE")
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
