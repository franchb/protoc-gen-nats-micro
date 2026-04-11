# GitHub CI Workflow Design

**Date:** 2026-04-11
**Status:** Draft
**PR context:** franchb/protoc-gen-nats-micro#5

## Goal

Add CI checks (linting and tests) to the repository so PRs get automated validation. Currently only a docs deployment workflow exists.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Linter | golangci-lint | Industry standard, 70+ linters in one tool |
| Go versions | `stable` + `oldstable` | Auto-resolves to latest two Go release lines, zero maintenance |
| Triggers | PR + push to main + tags `v*` | Covers PRs, post-merge, and releases |
| Job layout | Separate lint and test jobs | Lint failures show distinctly from test failures |
| Coverage | Codecov | PR comments, badges, trend tracking |
| Workflow file | Single `.github/workflows/ci.yml` | Simple, standard for project this size |

## Files to Create

### 1. `.github/workflows/ci.yml`

Single workflow, two jobs.

**Triggers:**
- `pull_request` (all branches)
- `push` to `main`
- `push` tags matching `v*`

**Job: `lint`**
- `ubuntu-latest`
- `actions/checkout@v4`
- `actions/setup-go@v5` with `go-version: stable`
- `golangci/golangci-lint-action@v8` with `working-directory: tools/protoc-gen-nats-micro`
- Runs only once (not matrixed) — linting on the latest Go is sufficient

**Job: `test`**
- `ubuntu-latest`
- Matrix: `go-version: ['stable', 'oldstable']`
- `actions/checkout@v4`
- `actions/setup-go@v5` with matrix version
- `go test -race -count=1 -coverprofile=coverage.out ./...` in `tools/protoc-gen-nats-micro`
- `go build ./...` in root module to verify compilation
- `codecov/codecov-action@v5` to upload `coverage.out`
  - Uses `CODECOV_TOKEN` secret
  - Flagged by Go version for matrix deduplication

### 2. `.golangci.yml`

Minimal config at repo root. Keeps default linters enabled. No custom rules — start simple, tighten later as needed.

## Out of Scope

- Release automation (goreleaser, etc.)
- Docker builds
- Example project CI (separate modules in `examples/`)
- Branch protection rules (manual GitHub settings)

## Codecov Setup

Requires a `CODECOV_TOKEN` repository secret. The user must:
1. Sign up / log in at codecov.io with their GitHub account
2. Add the `franchb/protoc-gen-nats-micro` repository
3. Copy the upload token
4. Add it as a repository secret named `CODECOV_TOKEN` in GitHub Settings > Secrets
