# Sitemap Generator Service

Generates MetaRang sitemap XML on a 3-hour schedule and writes it to the `sitemaps` directory in this service root.

Rules live in `sitemap-spec.md`. Citizen URL patterns live in `templates.json` in this directory.

## Schedule

`cmd/server` starts an in-process ticker (`internal/scheduler`, interval `3 * time.Hour`). It generates once at process start, then again every 3 hours, until SIGINT or SIGTERM. Set `SITEMAP_RUN_ONCE=true` to generate a single time and exit.

Liveness is `GET /live` and `GET /health` on `HTTP_PORT` (default `8071`). `GET /ready` returns 200 after a successful generation.

## Export path

`config.env` (copied from `config.env.sample`) sets:

```env
SITEMAPS_EXPORT_PATH=/opt/metarang/sitemaps
```

The process writes to `SITEMAPS_OUTPUT_DIR` (default `<service root>/sitemaps`, `/app/sitemaps` in Docker). Compose bind-mounts the host path onto that directory:

```yaml
volumes:
  - ${SITEMAPS_EXPORT_PATH:-/opt/metarang/sitemaps}:/app/sitemaps
```

Compose interpolates `SITEMAPS_EXPORT_PATH` from the project `.env` or the shell, the same way `UPLOADS_PATH` is interpolated for storage-service. `env_file` does not feed Compose interpolation, so the default host path is `/opt/metarang/sitemaps`, matching `config.env`. The container path is always the service `sitemaps` directory.

`config.env` is gitignored like the other services. Create it with:

```bash
cp services/sitemap-generator-service/config.env.sample services/sitemap-generator-service/config.env
```

or `./scripts/generate-configs.sh` from the repo root.

## Database

The generator reads `users`, `videos`, `video_categories`, `video_sub_categories`, and `calendars` through `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, and `DB_DATABASE`. It does not embed production credentials. Compose overrides `DB_*` with the shared MySQL settings.

## Run locally

```bash
cd services/sitemap-generator-service
cp config.env.sample config.env
SITEMAP_RUN_ONCE=true SITEMAPS_OUTPUT_DIR=./sitemaps go run ./cmd/server
```
