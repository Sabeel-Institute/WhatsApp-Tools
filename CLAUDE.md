# CLAUDE.md

Context for AI assistants working in this repository. User-facing docs: README.md. Architecture and build: DEVELOPING.md.
Why things are the way they are: DECISIONS.md.

## What this is

Sabeel WhatsApp Tools: a Go desktop app (local web UI embedded with go:embed) for Sabeel Institute staff. It is a
**host for WhatsApp workflows**; "Course groups" is the first. Users are non-technical: plain language everywhere, never
"JID", "E.164", "API" or status codes.

## Commands

- `make check`: gofmt (CI), vet, Go tests, shellcheck, filename-collision check, svelte-check, prettier. Run before
  committing.
- `make e2e`: Playwright suite against the built binary in demo mode. `make screenshots` refreshes `docs/screenshots/`.
- `make demo` / `make run`, `make dist` (all OSes, from Linux).
- Web formatting: `cd web && npm run format`.

## Conventions

- Commit often and push straight to `main` (the user asked for this); CI runs on every push.
- Every colour is a token in `web/src/app.css` (Sabeel brand: ivory, raspberry, sage; light theme only). No raw
  colours in components.
- Every WhatsApp call inside a job goes through `people.Call` and reads delays from `deps.Pacing`
  (`internal/whatsapp/pacing.go`). Never add people or send messages without pacing.
- Anything new from WhatsApp goes into the `whatsapp.Client` interface **and** both implementations (`real`, `demo`).
- User-facing error text is written on the Go side (`api.Error.Message`, `jobs.Failure.Message`) and shown as is.
- Never log phone numbers; `internal/logging` redacts digit runs as a safety net, not as permission.
- File names must not collide case-insensitively (`scripts/check-filenames.sh`; macOS CI caught `Toast.svelte` vs
  `toast.svelte.ts`).
- Platform code lives in `internal/platform/*_<os>.go` with build tags; the core stays OS-neutral and CGO-free.

## Gotchas

- whatsmeow is pinned to a commit; its API drifts. See "Updating whatsmeow" in DEVELOPING.md.
- `internal/webui/dist/.gitkeep` must stay: Vite empties `dist/` and the build restores the placeholder.
- The e2e suite shares one app and one page across tests in order (`serial`); wizard drafts persist across tests
  by design.
- Input robustness that is a safety net, not a contract: column-header matching ignores case and punctuation; numbers
  accept Arabic-Indic digits and stray labels.
