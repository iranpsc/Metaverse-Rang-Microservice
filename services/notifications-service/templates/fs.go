// Package templates embeds HTML email layouts for notifications-service.
package templates

import "embed"

//go:embed email/*.html.tmpl email/dynasty/*.html.tmpl
var EmailFS embed.FS
