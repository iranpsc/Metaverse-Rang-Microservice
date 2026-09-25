# Sitemap generator specification

This document is the generation contract for `sitemap-generator-service`.

`sitemap-spec.md` and `templates.json` were not present anywhere in this repository (all branches and the Git tree were searched). The rules below are the behavior of the existing MetaRang sitemap job:

- `App\Jobs\SitemapGenerator` (scheduled `everyThreeHours()` in `app/Console/Kernel.php`)
- `Video::toSitemapTag`, `VideoCategory::toSitemapTag`, `VideoSubCategory::toSitemapTag`
- `Calendar::scopeEvents` / `Calendar::scopeVersions`

Citizen URL patterns are not hardcoded in that job. They are read from `templates.json` (`citizens` → language → list of URL templates containing the literal placeholder `[code]`). The copy in this service root lists the public citizen routes on `https://metarang.com`.

## Schedule

Generate and replace sitemap files every 3 hours. The process also generates once when it starts, then on each 3-hour tick, so the export directory is not empty until the first clock boundary.

## Output directory

Write files into the `sitemaps` directory inside the service root (container path `/app/sitemaps`). Do not write sitemap XML to the repository root.

`SITEMAPS_EXPORT_PATH` is the host directory bind-mounted onto that `sitemaps` directory (example `/opt/metarang/sitemaps`).

## Files

| File | Source |
| --- | --- |
| `citizen-sitemap.xml`, then `citizen-sitemap-2.xml`, `citizen-sitemap-3.xml`, … | `users.code` expanded through `templates.json` |
| `education_single_video-sitemap.xml` | `videos` |
| `education_category-sitemap.xml` | `video_categories` |
| `education_sub_category-sitemap.xml` | `video_sub_categories` joined to `video_categories` |
| `calendar_events-sitemap.xml` | `calendars` where `is_version = 0` |
| `calendar_versions-sitemap.xml` | `calendars` where `is_version = 1` |

No sitemap index is written. A successful citizen run removes `citizen-sitemap*.xml` files that this run did not produce. If `templates.json` has no `citizens` key, citizen files are left unchanged and the other files are still written.

## XML

Each file is a sitemap urlset:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1" xmlns:video="http://www.google.com/schemas/sitemap-video/1.1" xmlns:news="http://www.google.com/schemas/sitemap-news/0.9">
  <url>
    <loc>https://example.com/path</loc>
    <lastmod>2006-01-02T15:04:05+03:30</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
</urlset>
```

- `lastmod` is the row `updated_at` formatted as an RFC 3339 / ATOM offset (`2006-01-02T15:04:05-07:00`). Omit `lastmod` when `updated_at` is null.
- `priority` is one decimal place.
- Category and subcategory URLs include `<image:image><image:loc>{ADMIN_PANEL_URL}/uploads/{image}</image:loc></image:image>` when `image` is not empty. `ADMIN_PANEL_URL` defaults to `https://admin.rgb.irpsc.com`.

## Citizen templates

`templates.json` shape:

```json
{
  "citizens": {
    "fa": ["https://metarang.com/fa/citizens/[code]"],
    "en": ["https://metarang.com/en/citizens/[code]"]
  }
}
```

Language keys and template arrays are applied in file order. For each user, replace `[code]` with `users.code`. Users are read in primary-key order.

- `changefreq`: `daily`
- `priority`: `0.8`
- At most 5000 `<url>` entries per file. The first file is `citizen-sitemap.xml`. The next files are `citizen-sitemap-2.xml`, `citizen-sitemap-3.xml`, and so on. A single user may span a file boundary.
- Skip users whose `code` is empty.
- If `citizens` is missing, skip citizen generation.

## Videos

One file, `education_single_video-sitemap.xml`, for every `videos` row with a non-empty `slug`:

- `https://rgb.irpsc.com/fa/education/watch/{slug}`
- `https://rgb.irpsc.com/en/education/watch/{slug}`
- `changefreq`: `monthly`
- `priority`: `0.8`

## Categories

One file, `education_category-sitemap.xml`, for every `video_categories` row with a non-empty `slug`:

- `https://rgb.irpsc.com/fa/education/category/{slug}`
- `https://rgb.irpsc.com/en/education/category/{slug}`
- `changefreq`: `monthly`
- `priority`: `0.8`
- image loc: `{ADMIN_PANEL_URL}/uploads/{image}` when `image` is set

## Subcategories

One file, `education_sub_category-sitemap.xml`. Each `video_sub_categories` row needs its parent category slug. Skip a row when either slug is empty.

- `https://rgb.irpsc.com/fa/education/category/{categorySlug}/{slug}`
- `https://rgb.irpsc.com/en/education/category/{categorySlug}/{slug}`
- `changefreq`: `monthly`
- `priority`: `0.8`
- image loc: `{ADMIN_PANEL_URL}/uploads/{image}` when `image` is set

## Calendar events

`calendars` rows with `is_version = 0`, ordered by `starts_at` descending, file `calendar_events-sitemap.xml`:

- `https://metarang.com/fa/calendar/{id}`
- `https://metarang.com/en/calendar/{id}`
- `changefreq`: `monthly`
- `priority`: `0.6`

## Calendar versions

`calendars` rows with `is_version = 1`, ordered by `created_at` descending, file `calendar_versions-sitemap.xml`. Skip an empty `version_title`.

- `https://metarang.com/fa/versions/{version_title}`
- `https://metarang.com/en/versions/{version_title}`
- `changefreq`: `monthly`
- `priority`: `0.6`

## Data source

Rows come from the shared MetaRang MySQL schema (`users`, `videos`, `video_categories`, `video_sub_categories`, `calendars`) using `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, and `DB_DATABASE`. No production credentials are embedded in the service.
