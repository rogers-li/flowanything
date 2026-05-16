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

- `/platform-api` -> `http://localhost:8080`
- `/agent-runtime` -> `http://localhost:8082`
- `/model-gateway` -> `http://localhost:8085`

## Current Scope

This is a front-end framework skeleton. It uses realistic mock data to validate information architecture before all back-end management APIs are available.
