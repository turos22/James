// Package armazenamento concentra os artefatos compartilhados entre os
// repositórios (SQL numerado embutido no binário).
package armazenamento

import "embed"

//go:embed migracoes/*.sql
var ArquivosMigracao embed.FS
