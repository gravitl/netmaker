# nmctl

Command-line interface for the Netmaker server API.

Configuration is stored in `~/.netmaker/config.yml`. Each **context** holds connection settings for one server or SaaS tenant. Use `nmctl context` to manage contexts, then run commands against the active context.

```bash
nmctl context list
nmctl context use <name>
```

Most list commands support JSON output: `-o json`.

---

## Self-hosted (single tenant)

Use this for a typical self-hosted install with one default organization and tenant. You do **not** need `--tenant_id` or `--org_id`; the server resolves the sole tenant automatically.

### Master key (automation / install scripts)

```bash
nmctl context set default \
  --endpoint=https://api.example.com \
  --master_key=<server-master-key>

nmctl context use default
nmctl network list
```

### Username and password

```bash
nmctl context set default \
  --endpoint=https://api.example.com \
  --username=admin \
  --password='your-password'

nmctl context use default
```

### SSO

```bash
nmctl context set default \
  --endpoint=https://api.example.com \
  --sso

nmctl context use default
```

Follow the URL printed in the terminal to complete authentication.

---

## SaaS (Netmaker Cloud)

SaaS contexts use a tenant-specific API hostname and Netmaker Accounts for login. `--tenant_id` is **required** with `--saas`.

### Username and password

```bash
nmctl context set my-tenant \
  --saas \
  --tenant_id=<tenant-uuid> \
  --username=you@example.com \
  --password='your-password'

nmctl context use my-tenant
nmctl network list
```

The API endpoint is set automatically to `https://api-<tenant_id>.app.prod.netmaker.io`.

### SSO

```bash
nmctl context set my-tenant \
  --saas \
  --tenant_id=<tenant-uuid> \
  --sso

nmctl context use my-tenant
```

### Auth token

If you already have a JWT:

```bash
nmctl context set my-tenant \
  --saas \
  --tenant_id=<tenant-uuid> \
  --auth_token=<jwt>
```

---

## Self-hosted multi-tenancy

When the server runs multiple organizations or tenants, scope must be sent on each request:

| Context field | HTTP header |
|---------------|-------------|
| `--tenant_id` | `X-Tenant-ID` |
| `--org_id`    | `X-Organization-ID` |

Set these on the context; nmctl attaches the headers to every API call.

### Discover organization and tenant IDs

```bash
# List orgs (global scope; master key or super-admin)
nmctl context set admin \
  --endpoint=https://api.example.com \
  --master_key=<key>

nmctl organization list
# or: nmctl organization list -o json

# List tenants in an org (requires org_id on the context)
nmctl context set admin \
  --endpoint=https://api.example.com \
  --master_key=<key> \
  --org_id=<organization-uuid>

nmctl tenant list
# or: nmctl tenant list -o json
```

### Work in a specific tenant

```bash
nmctl context set prod \
  --endpoint=https://api.example.com \
  --username=admin \
  --password='your-password' \
  --org_id=<organization-uuid> \
  --tenant_id=<tenant-uuid>

nmctl context use prod
nmctl network list
nmctl node list
```

You can combine `--master_key` instead of username/password for scripts.

### Notes

- **Tenant-scoped** commands (networks, nodes, hosts, enrollment keys, etc.) require `--tenant_id` when more than one tenant exists.
- **Org-scoped** commands (e.g. `nmctl tenant list`) require `--org_id`.
- If neither header is set on a single-tenant server, the API still works via the default tenant fallback.
- SaaS contexts also send `X-Tenant-ID` when `--tenant_id` is set.

---

## Context reference

```bash
nmctl context set <name> [flags]
nmctl context use <name>
nmctl context list
nmctl context delete <name>
```

| Flag | Self-hosted | SaaS | Multi-tenant self-hosted |
|------|-------------|------|---------------------------|
| `--endpoint` | Required | Auto-set from tenant ID | Required |
| `--master_key` | Optional | — | Optional |
| `--username` / `--password` | Optional | Required (or token/SSO) | Optional |
| `--auth_token` | Optional | Optional | Optional |
| `--sso` | Optional | Optional | Optional |
| `--saas` | — | Required | — |
| `--tenant_id` | Optional | Required | Required (multiple tenants) |
| `--org_id` | Optional | Optional | Required for org-scoped APIs |

---

## Common commands

```bash
nmctl network list
nmctl network create --name mynet --ipv4_addr 10.10.0.0/16

nmctl node list
nmctl node list <network>

nmctl host list

nmctl user list
nmctl enrollment_key list

nmctl organization list    # list organizations
nmctl tenant list          # list tenants (set --org_id on context first)

nmctl server health
nmctl server info
```

Run `nmctl --help` or `nmctl <command> --help` for full command trees.

---

## Build from source

From the repository root:

```bash
go build -o nmctl ./cli
```

Release binaries: [Netmaker releases](https://github.com/gravitl/netmaker/releases) (`nmctl-linux-*`, `nmctl-darwin-*`, etc.).
