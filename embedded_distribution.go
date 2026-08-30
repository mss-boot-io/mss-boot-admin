// Package distribution contains the reviewed Blueprint source embedded in the
// release-built mss tools. It deliberately exports read-only bytes only; all
// provenance validation and rendering policy remains in internal/mss.
package distribution

import "embed"

// embeddedDistribution is the single package-first application source. The
// all: prefix is required because Thin Host contracts include dot-directories
// and dotfiles such as .mss, .github, .gitignore, and web/.npmrc.
//
//go:embed .mss/admin-presentation-catalog.yaml .mss/schemas/admin-presentation-catalog.schema.json .mss/blueprints/management-system.yaml all:templates/application
var embeddedDistribution embed.FS

// EmbeddedFS returns the immutable read-only Distribution source compiled into
// the current binary.
func EmbeddedFS() embed.FS {
	return embeddedDistribution
}
