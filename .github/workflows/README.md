# CI/CD Workflows

## Overview

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| Services CI/CD | `services-ci.yml` | Push/PR to `main`/`dev` (service, shared, or schema changes), manual | Detects changed services and runs the full CI/CD pipeline for each |
| Service CI (Reusable) | `service-ci.yml` | Called by `services-ci.yml` | Lint → Test → Build/Scan → Deploy for a single service |
| Shared Packages CI | `shared-packages.yml` | Push/PR touching `shared/**`, manual | Lint, test, and vulnerability-check shared Go packages |

## How service CI works

`services-ci.yml` is the single entry point for all 16 Go services. On every push or PR it:

1. Runs `dorny/paths-filter` to detect which services changed.
2. Builds a JSON matrix of affected services:
   - A change under `services/<name>/**` rebuilds only that service.
   - A change under `shared/**`, `scripts/schema.sql`, or the CI workflows rebuilds **all** services.
3. Calls the reusable `service-ci.yml` for each service in parallel.

The reusable pipeline per service:

```
lint (golangci-lint)
  └─ test (unit + integration tests, govulncheck, coverage summary + Codecov, against MySQL/Redis containers)
       └─ build (Docker build → Trivy scan → push on non-PR events)
            └─ deploy (main only, production environment: kubectl set image + rollout + rollback on failure)
```

Notes:

- PRs build the Docker image but never push it.
- Images are tagged with the commit SHA (used by deploy), branch name, `branch-shortsha`, semver (on tags), and `latest` on the default branch.
- `health-check-service` skips the test and deploy jobs (`enable_test: false`, `enable_deploy: false`).
- Deploys target the `production` GitHub environment — configure required reviewers there to gate deploys.

## Manual full rebuild

Use **Actions → Services CI/CD → Run workflow**:

- `services: all` — rebuild every service (replaces the old `all-services.yml`).
- `services: auth-service,support-service` — rebuild specific services (comma-separated).

## Required secrets

| Secret | Used by | Required |
|--------|---------|----------|
| `DOCKER_USERNAME` / `DOCKER_PASSWORD` | Image push to Docker Hub (`abbasajorloo/<service>`) | Yes |
| `KUBE_CONFIG` | Deploy | For deploys |
| `CODECOV_TOKEN` | Coverage upload | Optional |

## Shared building blocks

- `.github/actions/setup-test-db/` — composite action that installs the MySQL client and loads `scripts/schema.sql` into the MySQL service container. Used by `service-ci.yml`.
- `.github/scripts/go-test-checklist.sh` — runs `go test -json` and writes a package checklist to the job summary.
- `.github/scripts/go-coverage-summary.sh` — merges unit/integration coverage profiles and writes total + per-package coverage to the job summary.
- All third-party actions are pinned to commit SHAs; Dependabot (`.github/dependabot.yml`) keeps them updated weekly.

## Branch protection

Recommended required checks for `main`/`dev`:

- **Services CI/CD** (covers lint/test/build of changed services)
- **Shared Packages CI** (for shared-only changes)
