# Flow Anything

Go-first AI agent platform built around config-as-code.

The current rebuild centers runtime behavior in `core/*` and wires application
services through `internal_new/*`. Console pages edit standardized config;
runtime loads bundle snapshots and executes agents, tools, connectors, and
workflows from those configs.

## Architecture

- `core/config`: config protocol for agents, skills, tools, connectors,
  workflows, models, knowledge, policies, and deployable bundles.
- `core/flowengine`: generic event-driven flow engine with context, state, and
  control-flow nodes.
- `core/workflow`: workflow node implementations on top of `flowengine`.
- `core/agentcore`: model-driven agent execution and action planning.
- `core/tools` and `core/connector`: tool and connector execution abstractions.
- `core/trace`: event-based trace collection and tree rendering support.
- `internal_new`: application composition, adapters, HTTP API, debug sessions,
  run history, and config lifecycle management.
- `web/admin-console`: React admin console that edits bundle resources and
  runs preview/debug sessions against `ai-platform-runtime`.

## Local Development

Install frontend dependencies once:

```bash
make web-install
```

Start the rebuilt backend runtime:

```bash
make start-services
```

Start the admin console:

```bash
make web-dev
```

Stop backend services:

```bash
make stop-services
```

Restart backend and frontend together:

```bash
make restart-services
```

The local backend script starts `ai-platform-runtime` by default, loads
`configs/local/services.env`, writes PID records to `.runtime/local/services.pid`,
and writes logs to `log/local/*.log`.

## Local Config Files

The local workspace is lifecycle-separated:

- Draft: `configs/local/workspace.draft.bundle.json`
- Preview: `configs/local/workspace.preview.bundle.json`
- Release: `configs/local/workspace.release.bundle.json`

The console edits draft resources. Debug/test creates preview snapshots.
Publish writes release snapshots. Runtime reloads release bundles.

Machine-local secrets and model-provider overrides belong in:

```text
configs/local/services.local.env
```

That file is ignored by git and is loaded after `configs/local/services.env`.

Example override:

```bash
FLOW_ANYTHING_MODEL_PROVIDER=deepseek
FLOW_ANYTHING_MODEL_BASE_URL=https://api.deepseek.com
FLOW_ANYTHING_MODEL_API_KEY=your-local-secret
```

## Commands

```bash
make fmt
make test
make web-build
make run-ai-platform-runtime
```

Migrate P0 legacy config from the old local SQLite registry into a reviewable
bundle file:

```bash
make migrate-p0-config
```

This migrates only Connector/Operation, Tool, Skill, Agent, and model
references. Workflow and Agent Flow configs are intentionally skipped so they
can be rebuilt against the new workflow/agent-flow protocol.

The old MVP service commands are still present as reference entrypoints while
the rebuild is being completed, but new development should target
`ai-platform-runtime` and the config-as-code bundle APIs.
