# Nexus CLI Testing Guide

This guide helps you validate the Nexus CLI locally with real modules.

## Prerequisites

- Go 1.25+ installed
- Kubernetes access (for `kubernetes` module)
- Prometheus reachable (for `prometheus` module)

## 1) Run Tests

```bash
go test ./...
```

## 2) Build the Binary

```bash
go build -o nexus ./cmd/nexus
```

## 3) Verify Configuration

Default config is `nexus.yaml` in the repo root.

Key fields:

- `server.log_level`
- `server.safe_mode`
- `modules.kubernetes.enabled`
- `modules.prometheus.enabled`
- `modules.prometheus.url`

## 4) Start Nexus (stdio)

```bash
./nexus --config nexus.yaml
```

If config is missing, Nexus falls back to safe defaults.

## 4.1) Start Nexus (SSE)

```bash
./nexus --transport sse --http-addr :8080 --base-url http://localhost:8080 --base-path /mcp
```

SSE endpoints:

- `GET /mcp/sse` (SSE connection)
- `POST /mcp/message?sessionId=...` (JSON-RPC messages)
- `GET /tools` (tool inventory)
- `GET /healthz` (health check)

## 5) Enable/Disable Modules

Edit `nexus.yaml`:

```yaml
modules:
  kubernetes:
    enabled: true
  prometheus:
    enabled: true
```

Then restart Nexus.

## 6) Test Kubernetes Tool

Using an MCP client, call:

- Tool: `k8s_list_pods`
- Arguments:
  - `namespace`: e.g., `default`

Expected: a list of pods or a “no pods found” response.

## 6.0) List Namespaces

- Tool: `k8s_list_namespaces`
- Arguments:
  - `max_namespaces`: optional (default 200, max 1000)

## 6.0.1) List Pods Across All Namespaces

- Tool: `k8s_list_pods_all`
- Arguments:
  - `error_only`: optional (default true)
  - `max_pods`: optional (default 200, max 1000)

## 6.1) Test Kubernetes Logs

Use `k8s_get_logs` for pods or workload logs:

- Tool: `k8s_get_logs`
- Arguments:
  - `namespace`: e.g., `default`
  - `kind`: `pod` | `deployment` | `daemonset` | `statefulset` | `job`
  - `name`: resource name
  - `container`: optional
  - `tail_lines`: optional (default 200, max 500)
  - `since_seconds`: optional
  - `previous`: optional (true to read previous crash logs)
  - `contains`: optional (filters lines by substring)
  - `error_only`: optional (default true)
  - `event_limit`: optional (default 5, max 20)

Example (deployment):

```json
{
  "name": "k8s_get_logs",
  "arguments": {
    "namespace": "default",
    "kind": "deployment",
    "name": "api",
    "tail_lines": 200,
    "error_only": true,
    "contains": "error"
  }
}
```

## 7) Test Prometheus Tool

Ensure `modules.prometheus.url` points to your Prometheus base URL, e.g.:

```yaml
modules:
  prometheus:
    enabled: true
    url: "http://localhost:9090"
```

Using an MCP client, call:

- Tool: `prometheus_query_metric`
- Arguments:
  - `query`: e.g., `up`

Expected: live Prometheus response (no mock data).

## 7.1) Test Logs Module (Local Files)

Enable `logs` in `nexus.yaml`, then set an allowlist:

```yaml
modules:
  logs:
    enabled: true
    allow_paths:
      - "/var/log"
      - "./logs"
```

Using an MCP client, call:

- Tool: `logs_tail`
- Arguments:
  - `path`: `/var/log/system.log`
  - `tail_lines`: `200`
  - `error_only`: `true`
  - `contains`: `error`

## 7.2) Test Docker Module

Enable `docker` in `nexus.yaml`:

```yaml
modules:
  docker:
    enabled: true
    cli: "docker"
```

Using an MCP client, call:

- Tool: `docker_list_containers`
- Arguments:
  - `all`: `false`

For logs:

- Tool: `docker_get_logs`
- Arguments:
  - `id`: `container_id_or_name`
  - `tail_lines`: `200`

## 8) Safe Mode

Run Nexus in read-only safe mode:

```bash
./nexus --safe-mode
```

Safe mode should be used for production demos and untrusted agents.

## 9) Debugging

- Logs go to `stderr` to avoid corrupting JSON-RPC `stdout`.
- Increase verbosity with:

```yaml
server:
  log_level: "debug"
```

## 10) Common Issues

- **No tools show up**: ensure modules are enabled in `nexus.yaml`.
- **Kubernetes auth errors**: verify kubeconfig and access.
- **Prometheus errors**: confirm URL and network reachability.
- **SSE connects but no messages**: ensure `base-url` matches the client origin.

## 11) Plugin Bundles (Day 5 MVP)

Plugin bundles live in `modules.plugins.dir` (default `./plugins`). Each plugin is a folder that contains a `nexus.yaml` manifest.

Example `nexus.yaml` (plugin manifest):

```yaml
apiVersion: nexus/v1alpha1
kind: ContextServer
metadata:
  name: "example"
  vendor: "edgeops-labs"
  version: "v0.1.0"
  description: "Example plugin"
spec:
  command: "./bin/example-plugin"
  args: []
  env:
    LOG_LEVEL: "info"
  capabilities:
    tools:
      - name: "status"
        description: "Return a status string"
        read_only: true
        args:
          - name: "detail"
            type: "string"
            required: false
            description: "Extra detail"
```

Install a plugin bundle:

```bash
./nexus install ./path/to/plugin-bundle --plugins-dir ./plugins
```

Tool names are exposed as:

```text
plugin/<plugin-name>/<tool-name>
```

**Security note:** keep secrets out of manifests. Use environment variables or external secret managers.

## 12) Installation Channels (Public)

macOS (Homebrew tap):

```bash
brew tap edgeopslabs/nexus
brew install edgeopslabs/nexus/nexus-cli
```

Linux (Snap):

```bash
sudo snap install nexus --classic
```

Linux (APT repo):

```bash
curl -fsSL https://YOUR_DOMAIN/KEY.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/nexus.gpg
echo "deb [signed-by=/etc/apt/keyrings/nexus.gpg] https://YOUR_DOMAIN/apt stable main" | sudo tee /etc/apt/sources.list.d/nexus.list
sudo apt update
sudo apt install nexus
```

Windows (Winget):

```powershell
winget install EdgeOpsLabs.Nexus
```

Linux (AppImage):

```bash
curl -fsSL https://github.com/edgeopslabs/nexus-core/releases/download/v0.0.1/nexus_linux_amd64.AppImage -o nexus.AppImage
chmod +x nexus.AppImage
./nexus.AppImage
```

Linux (RPM):

```bash
sudo rpm -i nexus_0.0.1_amd64.rpm
```

## 13) Release Automation

Releases are created on tag pushes (e.g. `v0.0.1`) using GoReleaser.

```bash
git tag v0.0.1
git push origin v0.0.1
```

Artifacts are attached to the GitHub release with `checksums.txt`.
AppImages are built in the release workflow and uploaded to the same release.
