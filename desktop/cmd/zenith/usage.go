package main

// The per-subcommand usage texts. They live in one file so that a flag added to
// a subcommand and the text describing it are edited in the same place.
const reportUsage = `Usage: zenith hardware report [flags]

Prints this machine's hardware, redacted. The output is safe to paste into an
issue: serials appear as stable tokens, never in the clear.

  --format text|json|profile   output shape (default text)
  --profile                    shorthand for --format profile
  --deep                       include ACPI-derived facts
  --raw                        unredacted dump, local only; refuses under CI
  --out FILE                   write to a file instead of stdout
  --root DIR                   read this sysfs tree instead of /sys
  --profiles DIR               profile database directory
`

const matchUsage = `Usage: zenith hardware match [flags]

Reports the profile that claims this machine, its support tier, and the
candidates that matched but lost. Exit code 3 means no profile matches, or two
profiles tie and disagree — an answer, not a failure.

  --root DIR        read this sysfs tree instead of /sys
  --profiles DIR    profile database directory
`

const lintUsage = `Usage: zenith hardware lint-profiles [flags]

Checks every profile against the schema rules in system/hardware/README.md, and
warns about pairs of profiles that can select the same machine — the shape that
becomes a conflict once a third arrives.

  --strict          exit non-zero on a warning as well as an error
  --profiles DIR    profile database directory
`

const captureUsage = `Usage: zenith hardware capture --out DIR [flags]

Runs the same readers a live machine does and writes the files they consume, so
a fixture is a machine. Captures are redacted as they are written, so a committed
fixture cannot contain a serial.

  --out DIR     required; the directory to write
  --root DIR    read this sysfs tree instead of /sys
`
