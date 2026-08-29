# Contributing

Thanks for contributing to Context Baggage. This is a small, intentionally boring Go CLI — the first rule is to keep it that way.

## Setup

- Go toolchain — see [docs/development.md](docs/development.md).
- Optional: `golangci-lint` and `git`.

## Branch / PR workflow

1. Start from an up-to-date `main`:

   ```bash
   git switch main && git pull --ff-only && git status --short
   ```

2. Create a short-lived branch, e.g. `feat/<desc>` or `docs/<desc>`.
3. Make a small, focused change. Do not commit directly to `main`.
4. Open a Pull Request to `main`. Changes enter `main` only through a reviewed PR (squash merge).

## Local validation

Run before opening a PR:

```bash
gofmt -l .
go vet ./...
go test ./...
go build ./...
golangci-lint run
```

`gofmt -l .` must return no files and `golangci-lint run` must report `0 issues`. Do not weaken lint rules to get a green check.

## Code-quality expectations

- Prefer obvious code over clever code.
- Use descriptive names.
- Keep functions focused; avoid speculative abstractions.
- Add idiomatic Go doc comments for exported APIs and non-obvious helpers.
- Explain WHY in comments for safety-sensitive behavior.
- Add focused regression tests for safety invariants.
- Do not refactor unrelated code in a feature PR.

The detailed, normative guide is [docs/code-quality.md](docs/code-quality.md). It applies equally to human contributors and AI coding agents.

## Testing expectations

- Descriptive test names that describe the behavior under test.
- Use `t.TempDir()` and existing project helpers.
- Assert semantic values, file presence/absence, IDs, hashes, state transitions, and errors.
- Add a regression test whenever code safeguards a safety invariant (no silent last-write-wins, no re-key, no duplicate LocalPath ownership, etc.).

## Documentation expectations

Update only current/normative docs made stale by a change. Do not rewrite historical project logs. Keep PRs small and focused.
