package content

import "embed"

// FS contains the versioned catalog and the default Chezmoi source.
//
//go:embed components/*.json profiles/*.json integrity.json chezmoi
var FS embed.FS
