#!/bin/sh
set -eu

mkdir -p /app/sitemaps
chown appuser:appuser /app/sitemaps
exec su-exec appuser /app/sitemap-generator-service
