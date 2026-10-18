// The control tools. The hardware detection libraries live in their own module
// under system/hardware because they are self-contained and separately
// testable; this module is what a user actually runs.
//
// The replace directive points at that module in-tree rather than resolving it
// from a registry. There is no published module to resolve, and a build that
// needed one could not be reproduced from a checkout.
module zenith/desktop

go 1.26

require zenith/hardware v0.0.0

replace zenith/hardware => ../system/hardware
