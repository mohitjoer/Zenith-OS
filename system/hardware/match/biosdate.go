package match

import (
	"fmt"
	"strings"
	"time"
)

// compareBIOSDate evaluates a profile's bios_date comparator against the
// machine's firmware date.
//
// The comparator is the only non-equality in the match key: ">=", ">", "<=",
// "<" and "==" against an ISO date, because a profile that pins an exact
// firmware date stops matching the moment the vendor ships a new one.
//
// The machine's date is SMBIOS MM/DD/YYYY and the profile's is ISO. Both are
// parsed explicitly rather than reinterpreted, because the day/month ambiguity
// is exactly the kind of thing that silently accepts a profile on one machine
// and rejects it on another.
func compareBIOSDate(machineValue, profileValue string) (bool, error) {
	parsedMachine, err := parseSMBIOSDate(machineValue)
	if err != nil {
		return false, err
	}

	operator, operand := splitComparator(profileValue)
	if operator == "" {
		// No comparator at all: the field is a plain equality check, which is
		// almost certainly a mistake in a profile but is not this function's
		// business to reject.
		return false, fmt.Errorf("bios_date: %q has no comparator; expected "+
			"one of >=, >, <=, <, ==", profileValue)
	}

	parsedProfile, err := time.Parse("2006-01-02", operand)
	if err != nil {
		return false, fmt.Errorf("bios_date: %q is not an ISO date: %w", operand, err)
	}

	switch operator {
	case ">=":
		return !parsedMachine.Before(parsedProfile), nil
	case ">":
		return parsedMachine.After(parsedProfile), nil
	case "<=":
		return !parsedMachine.After(parsedProfile), nil
	case "<":
		return parsedMachine.Before(parsedProfile), nil
	case "==":
		return parsedMachine.Equal(parsedProfile), nil
	}
	return false, fmt.Errorf("bios_date: unknown comparator %q", operator)
}

// parseSMBIOSDate parses the machine's firmware date.
//
// SMBIOS defines it as MM/DD/YYYY, so 12/03/2024 is 3 December. Go's time.Parse
// is lenient in exactly the wrong direction here: it accepts 12/31/2023 as a
// valid date when that is correct, but it also accepts a DD/MM value whose first
// component happens to be a valid month. Checking the shape first is what makes
// a rejection a rejection.
func parseSMBIOSDate(value string) (time.Time, error) {
	const layout = "01/02/2006"
	if len(value) != len(layout) {
		return time.Time{}, fmt.Errorf("bios_date: %q is not MM/DD/YYYY", value)
	}
	for i, r := range value {
		if i == 2 || i == 5 {
			if r != '/' {
				return time.Time{}, fmt.Errorf("bios_date: %q is not MM/DD/YYYY", value)
			}
			continue
		}
		if r < '0' || r > '9' {
			return time.Time{}, fmt.Errorf("bios_date: %q is not MM/DD/YYYY", value)
		}
	}
	parsed, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("bios_date: %q: %w", value, err)
	}
	return parsed, nil
}

// splitComparator separates a leading comparison operator from its operand.
func splitComparator(value string) (operator, operand string) {
	for _, candidate := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(value, candidate) {
			return candidate, strings.TrimSpace(strings.TrimPrefix(value, candidate))
		}
	}
	return "", value
}
