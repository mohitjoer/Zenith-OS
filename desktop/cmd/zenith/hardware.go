package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"zenith/hardware/lint"
	"zenith/hardware/match"
	"zenith/hardware/probe"
	"zenith/hardware/redact"
	"zenith/hardware/report"
)

// profilesDirOverride lets --profiles point at a different database. It is a
// package variable rather than a global flag because each subcommand declares
// its own flags and a flag registered on no FlagSet is a flag nobody can set.
var profilesDirOverride string

// resolveProfilesDir finds the profile database.
//
// The default looks alongside the installed binary first, so an installed
// zenith finds the database it shipped with, and falls back to the repository
// layout so the tool runs from a checkout with nothing installed.
func resolveProfilesDir() string {
	if profilesDirOverride != "" {
		return profilesDirOverride
	}
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(filepath.Dir(executable)),
			"share", "zenith", "hardware", "profiles")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("system", "hardware", "profiles")
}

// saltPath is where the report token key lives: a Zenith-owned path under the
// user's state directory, never a home directory written by a package.
func saltPath() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "zenith", "hardware", "report-salt")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "zenith", "hardware", "report-salt")
	}
	return filepath.Join(os.TempDir(), "zenith-report-salt")
}

// hostContext builds the redaction sweep's context from the live machine.
//
// The sweep cannot recognise a hostname, a username or a machine-id by shape, so
// it has to be told them. Gathering them here rather than inside package report
// is what keeps that package free of process-global access, and therefore
// testable.
func hostContext(rawDMI map[string]string, identifiers []string) redact.Context {
	context := redact.Context{
		RawDMI:             rawDMI,
		AllowedIdentifiers: identifiers,
	}

	if hostname, err := os.Hostname(); err == nil {
		context.Hostname = hostname
	}
	context.User = os.Getenv("USER")
	context.Logname = os.Getenv("LOGNAME")

	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(path); err == nil {
			context.MachineIDs = append(context.MachineIDs, strings.TrimSpace(string(data)))
		}
	}
	return context
}

// rawDMI reads the unclassified DMI dump.
//
// The sweep needs the raw values to compare against and only the raw dump has
// them. That is rule 1, and it is what makes redaction a policy rather than a
// list of regexes.
func rawDMI(root string) map[string]string {
	raw := map[string]string{}
	dir := filepath.Join(root, "class", "dmi", "id")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return raw
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		raw[entry.Name()] = strings.TrimRight(string(data), "\n")
	}
	return raw
}

// identifiers collects the PCI and USB identifiers a report may carry.
//
// Without this the sweep's MAC rule fires on the report's own evidence: a USB
// idVendor of 0x0011 and a device of 0x2233 reported as "0011:2233" sits close
// enough to a MAC to matter, and a redaction check that is disabled by the hex
// data it exists to protect is not a check.
func identifiers(devices []probe.Device, usbDevices []probe.USBDevice) []string {
	found := make([]string, 0, len(devices)*2+len(usbDevices)*2)
	for _, device := range devices {
		found = append(found, device.Vendor+":"+device.Device)
	}
	for _, device := range usbDevices {
		found = append(found, device.IDVendor+":"+device.IDProduct)
	}
	return found
}

// detect runs the probes and returns everything the subcommands need.
func detect(root string) (report.Input, error) {
	fs, err := probe.New(root)
	if err != nil {
		return report.Input{}, err
	}

	input := report.Input{KernelFlavour: currentKernelFlavour(root)}

	if input.DMI, err = probe.DMI(fs); err != nil {
		return input, fmt.Errorf("reading DMI: %w", err)
	}

	// A machine with no SMBIOS answers in its device tree instead. Without this
	// an ARM laptop could never match a profile at all.
	if tree, treeErr := probe.DeviceTree(fs); treeErr == nil && tree.Present {
		mergeDeviceTree(&input.DMI, tree)
	}

	rules, err := probe.AliasRules(filepath.Join(fs.Root(), "modules.alias"))
	if err != nil {
		return input, fmt.Errorf("reading modules.alias: %w", err)
	}
	if input.PCI, err = probe.PCI(fs, rules); err != nil {
		return input, fmt.Errorf("reading PCI: %w", err)
	}
	if input.USB, err = probe.USB(fs); err != nil {
		return input, fmt.Errorf("reading USB: %w", err)
	}
	if input.ACPI, err = probe.ACPI(fs); err != nil {
		return input, fmt.Errorf("reading ACPI: %w", err)
	}
	if input.EDID, err = probe.EDID(fs); err != nil {
		return input, fmt.Errorf("reading EDID: %w", err)
	}

	input.RawDMI = rawDMI(root)
	input.Salt, err = redact.LoadOrCreateSalt(saltPath())
	if err != nil {
		return input, err
	}
	input.Host = hostContext(input.RawDMI, identifiers(input.PCI, input.USB))

	return input, nil
}

// mergeDeviceTree projects a device tree onto the match-key fields.
//
// DMI wins where both answer: SMBIOS is the more specific source, and a machine
// carrying both is x86 with a device tree rather than an ARM board.
func mergeDeviceTree(fields *probe.Fields, tree probe.Tree) {
	for key, value := range tree.IdentityFields() {
		switch key {
		case "product_name":
			if fields.ProductName == "" {
				fields.ProductName = value
			}
		case "sys_vendor":
			if fields.SysVendor == "" {
				fields.SysVendor = value
			}
		}
	}
	if fields.ProductName != "" || fields.SysVendor != "" {
		fields.Present = true
	}
}

// osreleasePath is where the kernel publishes its release string. It lives in
// /proc rather than /sys, so it is the one place this tool reads outside the
// probe root — and it is a kernel string, not a hardware attribute.
const osreleasePath = "/proc/sys/kernel/osrelease"

// currentKernelFlavour returns the kernel's flavour for the tree being read, or
// "" when the tree cannot say.
//
// Only the flavour is ever returned. A patch version is deliberately not carried
// out of detection: a profile that pins one stops matching at the next security
// update, and the whole job here is to choose a flavour and not a build.
//
// The root argument matters more than it looks. osrelease lives in /proc, which a
// captured machine does not carry, so reading it unconditionally would name the
// *host's* kernel in a profile generated from a fixture — and --root promises to
// read the tree it was given and nothing else. A tree that cannot report its
// kernel reports nothing, and the renderer says so rather than guessing.
func currentKernelFlavour(root string) string {
	if filepath.Clean(root) != filepath.Clean(probe.RootDefault) {
		return ""
	}

	data, err := os.ReadFile(osreleasePath)
	if err != nil {
		return ""
	}

	release := strings.TrimSpace(string(data))
	if release == "" {
		return ""
	}

	// The flavour is the trailing component: "6.8.0-31-generic" → "generic".
	index := strings.LastIndex(release, "-")
	if index <= 0 {
		return ""
	}
	candidate := release[index+1:]
	if candidate == "" || strings.ContainsAny(candidate, "0123456789") {
		// A flavour never contains a digit, and a kernel release always has at
		// least one version component before it.
		return ""
	}
	return candidate
}

// machineFields flattens detection into the field map matching consumes.
//
// Only the match vocabulary is carried. Matching must depend on nothing that
// varies between two runs of the same machine, or resolution stops being
// reproducible and a golden test means nothing.
func machineFields(fields probe.Fields) map[string]string {
	return map[string]string{
		"sys_vendor":      fields.SysVendor,
		"product_name":    fields.ProductName,
		"product_version": fields.ProductVersion,
		"product_family":  fields.ProductFamily,
		"product_sku":     fields.ProductSKU,
		"board_vendor":    fields.BoardVendor,
		"board_name":      fields.BoardName,
		"board_version":   fields.BoardVersion,
		"chassis_type":    fields.ChassisType,
		"bios_vendor":     fields.BIOSVendor,
		"bios_version":    fields.BIOSVersion,
		"bios_date":       fields.BIOSDate,
	}
}

// hardwareReport prints this machine, redacted.
func hardwareReport(args []string) int {
	set := newFlags("report", reportUsage)
	var (
		format      = set.String("format", "text", "output shape: text, json or profile")
		deep        = set.Bool("deep", false, "include ACPI-derived facts")
		profileOnly = set.Bool("profile", false, "emit a candidate profile")
		raw         = set.Bool("raw", false, "unredacted dump, local only")
		out         = set.String("out", "", "write to a file instead of stdout")
		root        = set.String("root", probe.RootDefault, "sysfs root to read")
		profilesDir = set.String("profiles", "", "profile database directory")
	)
	if err := set.Parse(args); err != nil {
		return exitUsage
	}
	profilesDirOverride = *profilesDir

	// --raw writes a file containing serials. It refuses outright when CI is
	// set, so a pipeline cannot produce a file meant for humans to keep local
	// and then attach it somewhere.
	if *raw {
		if report.RawPathRefusesUnderCI(os.Getenv("CI")) {
			fmt.Fprintln(os.Stderr,
				"zenith hardware report --raw refuses to run when CI is set: the "+
					"output contains serials and is for local debugging only.")
			return exitUsage
		}
		return writeRaw(*root, *out)
	}

	shape := report.Format(*format)
	if *profileOnly {
		shape = report.FormatProfile
	}

	input, err := detect(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware report:", err)
		return exitFail
	}

	rendered, err := report.Render(input, report.Options{Format: shape, Deep: *deep})
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware report:", err)
		return exitFail
	}
	return write(rendered, *out)
}

// write prints or writes rendered output.
func write(rendered []byte, path string) int {
	if path == "" {
		os.Stdout.Write(rendered)
		return exitOK
	}
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "zenith: writing "+path+":", err)
		return exitFail
	}
	fmt.Fprintln(os.Stderr, "wrote "+path)
	return exitOK
}

// rawDumpSuffix is the suffix the repository's .gitignore already excludes, so
// a raw dump cannot be committed by accident.
const rawDumpSuffix = ".hardware.raw.yaml"

// writeRaw dumps this machine's DMI for local debugging.
//
// This is the one output that is deliberately not redacted, which is why it is
// a file with a loud name rather than a flag on a report. It is written 0600,
// printed with an absolute path, and refused outright under CI.
func writeRaw(root, outPath string) int {
	if outPath == "" {
		outPath = "zenith" + rawDumpSuffix
	}
	if !strings.HasSuffix(outPath, ".yaml") {
		outPath += rawDumpSuffix
	}

	raw := rawDMI(root)
	if len(raw) == 0 {
		fmt.Fprintln(os.Stderr, "zenith hardware report --raw: no DMI at "+root)
		return exitFail
	}

	var b strings.Builder
	b.WriteString("# UNREDACTED local dump. Contains serials and asset tags.\n")
	b.WriteString("# " + report.RawPathWarning + "\n")
	b.WriteString("dmi:\n")
	// Sorted so two dumps of the same machine are diffable.
	for _, name := range sortedDMIKeys(raw) {
		fmt.Fprintf(&b, "  %s: %q\n", name, raw[name])
	}

	absolute, err := filepath.Abs(outPath)
	if err != nil {
		absolute = outPath
	}
	if err := os.WriteFile(absolute, []byte(b.String()), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "zenith: writing "+absolute+":", err)
		return exitFail
	}

	fmt.Fprintln(os.Stderr, absolute)
	fmt.Fprintln(os.Stderr, report.RawPathWarning)
	return exitOK
}

func sortedDMIKeys(raw map[string]string) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// hardwareMatch reports which profile claims this machine.
func hardwareMatch(args []string) int {
	set := newFlags("match", matchUsage)
	var (
		root        = set.String("root", probe.RootDefault, "sysfs root to read")
		profilesDir = set.String("profiles", "", "profile database directory")
	)
	if err := set.Parse(args); err != nil {
		return exitUsage
	}
	profilesDirOverride = *profilesDir

	dir := resolveProfilesDir()
	profiles, err := lint.LoadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware match:", err)
		return exitFail
	}
	if len(profiles) == 0 {
		fmt.Fprintln(os.Stderr, "zenith hardware match: no profiles in "+dir)
		return exitNoMatch
	}

	input, err := detect(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware match:", err)
		return exitFail
	}

	fields := machineFields(input.DMI)
	if input.DMI.Present {
		// A machine that answered only through a device tree still has an
		// identity; matching just needs the same field names.
		fields = mergeMissing(fields, input)
	}

	resolution := match.Resolve(match.NewMachine(fields), lint.ToMatchProfiles(profiles))

	printResolution(resolution)
	if resolution.ProfileID == "" {
		return exitNoMatch
	}
	return exitOK
}

// mergeMissing folds a device-tree-derived identity into the match fields.
func mergeMissing(fields map[string]string, input report.Input) map[string]string {
	merged := make(map[string]string, len(fields))
	for key, value := range fields {
		merged[key] = value
	}
	for key, value := range machineFields(input.DMI) {
		if merged[key] == "" {
			merged[key] = value
		}
	}
	return merged
}

func printResolution(resolution match.Resolution) {
	fmt.Printf("profile:  %s\n", orNone(resolution.ProfileID))
	fmt.Printf("tier:     %d\n", resolution.Tier)
	fmt.Printf("kernel:   %s\n", orNone(resolution.Kernel))

	if resolution.Conflict != nil {
		fmt.Printf("conflict: %s disagree on %s\n",
			strings.Join(resolution.Conflict.Profiles, " and "),
			strings.Join(resolution.Conflict.Keys, ", "))
	}
	if resolution.NoDMI {
		fmt.Println("note:     this machine has no SMBIOS table and no device tree, " +
			"so no profile can claim it")
	}
	if len(resolution.MatchedFields) > 0 {
		fmt.Printf("matched:  %s\n", strings.Join(resolution.MatchedFields, ", "))
	}
	if len(resolution.AlsoMatched) > 0 {
		fmt.Println("also matched, not selected:")
		for _, candidate := range resolution.AlsoMatched {
			fmt.Printf("  %s (tier %d, specificity %d/%d/%d)\n",
				candidate.ID, candidate.Tier,
				candidate.Specificity.Identity,
				candidate.Specificity.Narrowing,
				candidate.Specificity.Comparator)
		}
	}
	if resolution.Reason != "" {
		fmt.Printf("reason:   %s\n", resolution.Reason)
	}
}

// hardwareLintProfiles checks the database.
func hardwareLintProfiles(args []string) int {
	set := newFlags("lint-profiles", lintUsage)
	var (
		strict      = set.Bool("strict", false, "fail on warnings as well as errors")
		profilesDir = set.String("profiles", "", "profile database directory")
	)
	if err := set.Parse(args); err != nil {
		return exitUsage
	}
	profilesDirOverride = *profilesDir

	dir := resolveProfilesDir()
	profiles, err := lint.LoadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware lint-profiles:", err)
		return exitFail
	}

	findings, err := lint.Run(profiles)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware lint-profiles:", err)
		return exitFail
	}

	if len(findings) == 0 {
		fmt.Printf("%d profile(s) in %s: no findings\n", len(profiles), dir)
		return exitOK
	}

	var errors, warnings int
	for _, finding := range findings {
		fmt.Fprintln(os.Stderr, finding)
		if finding.Severity == lint.Error {
			errors++
		} else {
			warnings++
		}
	}
	fmt.Fprintf(os.Stderr, "\n%d error(s), %d warning(s)\n", errors, warnings)

	switch {
	case errors > 0:
		return exitFail
	case warnings > 0 && *strict:
		return exitFail
	}
	return exitOK
}

// hardwareCapture writes this machine's sysfs tree to a directory, so that a
// fixture is a machine.
func hardwareCapture(args []string) int {
	set := newFlags("capture", captureUsage)
	var (
		out  = set.String("out", "", "directory to write (required)")
		root = set.String("root", probe.RootDefault, "sysfs root to read")
	)
	if err := set.Parse(args); err != nil {
		return exitUsage
	}
	if *out == "" {
		fmt.Fprintln(os.Stderr, "zenith hardware capture: --out is required")
		return exitUsage
	}

	captured, skipped, err := capture(*root, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "zenith hardware capture:", err)
		return exitFail
	}
	fmt.Printf("wrote %d file(s) to %s\n", captured, *out)
	if skipped > 0 {
		// Saying what was dropped is the point: a contributor has to know the
		// fixture cannot round-trip the serials or they will assume it can.
		fmt.Printf("skipped %d identifying value(s); a capture cannot contain a "+
			"serial, so a fixture is not the raw machine\n", skipped)
	}
	return exitOK
}

func orNone(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}
