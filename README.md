# Flare

Flare is a small, dependency-free Go utility that updates a Cloudflare `A` or `AAAA` DNS record with the Mac's current public IP address. It uses Go's `net/http`; it does not invoke `curl`.

Flare reads the current record before updating it, preserves its mutable settings, and skips the update when the address has not changed. It implements Cloudflare's [`PUT /zones/{zone_id}/dns_records/{dns_record_id}` update operation](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/update/).

## Cloudflare setup

Create an API token with **Zone / DNS / Edit** permission for the relevant zone. Flare needs:

- the API token;
- the zone ID; and
- either the complete record name or its DNS record ID.

When only a name is supplied, Flare finds the matching record through the DNS Records API. The token should be scoped to only the zone Flare updates.

## Run

```console
export CLOUDFLARE_API_TOKEN='…'
export CLOUDFLARE_ZONE_ID='…'
export FLARE_RECORD_NAME='home.example.com'

nix run . -- -dry-run
nix run .
```

For an `AAAA` record:

```console
export FLARE_RECORD_TYPE=AAAA
nix run .
```

Supported configuration:

| Environment variable | Flag | Meaning |
| --- | --- | --- |
| `CLOUDFLARE_API_TOKEN` | — | Cloudflare API token |
| `CLOUDFLARE_API_TOKEN_FILE` | — | File containing the token; used when the direct variable is unset |
| `CLOUDFLARE_ZONE_ID` | `-zone-id` | Cloudflare zone ID |
| `CLOUDFLARE_DNS_RECORD_ID` | `-record-id` | DNS record ID; optional when a name is set |
| `FLARE_RECORD_NAME` | `-name` | Complete DNS record name |
| `FLARE_RECORD_TYPE` | `-type` | `A` (default) or `AAAA` |
| `FLARE_IP_SOURCE` | `-ip-source` | Plain-IP or Cloudflare trace endpoint |

By default, Flare uses Cloudflare's trace endpoint through `1.1.1.1` for an `A` record or `2606:4700:4700::1111` for an `AAAA` record. The explicit addresses ensure the discovered address has the requested family. Run `nix run . -- -help` for timeout and dry-run options.

## Build and test

```console
nix flake check
nix build
```

Install the utility into the current Nix profile with:

```console
nix profile install .
```

## Run periodically on macOS

The example LaunchAgent runs Flare at login and every five minutes. Store the token in a mode-`0600` file rather than directly in the plist:

```console
mkdir -p ~/.config/flare ~/Library/LaunchAgents
printf '%s\n' 'YOUR_API_TOKEN' > ~/.config/flare/cloudflare-api-token
chmod 600 ~/.config/flare/cloudflare-api-token
cp launchd/com.marksisson.flare.plist.example \
  ~/Library/LaunchAgents/com.marksisson.flare.plist
```

Edit the copied plist to replace `YOU`, the zone ID, and the record name, then load it:

```console
launchctl bootstrap gui/"$(id -u)" \
  ~/Library/LaunchAgents/com.marksisson.flare.plist
```
