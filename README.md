# Nexus

[![Release](https://img.shields.io/github/v/release/EdgeOpslabs/nexus?display_name=tag&sort=semver)](https://github.com/EdgeOpslabs/nexus/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/EdgeOpslabs/nexus/ci.yml?branch=main)](https://github.com/EdgeOpslabs/nexus/actions)
[![License](https://img.shields.io/github/license/EdgeOpslabs/nexus)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/EdgeOpslabs/nexus)](go.mod)

Nexus is the unified bridge between AI agents and real infrastructure context. It runs as a single binary inside your environment (laptop or cluster) and exposes safe, read‑only or policy‑governed capabilities over MCP so agents can troubleshoot and operate without manual log dumps.

Built with love by [@iemafzalhassan](https://github.com/iemafzalhassan).

## Why Nexus

AI models are smart, but they lack live context. Nexus closes that gap by providing:

- Safe, read‑only access to Kubernetes, Prometheus, Docker, and logs
- Policy enforcement for allow/deny/confirm flows
- Token‑efficient log filtering and error‑only summaries
- Modular plugin architecture for community extensions

## Features

- MCP server over stdio and SSE
- Kubernetes tools:
  - List namespaces and pods
  - Error‑only pod summaries
  - Targeted logs with event context
- Prometheus querying with real HTTP calls
- Docker inspection and logs
- Local file log tail/grep (allowlisted paths only)
- Plugin bundles with a `nexus.yaml` manifest

## Quick Start

```bash
go build -o nexus ./cmd/nexus
./nexus --config nexus.yaml
```

SSE mode:

```bash
./nexus --transport sse --http-addr :8080 --base-url http://localhost:8080 --base-path /mcp
```

## Install

macOS (Homebrew tap):

```bash
brew tap edgeopslabs/nexus
brew install nexus
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

## Documentation

- `GUIDE.md` for testing and usage
- `ROADMAP.md` for milestones and release plan
- `CONTRIBUTING.md` for contributor workflow

## Security

- All tools are read‑only by default in safe mode
- Paths for local logs are allowlisted
- Policy engine supports allow/deny/confirm rules
- Logs and tool responses are constrained to reduce token burn

## License

Apache‑2.0
