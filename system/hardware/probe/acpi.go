package probe

import (
	"fmt"
	"strings"
)

// ACPIState is what the ACPI probe returns.
//
// Every field here is a derived fact. A table dump, a hex blob or ASL source
// must never reach a report: ACPI tables are machine-generated and routinely
// embed identifiers as _DSM method GUIDs and as strings in arguments, so a
// dump is a leak with extra steps. Which tables exist is useful; what is in
// them is not, and the difference is the whole reason this is a struct of
// facts rather than a captured table.
type ACPIState struct {
	// Present is false on a machine with no ACPI at all. It is distinct from
	// Present-with-no-tables: an ARM board with a device tree and no SMBIOS is
	// a different machine from an x86 laptop whose firmware exposes nothing.
	Present bool `json:"present"`

	// Tables are the ACPI table names the kernel exposes, uppercased by the
	// kernel already. This is the whole table probe: which tables exist, never
	// their contents.
	Tables []string `json:"tables,omitempty"`

	// SleepStates are the values from /sys/power/state, e.g. "freeze mem disk".
	SleepStates []string `json:"sleep_states,omitempty"`

	// MemSleep is the value of /sys/power/mem_sleep, a mask such as
	// "[s2idle] deep" where the brackets mark the mode the machine will
	// actually use. The brackets are kept, because dropping them loses the only
	// thing in this attribute that distinguishes the default from the merely
	// available.
	MemSleep string `json:"mem_sleep,omitempty"`

	// DefaultMemSleep is the bracketed mode, extracted for matching.
	DefaultMemSleep string `json:"default_mem_sleep,omitempty"`

	PlatformProfile        string   `json:"platform_profile,omitempty"`
	PlatformProfileChoices []string `json:"platform_profile_choices,omitempty"`

	// BacklightGraphics indicates whether the machine advertises a backlight
	// control, which is a cheap proxy for "this is a laptop with a panel".
	// Whether the panel actually dims is not a thing this can know.
	BacklightGraphics bool `json:"backlight_graphics"`

	// FPDTPresent reports whether the ACPI FADT is exposed, which is what
	// carries the low-power idle states on most modern firmware.
	FPDTPresent bool `json:"fpdt_present"`
}

// ACPI reads the ACPI sysfs tree relative to the reader's root.
//
// A machine with no ACPI returns (ACPIState{Present: false}, nil). That is not
// a degraded path: an ARM board has no ACPI, it simply is not that kind of
// machine, and calling that an error would make detection fail exactly where
// it should be reporting a fact.
//
// The deep path — acpidump, acpixtract, iasl — is deliberately not here. It
// writes hundreds of kilobytes to disk, it is CPU-bound, and it is needed by
// roughly one profile in fifty: the machine with a sleep problem. It belongs
// behind `zenith hardware report --deep`.
func ACPI(fs *FS) (ACPIState, error) {
	state := ACPIState{}

	tableNames, err := fs.entries("firmware/acpi/tables")
	if err != nil {
		return state, fmt.Errorf("acpi: reading firmware/acpi/tables: %w", err)
	}
	state.Tables = tableNames

	state.Present = fs.stat("firmware/acpi") == nil

	for _, name := range tableNames {
		if name == "FPDT" {
			state.FPDTPresent = true
		}
	}

	if raw, present, err := fs.readString("power/state"); err != nil {
		return state, fmt.Errorf("acpi: reading power/state: %w", err)
	} else if present {
		state.SleepStates = strings.Fields(raw)
	}

	// mem_sleep is a mask such as "[s2idle] deep" or "s3 s2idle". The bracketed
	// entry is the default, and the default is the only entry that predicts
	// what the machine will actually do on a lid close.
	if raw, present, err := fs.readString("power/mem_sleep"); err != nil {
		return state, fmt.Errorf("acpi: reading power/mem_sleep: %w", err)
	} else if present {
		state.MemSleep = raw
		state.DefaultMemSleep = defaultMemSleep(raw)
	}

	if profile, present, err := fs.readString("firmware/acpi/platform_profile"); err != nil {
		return state, fmt.Errorf("acpi: reading platform_profile: %w", err)
	} else if present {
		state.PlatformProfile = profile
	}

	if choices, present, err := fs.readString("firmware/acpi/platform_profile_choices"); err != nil {
		return state, fmt.Errorf("acpi: reading platform_profile_choices: %w", err)
	} else if present {
		state.PlatformProfileChoices = strings.Fields(choices)
	}

	if names, err := fs.entries("class/backlight"); err != nil {
		return state, fmt.Errorf("acpi: reading class/backlight: %w", err)
	} else {
		state.BacklightGraphics = len(names) > 0
	}

	return state, nil
}

// defaultMemSleep extracts the bracketed default from a mem_sleep mask.
//
// "deep [s2idle]" and "[s2idle] deep" both mean s2idle is the default, and a
// mask with no brackets means the kernel declared no default. Returning "" for
// the last case is correct: guessing the first token would claim a sleep
// behaviour the firmware never specified.
func defaultMemSleep(mask string) string {
	for _, token := range strings.Fields(mask) {
		if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]") && len(token) > 2 {
			return token[1 : len(token)-1]
		}
	}
	return ""
}
