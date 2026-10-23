// The control tools. The hardware detection libraries live in their own module
// under system/hardware because they are self-contained and separately
// testable; this module is what a user actually runs.
//
// The replace directive points at that module in-tree rather than resolving it
// from a registry. There is no published module to resolve, and a build that
// needed one could not be reproduced from a checkout.
module zenith/desktop

go 1.26

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/coder/websocket v1.8.12
	github.com/godbus/dbus/v5 v5.2.2
	golang.org/x/sys v0.27.0
	zenith/hardware v0.0.0
)

replace zenith/hardware => ../system/hardware
