# inventory-wg-sync

Sync WireGuard `AllowedIPs` from Ansible inventory files and keep a WireGuard profile updated.

## What it does
- Reads one or more Ansible inventory files (hosts format).
- Collects host IPs, CIDRs, and hostnames (A/AAAA/CNAME).
- Builds a unique, sorted list of CIDRs (IPv4 as /32, IPv6 as /128).
- Updates a WireGuard profile with `AllowedIPs`, `Table`, `PostUp`, `PostDown`.
- Restarts the `wg-quick@<name>` systemd unit to apply changes.

## Requirements
- Linux with `wg` and `wg-quick` (systemd service `wg-quick@`).
- Root access to write `/etc/wireguard/*.conf` and manage systemd.
- Go 1.21+ if you plan to build from source.

## Install
```bash
go install ./cmd/inventory-wg-sync
```

Or build a static binary:
```bash
go build -o inventory-wg-sync ./cmd/inventory-wg-sync
```

## Configuration
Place the config at:
- `$XDG_CONFIG_HOME/inventory-wg-sync.yml`, or
- any path in `$XDG_CONFIG_DIRS`

Start by copying `config.yml.sample`:
```bash
cp config.yml.sample /root/.config/inventory-wg-sync.yml
```

Example config:
```yaml
inventory_paths:
  - /etc/ansible/hosts

profile_path: /etc/wireguard/wg0.conf

allowed_ips:
  - 10.0.0.0/8
  - example.com

excluded_ips:
  - 10.10.0.0/16

table: 1234

post_up:
  - ip rule add from 10.0.0.0/8 table {{ .table }}

post_down:
  - ip rule del from 10.0.0.0/8 table {{ .table }}

debug: false
```

### Config fields
- `inventory_paths`: list of Ansible inventory files (hosts format).
- `profile_path`: WireGuard profile to update (`/etc/wireguard/wg0.conf`). If empty, no profile updates occur.
- `allowed_ips`: extra IPs/CIDRs/hostnames to always include.
- `excluded_ips`: IPs/CIDRs/hostnames to always exclude.
- `table`: optional routing table number; updates `Table =` in the profile.
- `post_up` / `post_down`: optional commands; supports `{{ .name }}` and `{{ .table }}`.
- `debug`: enable verbose logging.

## How host entries are resolved
- IPs: turned into `/32` (IPv4) or `/128` (IPv6).
- CIDRs: used as-is.
- Hostnames: resolved via A/AAAA records; CNAMEs are followed.

If the WireGuard profile `Address =` line lacks IPv4 or IPv6, unsupported `AllowedIPs` are filtered out.

## Usage
```bash
sudo inventory-wg-sync
```

If the interface is not up yet, the tool starts `wg-quick@<name>`. Otherwise it restarts the service.

## Notes
- The WireGuard profile file is written with `0600` permissions.
- Run as root to update the profile and manage systemd.
- Ansible is a trademark of Red Hat, Inc. This project is not affiliated with, endorsed by, or sponsored by Red Hat or the Ansible project.
