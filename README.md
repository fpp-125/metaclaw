# MetaClaw

MetaClaw is a local-first infrastructure engine for AI agents.

This MVP implements a daemonless Go CLI that:
- Parses and validates `.claw` files.
- Compiles `.claw` into immutable `ClawCapsule` bundles.
- Enforces deny-by-default habitat policies.
- Runs agent containers through runtime adapters (Podman, Apple Container, Docker fallback).
- Stores lifecycle state in SQLite and logs events as JSONL.

## Commands

```bash
metaclaw init
metaclaw validate agent.claw
metaclaw compile agent.claw -o out/
metaclaw run agent.claw --runtime=podman
metaclaw run agent.claw --detach
metaclaw ps
metaclaw logs <run-id>
metaclaw inspect <run-id>
metaclaw inspect <capsule-dir>
metaclaw debug shell <run-id>
```

## Security Model

- Habitat defaults are strict:
  - network: `none`
  - mounts: empty
- Runtime backend can be overridden with `--runtime`.
- CLI overrides that attempt to change security boundaries are blocked.

## Development

Use local Go cache locations in restricted environments:

```bash
GOCACHE=/tmp/metaclaw-go-build \
GOPATH=/tmp/metaclaw-go \
GOMODCACHE=/tmp/metaclaw-go/pkg/mod \
go test ./...
```

## Runtime E2E Integration Tests

Integration tests that execute real containers live in `internal/manager/manager_integration_test.go`.

Requirements:
- `docker` or `podman` installed and healthy (`docker info` / `podman info` works).
- Network access if the test image is not already present locally.

Run:

```bash
GOCACHE=/tmp/metaclaw-go-build \
GOPATH=/tmp/metaclaw-go \
GOMODCACHE=/tmp/metaclaw-go/pkg/mod \
go test -tags=integration ./internal/manager -run TestE2ERuntime -v
```

Optional runtime override:

```bash
METACLAW_TEST_RUNTIME=docker go test -tags=integration ./internal/manager -run TestE2ERuntime -v
```
