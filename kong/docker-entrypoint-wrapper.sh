#!/bin/sh
# Render kong.yml, then start Kong.
# PROJECT_ENV=staging or development inserts dev/localhost CORS origins
# at the "# cors-dev-origins" marker. Any other value keeps the public list.
set -eu

src=/kong/kong.yml
dst=/tmp/kong.rendered.yml
marker='# cors-dev-origins'

if [ ! -f "$src" ]; then
  echo "kong: missing declarative config at $src" >&2
  exit 1
fi

# Trim whitespace and CR. A Windows .env or a trailing space must not
# silently fall through to the public allowlist on a staging server.
project_env=$(printf '%s' "${PROJECT_ENV:-production}" | LC_ALL=C tr '[:upper:]' '[:lower:]' | LC_ALL=C tr -d '[:space:]')
if [ -z "$project_env" ]; then
  project_env=production
fi

# CORS origin entries inserted at the marker. Limited to that block so a
# localhost path elsewhere in kong.yml cannot trip the production guard.
dev_origin_items() {
  sed -n '/# cors-dev-origins/,/methods:/p' "$1" \
    | grep -E '^[[:space:]]+-' \
    | grep -e 'localhost' -e '127' -e 'dev-reactjs.metarang.com' -e 'dev-nextjs.metarang.com'
}

case "$project_env" in
  staging|development)
    if ! grep -q "$marker" "$src"; then
      echo "kong: $src is missing the $marker marker" >&2
      exit 1
    fi
    : > "$dst"
    inserted=0
    while IFS= read -r line || [ -n "$line" ]; do
      printf '%s\n' "$line" >> "$dst"
      case "$line" in
        *"$marker"*)
          if [ "$inserted" -eq 0 ]; then
            # Explicit ^ and $ so a future Kong that stops auto-anchoring
            # cannot treat these as prefix matches.
            cat >> "$dst" <<'EOF'
        - https://dev-reactjs.metarang.com
        - https://dev-nextjs.metarang.com
        - '^http://localhost:\d+$'
        - '^http://127\.0\.0\.1:\d+$'
EOF
            inserted=1
          fi
          ;;
      esac
    done < "$src"
    if [ "$inserted" -eq 0 ] || ! dev_origin_items "$dst" >/dev/null; then
      echo "kong: failed to insert dev CORS origins" >&2
      exit 1
    fi
    echo "kong: PROJECT_ENV=$project_env; dev CORS origins enabled" >&2
    ;;
  *)
    cp "$src" "$dst"
    if dev_origin_items "$dst" >/dev/null; then
      echo "kong: ERROR: dev CORS origin is present in $src while PROJECT_ENV=$project_env" >&2
      exit 1
    fi
    if [ "$project_env" != "production" ]; then
      printf 'kong: WARN: PROJECT_ENV=%s is not production, staging, or development; public CORS origins only\n' "$project_env" >&2
    fi
    echo "kong: PROJECT_ENV=$project_env; public CORS origins only" >&2
    ;;
esac

export KONG_DECLARATIVE_CONFIG="$dst"

# Validate the rendered file without starting the proxy:
#   docker-entrypoint-wrapper.sh render-only
if [ "${1:-}" = "render-only" ]; then
  exit 0
fi

if [ "$#" -eq 0 ]; then
  set -- kong docker-start
fi

exec /docker-entrypoint.sh "$@"
