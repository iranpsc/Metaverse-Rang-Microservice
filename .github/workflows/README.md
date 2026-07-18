# CI/CD Workflows

## Overview

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| Services CI/CD | `services-ci.yml` | Push/PR to `main`/`develop` (service, shared, or schema changes), manual | Detects changed services and runs the full CI/CD pipeline for each |
| Service CI (Reusable) | `service-ci.yml` | Called by `services-ci.yml` | Lint → Test → Build/Scan → Deploy for a single service |
| Integration Tests | `integration-tests.yml` | Push/PR to `main`/`develop`, daily at 02:00 UTC, manual | Full cross-service integration suite, wallet concurrency, and golden JSON tests |
| Shared Packages CI | `shared-packages.yml` | Push/PR touching `shared/**`, manual | Lint, test, and vulnerability-check shared Go packages |
| WebSocket Gateway CI/CD | `websocket-gateway.yml` | Push/PR touching `websocket-gateway/**`, manual | Build, scan, and push the Node.js WebSocket gateway image |
| Load Tests | `load-tests.yml` | Weekly on Sundays at 03:00 UTC, manual | k6 load tests against the staging environment |

## How service CI works

`services-ci.yml` is the single entry point for all 14 Go services. On every push or PR it:

1. Runs `dorny/paths-filter` to detect which services changed.
2. Builds a JSON matrix of affected services:
   - A change under `services/<name>/**` rebuilds only that service.
   - A change under `shared/**`, `scripts/schema.sql`, or the CI workflows rebuilds **all** services.
3. Calls the reusable `service-ci.yml` for each service in parallel.

The reusable pipeline per service:

```
lint (golangci-lint)
  └─ test (unit tests + govulncheck + Codecov, against MySQL/Redis containers)
       └─ build (Docker build → Trivy scan → push on non-PR events)
            └─ deploy (main only, production environment: kubectl set image + rollout + rollback on failure)
```

Notes:

- PRs build the Docker image but never push it.
- Images are tagged with the commit SHA (used by deploy), branch name, `branch-shortsha`, semver (on tags), and `latest` on the default branch.
- `health-check-service` skips the deploy job (`enable_deploy: false`).
- Deploys target the `production` GitHub environment — configure required reviewers there to gate deploys.

## Manual full rebuild

Use **Actions → Services CI/CD → Run workflow**:

- `services: all` — rebuild every service (replaces the old `all-services.yml`).
- `services: auth-service,grpc-gateway` — rebuild specific services (comma-separated).

## Required secrets

| Secret | Used by | Required |
|--------|---------|----------|
| `DOCKER_USERNAME` / `DOCKER_PASSWORD` | Image push | Yes |
| `KUBE_CONFIG` | Deploy, load tests | For deploys |
| `CODECOV_TOKEN` | Coverage upload | Optional |

## Shared building blocks

- `.github/actions/setup-test-db/` — composite action that installs the MySQL client and loads `scripts/schema.sql` into the MySQL service container. Used by `service-ci.yml` and `integration-tests.yml`.
- All third-party actions are pinned to commit SHAs; Dependabot (`.github/dependabot.yml`) keeps them updated weekly.

## Branch protection

Recommended required checks for `main`/`develop`:

- **Services CI/CD** (covers lint/test/build of changed services)
- **Integration Tests**
- **Shared Packages CI** (for shared-only changes)
