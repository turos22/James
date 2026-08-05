// Package web embute o build da SPA (web/dist) no binário do jamesd.
// Requer `npm run build` dentro de web/ antes de `go build`.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
