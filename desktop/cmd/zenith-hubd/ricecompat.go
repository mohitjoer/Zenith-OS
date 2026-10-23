package main

import (
	"strconv"
	"strings"
)

// riceCompat compares the major/minor version a rice was created with against
// the running Zenith version. Patch and prerelease suffixes do not change the
// rice schema compatibility boundary. Ported from Ryoku's ryostore/compat.
func riceCompat(createdWith, running string) string {
	createdMajor, createdMinor, createdOK := majorMinor(createdWith)
	runningMajor, runningMinor, runningOK := majorMinor(running)
	if !createdOK || !runningOK {
		return "unknown"
	}
	switch {
	case createdMajor == runningMajor && createdMinor == runningMinor:
		return "ok"
	case createdMajor < runningMajor || createdMajor == runningMajor && createdMinor < runningMinor:
		return "older"
	default:
		return "newer"
	}
}

func majorMinor(version string) (int, int, bool) {
	base := strings.TrimPrefix(version, "v")
	if i := strings.IndexByte(base, '-'); i >= 0 {
		base = base[:i]
	}
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil {
		return 0, 0, false
	}
	return major, minor, true
}
