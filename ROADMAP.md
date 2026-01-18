# Nexus Roadmap (v0.0.1 Launch Sprint)

This roadmap is optimized for a 7-day push to a production-candidate release. Each day ends with shippable milestones.

## Milestone 0: Baseline (Current)

- MCP server over stdio is stable.
- Module registry exists with Kubernetes and Prometheus modules.
- YAML config and contributor guide in place.

## Day 1: Reliability & Interface Contracts

- Finalize module lifecycle behavior (init errors, enable/disable).
- Add unit tests for config loading and registry.
- Add CLI flags for `--config` and `--safe-mode`.
- Harden logging to ensure no stdout corruption.

## Day 2: Governance Layer MVP

- Policy config: allow/deny by module/tool.
- Safe-mode defaults (read-only).
- Human confirmation flow for sensitive verbs.

## Day 3: Transport & Integration Demo

- Add SSE transport (HTTP endpoint).
- Tool inventory endpoint for UI/agent discovery.
- Demo integration with Claude Desktop / Cursor.

## Day 4: Official Modules (Tier 1)

- Logs module: tail, grep, last-N lines.
- Docker module: list/inspect containers.
- Prometheus: add auth support (token/basic).

## Day 5: Plugin Packaging MVP

- Define minimal plugin manifest spec.
- `nexus install <path|url>` for local bundles.
- Load external modules from a plugin bundle.

## Day 6: Wasm Sandbox Spike

- Integrate wazero and run a sample tool.
- Define minimal ABI for tool input/output.
- Document the SDK contract for plugin authors.

## Day 7: Release Candidate

- CI pipeline (build, test, lint).
- Versioned binaries and release notes.
- Docs: quickstart, security model, plugin authoring.

## Post-Launch (v0.1+)

- Registry index (Hub) + verified publishers.
- OCI/ORAS plugin distribution.
- Auto-discovery in K8s clusters.
- Multi-protocol support beyond MCP.
