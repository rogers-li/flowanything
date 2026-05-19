# internal_new

`internal_new` is the rebuild workspace for the next backend architecture.

The goal is to build the AI platform runtime around `core/*` first, then switch
`cmd/*` entrypoints only after the new runtime is verified.

Layering rules:

- `core/*` owns runtime semantics: agent reasoning, flow execution, workflow
  nodes, tools, connectors, config schema, expression mapping, and trace events.
- `internal_new/*` owns application composition: loading config, wiring core
  runtimes, selecting adapters, exposing use-case services, and later HTTP.
- Existing `internal/*` is read-only reference material during the rebuild.
  Do not copy compatibility shims or old data-shape conversion logic into the
  new runtime. If a protocol is wrong, fix the new config/API contract directly.

Current structure:

```text
internal_new/
  app/                  # use-case services and runtime composition around core
  platformconfig/       # config catalog assembly, validation, and publication
  adapters/
    config/             # config-as-code file loading
    connector/          # external protocol adapters, e.g. HTTP
    model/              # model provider adapters, e.g. mock/OpenAI-compatible
  interfaces/
    http/               # HTTP API layer over app.Host
  bootstrap/            # future cmd entrypoint helper
```

Current runnable paths:

- Bundle management: `HTTP -> platformconfig.Service -> BundleStore`
- Agent: `HTTP -> app.Host.RunAgent -> core/agentcore`
- Tool: `HTTP -> app.Host.InvokeTool -> core/tools`
- Connector: `HTTP -> app.Host.InvokeConnector -> core/connector`
- Workflow: `HTTP -> app.Host.RunWorkflow -> core/workflow -> core/flowengine`
- Trace: `core events -> core/trace.Collector -> trace store`
- Config lifecycle: `Draft Bundle -> Preview Bundle for debug/test` or
  `Draft Bundle -> Release Bundle -> Runtime Reload`
- Config publish: `platformconfig.Catalog -> core/config BundleSpec -> Publisher
  -> ValidateBundle/CompileRuntimeCatalog -> immutable release BundleStore`
- Debug sessions: `Preview Bundle -> app.DebugSessionManager -> preview Host`
- Run history: HTTP execution records bind request/result/trace to bundle
  metadata and can replay while the same preview session or active runtime is
  still available.

Bundle stores are lifecycle-separated:

- Draft store: console-edited config-as-code.
- Preview store: persisted test/debug snapshots generated from draft config.
- Release store: published immutable snapshots; active runtime reload only
  accepts release bundles.

New backend API surface:

- `GET /v1/bundles`
- `POST /v1/bundles`
- `GET /v1/previews`
- `GET /v1/releases`
- `GET /v1/bundles/{bundle_id}`
- `PUT /v1/bundles/{bundle_id}`
- `DELETE /v1/bundles/{bundle_id}`
- `GET /v1/bundles/{bundle_id}/inspect`
- `POST /v1/bundles/validate`
- `POST /v1/bundles/{bundle_id}/validate`
- `POST /v1/bundles/{bundle_id}/preview`
- `POST /v1/bundles/{bundle_id}/publish`
- `POST /v1/bundles/{bundle_id}/publish-and-reload`
- `GET /v1/bundles/{bundle_id}/resources?kind={resource_kind}&q={query}`
- `GET /v1/bundles/{bundle_id}/resources/{resource_kind}?q={query}`
- `GET /v1/bundles/{bundle_id}/resources/{resource_kind}/{resource_id}`
- `PUT /v1/bundles/{bundle_id}/resources/{resource_kind}/{resource_id}`
- `DELETE /v1/bundles/{bundle_id}/resources/{resource_kind}/{resource_id}`
- `GET /v1/bundles/{bundle_id}/connectors/{connector_id}/operations?q={query}`
- `GET /v1/bundles/{bundle_id}/connectors/{connector_id}/operations/{operation_id}`
- `PUT /v1/bundles/{bundle_id}/connectors/{connector_id}/operations/{operation_id}`
- `DELETE /v1/bundles/{bundle_id}/connectors/{connector_id}/operations/{operation_id}`
- `GET /v1/runtime/active-bundle`
- `POST /v1/runtime/reload`
- `GET /v1/catalog`
- `POST /v1/agents/run`
- `POST /v1/tools/invoke`
- `POST /v1/connectors/invoke`
- `POST /v1/workflows/run`
- `GET /v1/traces/{trace_id}`
- `GET /v1/debug-sessions`
- `POST /v1/debug-sessions`
- `GET /v1/debug-sessions/{session_id}`
- `DELETE /v1/debug-sessions/{session_id}`
- `POST /v1/debug-sessions/{session_id}/agents/run`
- `POST /v1/debug-sessions/{session_id}/workflows/run`
- `GET /v1/run-history`
- `GET /v1/run-history/{run_id}`
- `POST /v1/run-history/{run_id}/replay`

Local command:

```bash
make start-services
```

`scripts/local/start-services.sh` loads `configs/local/services.env`, then
optionally loads ignored machine-local overrides from
`configs/local/services.local.env`. The default local configuration uses
lifecycle-separated bundle files:

- `configs/local/workspace.draft.bundle.json`
- `configs/local/workspace.preview.bundle.json`
- `configs/local/workspace.release.bundle.json`

Migration plan:

1. Build `internal_new` services on top of `core/*`.
2. Re-implement adapters against the new protocol. Existing code may be used as
   behavioral reference only, not as compatibility baggage.
3. Switch `cmd/*` main files to `internal_new`.
4. Verify behavior and tests.
5. Archive/delete old `internal/*`, then rename `internal_new` to `internal`.
