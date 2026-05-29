# Flow Anything Admin Console

Enterprise management console for the Flow Anything AI platform.

## Design Direction

- Pattern: Enterprise Gateway
- Style: Data-Dense Dashboard
- Palette: navy, blue, green, and calm blue-gray surfaces
- Typography: Fira Sans for interface text, Fira Code for IDs and operational metadata
- UX rules: keyboard-accessible navigation, visible focus states, mobile-safe tables, no emoji icons

## Local Development

```bash
cd web/admin-console
npm install
npm run dev
```

The Vite dev server proxies:

- `/ai-platform-runtime` -> `http://localhost:8081`

## Current Scope

This console edits config-as-code resources and talks to the rebuilt unified AI platform runtime.
