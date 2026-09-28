# AGENTS.md

Guidance for AI coding agents working in this repository. Follow this file first, then load the skills it references.

## Skills

This repository uses the following installed skills. Read them when they are relevant to the task:

- `.agents/skills/golang-code-style/SKILL.md` — Go code style: line-length/breaking, variable declarations, control flow, function design, code organization. Follow when writing or reviewing Go code.
- `.agents/skills/golang-error-handling/references/error-handling.md` — Error handling and logging: **single handling rule** (log *or* return, never both), panic/recover rules, `oops` context, `slog` logging. Follow when touching error paths.

## Conventions (summary)

- Errors are handled once: return with context (`fmt.Errorf(... %w)`) through the stack; log only at boundaries (main, handlers, goroutines). No `log.Fatal`/panic for runtime conditions — panic only for programmer errors (e.g. invalid config).
- Keep functions short, ≤4 parameters (use an options struct beyond that). Early-return edge cases; drop unnecessary `else`.
- Use `:=` for non-zero values, `var` for zero values. Initialize slices/maps explicitly, never nil. Composite literals use field names.
- Break lines beyond ~120 chars at semantic boundaries; one argument per line for 4+ arg calls.
- No comments unless they earn their place (see `weightedRoundRobin` doc comment).
- Balance algorithms are in `pool/pool.go`; `Pool` uses a mutex (`p.mu`) for Weighted Round Robin state and `atomic` for round-robin cursor.

## Commands

```bash
go build ./...
go vet ./...
go test ./...          # unit tests (backend, pool, utils)
go test -race ./...    # required for concurrency-sensitive changes
go mod tidy            # keep go.mod tidy after dependency changes
```

Always run `go build`, `go vet`, and `go test ./...` (plus `-race` for concurrency changes) before finishing a task.

## Layout

- `main.go` — flag parsing, backend/proxy/pool wiring, health-check & stats loops
- `backend/` — `Backend` implementation of the `server.Server` interface (metrics + weight)
- `pool/` — `Pool`: algorithm selection, health checks, stats
- `proxy/` — reverse-proxy construction and the failure retry/refailover `ErrorHandler`
- `server/` — the `Server` interface
- `utils/` — request-attempt/retry counters stored in request context