// Package templates embeds the deterministic module templates used by the
// distributable mss CLI. Generated thin hosts therefore do not need to copy or
// vendor the Foundation template source tree.
package templates

import "embed"

// Module contains every deterministic AdminModule template.
//
//go:embed module
var Module embed.FS
