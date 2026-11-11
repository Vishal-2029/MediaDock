package migrations

import "embed"

// FS embeds all SQL migration files into the compiled binary.
//
//go:embed *.sql
var FS embed.FS

