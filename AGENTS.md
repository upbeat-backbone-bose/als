# AGENTS.md

ALS — Another Looking-glass Server. A single Go binary (`backend/als/als`) that serves a
Vue 3 SPA (`ui/`) over HTTP/WS. The UI bundle is compiled into the binary via `go:embed`,
so every backend build must be preceded by a UI build. This file is the single source of
truth for repo guidance — `CLAUDE.md` only imports it (`@AGENTS.md`) to avoid drift.

## Setup commands

Submodules are required (`ui/speedtest`, librespeed):

```bash
git clone --recursive <repo>            # or: git submodule update --init --recursive
cd backend && go mod download
cd ui && npm ci
```

## Repo layout

- `backend/main.go` — entry point. `--shell` runs `fakeshell.HandleConsole()` (restricted
  CLI, no HTTP server; sets `config.IsInternalCall = true` to skip log prefixes).
  Otherwise: `config.LoadWebConfig()` → `als.Init()`.
- `backend/als/` — the server core (`als.Init()` creates the gin server → registers
  routes → starts timers + queue → serves):
  - `route.go` — route registration. Feature-flagged endpoints (`config.Config.Feature*`)
    are only mounted when enabled: `/session` (SSE session) → `/method/…` (requires the
    `Session` header via `MiddlewareSessionOnHeader`) → `/session/:session/…` (requires
    `MiddlewareSessionOnUrl`; shell WS, speedtest file/download/upload). Static SPA files
    are served by `handleStatisFile` (`fs.Sub` + `http.FileServer`).
  - `controller/` — per-feature endpoints: `session/`, `ping/`, `iperf3/`, `speedtest/`
    (CLI + fakefile + librespeed), `shell/`, `cache/`. The client-facing config
    projection lives in `controller/session/session.go::buildClientConfig`.
  - `client/` — session store: `clients` map guarded by `sync.RWMutex` (exposed as
    `ClientsMu()`), sessions expire after 24 h and are swept hourly
    (`cleanupExpiredClients`). `client.SafeChannelSend` is the only sanctioned way to
    write an SSE channel.
  - `timer/` — two broadcast loops: `UpdateSystemResourceContext` (every 5 s, broadcasts
    the `MemoryUsage` event) and `SetupInterfaceBroadcastContext` (every 1 s,
    per-NIC RX/TX `InterfaceTraffic` event; only started when `FeatureIfaceTraffic`
    is on).
- `backend/config/` — env-driven runtime config (`load_from_env.go`); **no `.env` file**.
  Defaults in `init.go` (e.g. `ListenPort: "80"`).
- `backend/http/` — thin gin wrapper (`CreateServer` / `SetListen` / `Start` / `Shutdown`).
- `backend/fakeshell/` — restricted interactive console (`--shell` flag).
- `backend/embed/` — `//go:embed ui`; the `ui/` subdir is **build output, gitignored**.
  Embed data is baked into the binary at compile time: deleting it does **not** affect an
  already-built binary, it only makes the next `go build` fail (CI verifies
  `index.html` exists before building).
- `backend/internal/testutil/` — shared test helpers.
- `ui/` — Vue 3 + Vite 8 + Pinia + Tailwind v4 + naive-ui + vue-i18n (notes below).
- `docs/` — long-form documentation (`architecture.md`, `backend.md`, `interfaces.md`,
  `session.md`, `developer-guide.md`, `config.md`, `feature-flags.md`, …).
- `scripts/` — host bootstrap (`install-software.sh`, `install-speedtest.sh`).
- `.github/workflows/` — `ci.yml` (job per stage: `build-ui` → `backend`
  test/coverage/build, plus an independent `backend-lint` job), `release.yml`
  (tag-triggered cross-compile matrix `linux/darwin/windows × amd64/arm64`),
  `docker-image.yml` (release-published / manual dispatch), `docs.yml`,
  `cleanup-workflow-runs.yml`.

## Dev workflow (hot reload)

```bash
# Terminal 1 — UI dev server on :5173; proxies /session and /method (with WS) to 127.0.0.1:8080
cd ui && npm run dev
# Terminal 2 — backend on :80 by default (HTTP_PORT / LISTEN_IP override)
cd backend && go run .
```

Without a prior UI build the SPA paths 404: `npm run build` only writes `ui/dist/`;
`npm run build:embed` additionally syncs it into `backend/embed/ui/` so the Go embed
picks it up.

## Build pipeline (UI → backend)

```
npm run build        produces ui/dist/
npm run build:embed  copies ui/dist/ → backend/embed/ui/
go build             embeds backend/embed/ui into the als binary
```

Local one-shot: `cd ui && npm run build:embed && cd ../backend && go build -o als ./...`.
Because `-o als` targets the existing `backend/als/` package directory, the executable
lands at `backend/als/als` (`backend/als/als.exe` on Windows). CI transfers the UI the
same way: `build-ui` uploads the `ui-dist` artifact, the `backend` job downloads it into
`backend/embed/ui/` before `go test` / `go build`.

## Backend invariants (docs/developer-guide.md §3.4)

A regression in any of these has produced real production bugs; regression tests pin
them. Treat as hard rules:

1. **The SSE `Config` event only serialises the `ClientConfig` DTO** — never
   `*ALSConfig` (internal fields like `listen_host` or the iperf3 port range must not
   leak to clients).
2. **Write SSE channels only via `client.SafeChannelSend`** — a bare `channel <- msg`
   blocks forever when a consumer stalls, hanging every queued request.
3. **`HandleQueue(ctx)` must call `shutdownQueue` before returning on `<-ctx.Done()`** —
   otherwise graceful shutdown leaves parked waiters and in-flight HTTP hanging 60 s.
4. **No blocking work inside `WaitQueue` callbacks** — `SafeChannelSend` is fine; bare
   channel sends, `time.Sleep`, synchronous network calls are not.
5. **`GetContext` returns a fresh, independent ctx per call** — never store a
   `cancelFunc` field on `ClientSession` (concurrent callers overwrite each other and
   goroutines leak), and `RemoveClient`/`RemoveExpiredClients` must not force-cancel
   in-flight contexts.
6. **`sizeToBytes` rejects non-positive numbers** — otherwise a 0-byte HTTP body slips
   through to speedtest consumers.
7. **`HandleFakeFile` must not call `WaitQueue`** — that route has no queue handler;
   `WaitQueue` would block forever.

## Frontend notes

- `ui/vite.config.js` is a thin shell: all plugins live in `ui/vite.shared.js`
  `createSharedPlugins()`, which `vitest.config.js` reuses — change the plugin stack in
  exactly one place.
- `vite.shared.js#build-script` copies `ui/speedtest/speedtest_worker.js` (git submodule)
  to `dist/speedtest_worker.js` during build.
- Path alias `@` → `ui/src`. `unplugin-auto-import` injects Vue reactivity APIs plus
  `useDialog/useMessage/useNotification/useLoadingBar`; `unplugin-vue-components` +
  `NaiveUiResolver` register naive-ui components on demand.
- Sole Pinia store: `ui/src/stores/app.js` (`useAppStore`). It opens the SSE connection
  (`./session`, ~1 s reconnect delay with `?resume=<sessionId>` so in-flight requests
  keep the same session), listens to `SessionId` / `Config` / `MemoryUsage`; components
  add extra listeners on the same source (e.g. `TrafficDisplay.vue` handles
  `InterfaceTraffic`).
- WebSocket shell: `ui/src/components/Utilities/Shell.vue` (`@xterm/xterm` +
  `addon-fit`), frame protocol `1<data>` for input, `2<rows>;<cols>` for resize.
- Theme: light/dark follows the OS theme via naive-ui `useOsTheme()` in `App.vue`.
  naive-ui is CSS-in-JS, so any theme overrides must use concrete colour strings, never
  `var(--…)`.
- Tailwind v4 CSS-first: the stylesheet entry is `ui/src/assets/base.css`
  (`@import "tailwindcss/…"`, imported by `main.js`).
- i18n: `ui/src/locales/*.json` (8 languages); detection in `config/lang.js::autoLang()`
  (saved locale → exact match → prefix match → default en-US).
- `src/test-setup.js` globally mocks i18n / EventSource / matchMedia for Vitest.

## Code style

- Go: config in `backend/.golangci.yml` (errcheck, govet, staticcheck, gocritic, etc.).
  Run `go vet ./...` before pushing. CI also runs gofmt, goimports, staticcheck and
  golangci-lint (pinned v2.15.0).
- UI: ESLint flat config (`ui/eslint.config.js`) + Prettier. Run
  `cd ui && npm run lint` and `npm run format` before pushing.

## Testing

- UI: `cd ui && npm test` (Vitest 5; ~80 tests / 10 files). Tests live in
  `ui/src/**/__tests__/*.test.js`; stub heavy components (ApexCharts, xterm) when
  mounting or jsdom crashes. Single file:
  `npx vitest run src/components/__tests__/Copy.test.js`. `npm run build` doubles as a
  build smoke test.
- Backend: `cd backend && go test -race -count=1 -timeout 120s ./...`. Single
  package/test: `go test ./als/client -run TestHandleQueue -v`.
- Coverage gate: CI fails if total backend coverage drops below **52%** — the threshold
  and its rationale comment live in the Coverage step of `.github/workflows/ci.yml`.
- Add tests next to the code under test (convention: `*.test.js` for UI, `*_test.go`
  for backend).

## PR & commit conventions

- Branch from `master`; never push to it directly.
- Conventional commits: `feat:` / `fix:` / `docs:` / `refactor:` / `chore:`. Multi-stage
  work is split into atomic commits (`stage 1 - …`, `stage 2 - …`) so per-stage diffs
  stay reviewable and revertible.
- Dev-review fixes use `fix(ui): <scope> - <one-line>` and stack on the stage commits
  without rebasing away.
- Commit messages must **not** include AI attribution footers (`Co-Authored-By:`,
  "Generated with Claude Code", "🤖 Generated by Claude", etc.).
- Push + open PR only on explicit user go-ahead.

## Security & config

- Runtime config is read from environment variables only (`HTTP_PORT`, `LISTEN_IP`,
  `PUBLIC_IPV4`, `UTILITIES_*` for feature flags and the iperf3 port range). Full list:
  `docs/config.md`.
- `backend/als/als` (and `als.exe`) are local build artifacts — gitignored, never
  commit them. Dev logs (`als-dev*.log`) are gitignored as well.
- The locally populated `backend/audit/` tree is gitignored; only
  `backend/audit/{.gitignore,EVIDENCE.md,SPEC.md}` are tracked.
- Dependabot monitors `ui/package.json` and Go modules; transitive CVEs
  (`brace-expansion`, `browserslist`) need a manual `npm audit fix` PR because
  dependabot does not bump indirect dependencies on its own.
