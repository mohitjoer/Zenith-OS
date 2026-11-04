package wm

import _ "embed"

// schemaRows are the Hyprland exclusive settings rows, in the shape the Hub's
// settings renderer consumes. The rows are the single source: moving them here is
// what lets a compositor's settings arrive by shipping this file rather than
// editing the Hub. Each row's `page` names the surface that draws it: the shared
// window-manager page, or a capability-gated bespoke page (plugins, animations,
// layerrules) that keeps its own interactions and reads its rows from here.
//
//go:embed schema.json
var schemaRows []byte
