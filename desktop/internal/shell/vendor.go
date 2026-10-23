package shell

// The vendored upstream tree.
//
// Zenith's shell draws some of upstream's surfaces: the bar, the dock, the
// side panel, and the component kit they are made of. Those files are
// vendored — copied out of a pinned upstream commit, byte for byte — and
// they live inside the shell's own tree, at the paths upstream keeps them
// at, so that upstream's components import one another without a single one
// of them being edited.
//
// Two things follow from that, and they pull in opposite directions.
//
// The first is that a vendored file is not Zenith's to edit. The project's
// rule is that a shipped file is never edited in place, and a change to a
// vendored component is a configuration value, an overlay file Zenith
// supplies, or a patch — never a hand edit. So the rules in shell.go, every
// one of which is a rule about Zenith's own code, do not run over the
// vendored tree. A rule that would demand an edit of a vendored file is the
// wrong rule for it: upstream's palette literals are upstream's palette, and
// this project does not get to have an opinion about them.
//
// The second is that "do not edit it" is a convention, and a convention is
// not a check. A vendored file that is edited by hand still loads, still
// draws, and still looks right — until the day it does not, by which time
// the edit is three commits deep and nobody remembers why the bar stopped
// matching upstream. So the tree is checked against the manifest: each
// vendored file's bytes are compared against the blob id its pinned upstream
// commit has, and a file that differs is reported.
//
// That is the same check `shell/vendor/vendor.sh verify` does. It is done
// here as well because the script needs git and a checkout of upstream, and
// because a check that only runs when somebody remembers to run it is not a
// check.

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// vendorManifestPath is where the manifest lives, relative to the shell's
// own directory.
//
// It is the same place in a checkout and in an installed tree, because the
// manifest travels with the components it describes: the tree is installed
// as a whole or not at all.
const vendorManifestPath = "vendor/manifest.yaml"

// vendoredRoots are the directories the vendored upstream tree occupies.
//
// The vendored tree is upstream's namespace inside Zenith's shell: the
// component kit, the bar, the dock, the side panel and the services all
// live under these two directories, at the paths upstream keeps them at.
// Everything else in the shell is Zenith's own code, and only Zenith's own
// code is subject to the shell's rules.
//
// The two are namespaces and not merely directories: upstream's components
// import one another as `qs.modules.*` and `qs.services.*`, and the shell
// root is `qs` — so the whole subtree is one import namespace, and a file
// added under it is a file that changes what upstream's components see.
var vendoredRoots = map[string]bool{
	"modules":      true,
	"services":     true,
	"Services":     true,
	"Ui":           true,
	"components":   true,
	"Barkit":       true,
	"FrameBars":    true,
	"PluginKit":    true,
	"plugins":      true,
	"hub":          true,
	"shot":         true,
	"pin":          true,
	"welcome":      true,
	"reload-cover": true,
	"store":        true,
	"vm":           true,
	"wallpaper-ui": true,
}

// overlayFiles are the files Zenith supplies at upstream's paths.
//
// They are Zenith's: upstream's copies are not vendored, and these are the
// files upstream's components read instead. They are declared here so that
// the drift check knows the difference between a file that is Zenith's and a
// file that is upstream's — a file in the vendored tree that is not in the
// manifest and is not one of these is drift in the other direction, an
// extra file nobody declared.
//
// Their shape is upstream's contract rather than Zenith's own: the
// components read them by upstream's names, and a rule that demanded a
// change would demand breaking that contract. So they are not held to the
// shell's house rules, and the reasons are recorded per file rather than
// left to be guessed at.
//
// The list is the validator's copy of the vendor script's EXCLUDE_FILES: the
// script is what writes the tree, and the two have to say the same thing
// about which paths are Zenith's. TestTheOverlayListMatchesTheVendorScript
// keeps them together.
var overlayFiles = map[string]string{
	"GlobalStates.qml":                   "the vendored components read these flags by name",
	"modules/common/Appearance.qml":      "the vendored components read these tokens by name",
	"modules/common/Config.qml":          "the vendored components read these options by name",
	"modules/common/widgets/AppIcon.qml": "the vendored components read this icon by name",
}

// isVendored reports whether a file belongs to the vendored upstream tree.
//
// The path is relative to the shell's own directory, which is what the walk
// produces and what every rule in this package is written against.
func isVendored(relative string) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	return vendoredRoots[parts[0]]
}

// VendorManifest is the parsed shell/vendor/manifest.yaml.
type VendorManifest struct {
	// Schema is the manifest's format version.
	Schema int
	// Components are the vendored upstream projects.
	Components []VendorComponent
}

// VendorComponent is one vendored upstream project.
type VendorComponent struct {
	// Name is the component's short name, as the vendor script names it.
	Name string
	// UpstreamName is the upstream repository, owner and name.
	UpstreamName string
	// UpstreamURL is where it was fetched from.
	UpstreamURL string
	// UpstreamPath is the path inside the repository the files came from.
	UpstreamPath string
	// PinnedCommit is the commit the files are pinned to.
	PinnedCommit string
	// Licence is the component's licence in the manifest's own words.
	Licence string
	// LicenceFile is the path of the licence text, relative to the
	// repository root.
	LicenceFile string
	// Copyright is the copyright line the component carries.
	Copyright string
	// SPDX is the licence's SPDX identifier.
	SPDX string
	// Notices is where the attribution notices live.
	Notices string
	// Files are the component's files, and the blob ids they must have.
	Files []VendorFile
}

// VendorFile is one file of a component and the blob id it must have.
type VendorFile struct {
	// Path is relative to the shell's own directory.
	Path string
	// Blob is the git blob id the file must have.
	Blob string
}

// set records one of the manifest's scalar fields.
//
// An unknown field is an error rather than something to skip: the manifest
// is generated, and a field this parser does not know about is a field
// whose meaning has changed. Reading it as if it had not is how a check
// ends up passing over the thing it was written to check.
func (c *VendorComponent) set(key, value string) error {
	switch key {
	case "upstream_name":
		c.UpstreamName = value
	case "upstream_url":
		c.UpstreamURL = value
	case "upstream_path":
		c.UpstreamPath = value
	case "pinned_commit":
		c.PinnedCommit = value
	case "licence":
		c.Licence = value
	case "licence_file":
		c.LicenceFile = value
	case "copyright":
		c.Copyright = value
	case "spdx":
		c.SPDX = value
	case "notices":
		c.Notices = value
	default:
		return fmt.Errorf("unknown field %q", key)
	}
	return nil
}

// parseManifest reads the manifest the vendor script writes.
//
// The manifest is generated, so its shape is known: a schema version, and a
// list of components, each with a set of scalar fields and a list of files
// with a path and a blob id.
//
// It is read with a small hand-rolled reader rather than a YAML library
// because the control tools carry no YAML dependency, and because a
// dependency added for one file with six keys and a list is a dependency the
// project carries for every build afterwards. The reader is strict, and
// says so when it meets something it does not understand.
func parseManifest(data []byte) (VendorManifest, error) {
	var manifest VendorManifest
	var component *VendorComponent
	var file *VendorFile
	inFiles := false

	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			key, value, ok := splitManifestEntry(strings.TrimSpace(trimmed[2:]))
			if !ok {
				return manifest, fmt.Errorf("line %d: cannot read %q", i+1, trimmed)
			}
			switch key {
			case "name":
				manifest.Components = append(manifest.Components,
					VendorComponent{Name: value})
				component = &manifest.Components[len(manifest.Components)-1]
				file = nil
				inFiles = false
			case "path":
				if component == nil {
					return manifest, fmt.Errorf("line %d: a file outside any component", i+1)
				}
				component.Files = append(component.Files, VendorFile{Path: value})
				file = &component.Files[len(component.Files)-1]
				inFiles = true
			default:
				return manifest, fmt.Errorf("line %d: unknown list entry %q", i+1, key)
			}
			continue
		}

		key, value, ok := splitManifestEntry(trimmed)
		if !ok {
			return manifest, fmt.Errorf("line %d: cannot read %q", i+1, trimmed)
		}
		switch {
		case key == "components":
			inFiles = false
		case key == "files":
			if component == nil {
				return manifest, fmt.Errorf("line %d: a file list outside any component", i+1)
			}
			inFiles = true
		case inFiles && file != nil:
			if key != "blob" {
				return manifest, fmt.Errorf("line %d: unknown field %q", i+1, key)
			}
			file.Blob = value
		case component != nil:
			if err := component.set(key, value); err != nil {
				return manifest, fmt.Errorf("line %d: %w", i+1, err)
			}
			inFiles = false
		case key == "schema":
			number, err := strconv.Atoi(value)
			if err != nil {
				return manifest, fmt.Errorf("line %d: schema is not a number: %q", i+1, value)
			}
			manifest.Schema = number
		default:
			return manifest, fmt.Errorf("line %d: unknown field %q", i+1, key)
		}
	}

	if len(manifest.Components) == 0 {
		return manifest, fmt.Errorf("the manifest declares no components")
	}
	for _, component := range manifest.Components {
		if len(component.Files) == 0 {
			return manifest, fmt.Errorf("component %q lists no files", component.Name)
		}
	}
	return manifest, nil
}

// splitManifestEntry splits a `key: value` line.
//
// The value may contain a colon — an upstream URL is one — so the line is
// split at the first colon and nowhere else.
func splitManifestEntry(line string) (string, string, bool) {
	index := strings.Index(line, ":")
	if index < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:index])
	if key == "" {
		return "", "", false
	}
	value := strings.Trim(strings.TrimSpace(line[index+1:]), `"'`)
	return key, value, true
}

// blobID is the git blob id of a file's contents.
//
// Git stores a file as a blob: the length of the content, a NUL byte, and
// then the content, hashed with SHA-1. `git hash-object` computes exactly
// this, and the manifest's ids are git's, so this is the same check the
// vendor script does — done without needing git on the machine that runs
// it, and without needing a checkout of upstream to compare against.
func blobID(content []byte) string {
	sum := sha1.New()
	fmt.Fprintf(sum, "blob %d\x00", len(content))
	sum.Write(content)
	return hex.EncodeToString(sum.Sum(nil))
}

// VendorReport is the state of the vendored tree against its manifest.
type VendorReport struct {
	// Manifested says whether there is a vendored tree here at all.
	//
	// A shell with no vendored components in it is not a fault in itself:
	// the Hub window has none, and neither does an image built before the
	// tree was adopted. It is only a fault when the tree is half there.
	Manifested bool
	// Files is how many vendored files were compared against the manifest.
	Files int
	// Drifted is how many of them were not byte-identical to their pinned
	// upstream commit.
	//
	// It is counted apart from the findings because the report says how
	// many files were checked and how many of them were wrong, and a line
	// that says every file is byte-identical while a drift is listed above
	// it is a line that contradicts itself.
	Drifted int
	// Declared is how many of the declared overrides are in the tree.
	//
	// They are counted apart from Files because they are not compared:
	// an override is Zenith's file, and there is no upstream blob for it
	// to match.
	Declared int
	// Components are the components the manifest declares.
	Components []VendorComponent
	// Findings are the drifts: a file that differs, a file that is missing,
	// a file nobody declared, a licence that is not the upstream text.
	Findings []Finding
	// Modules are the QML modules the vendored tree imports that the image
	// has to carry a package for.
	Modules []string
	// Carried are the modules of that list the image's package manifest
	// already declares a package for.
	Carried []string
	// Packages says whether the image's package manifest was readable at
	// all.
	//
	// It is not on an installed machine — it describes how the image is
	// built, and it is not shipped with it — and a report that said "these
	// modules are missing" about a machine whose image carries them would
	// be a lie. So the list is reported as what the tree needs, and the
	// gap is only claimed where the manifest was there to be read.
	Packages bool
}

// Missing returns the modules the image's package manifest does not carry.
//
// It is empty when the manifest could not be read: the gap is then unknown
// rather than empty, and the report says so rather than claiming a
// dependency the image may well have.
func (r VendorReport) Missing() []string {
	if !r.Packages {
		return nil
	}
	carried := map[string]bool{}
	for _, module := range r.Carried {
		carried[module] = true
	}
	var missing []string
	for _, module := range r.Modules {
		if carried[module] {
			continue
		}
		missing = append(missing, module)
	}
	return missing
}

// CheckVendored checks the vendored tree against the manifest.
//
// The findings are faults: a vendored file that differs from its pinned
// commit is a file this project has taken responsibility for and no longer
// holds to, and the fix is to re-sync the tree rather than to edit the
// file.
func CheckVendored(env Env) VendorReport {
	report := VendorReport{}

	manifestPath := env.path(vendorManifestPath)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// A tree with a vendored component in it and no manifest is a
		// fault: the files are there and nothing says what they should be.
		// A tree with neither is not.
		if len(vendoredFiles(env)) > 0 {
			report.Findings = append(report.Findings, Finding{
				Rule: "vendored manifest", Path: manifestPath,
				Detail: "cannot be read (" + err.Error() + "), and the " +
					"vendored tree is not empty; nothing says what those " +
					"files are supposed to be",
			})
		}
		return report
	}
	report.Manifested = true

	manifest, err := parseManifest(data)
	if err != nil {
		report.Findings = append(report.Findings, Finding{
			Rule: "vendored manifest", Path: manifestPath, Detail: err.Error(),
		})
		return report
	}
	report.Components = manifest.Components

	// Every file in the vendored tree, as it is on disk.
	tree := vendoredFiles(env)

	declared := map[string]bool{}
	for _, component := range manifest.Components {
		for _, file := range component.Files {
			declared[file.Path] = true

			// A declared override is Zenith's file, not upstream's, so
			// there is no upstream blob for it to match. The manifest
			// still lists it when it was generated before the override
			// was declared, and the next run of the vendor script drops
			// it.
			if _, overlay := overlayFiles[file.Path]; overlay {
				continue
			}

			report.Files++

			path := env.path(file.Path)
			content, err := os.ReadFile(path)
			if err != nil {
				report.Drifted++
				report.Findings = append(report.Findings, Finding{
					Rule: "vendored absent", Path: path,
					Detail: fmt.Sprintf("the manifest lists it as part of %s, "+
						"and it is not in the tree; the component that "+
						"imports it would fail to load", component.Name),
				})
				continue
			}
			have := blobID(content)
			if have == file.Blob {
				continue
			}
			report.Drifted++
			report.Findings = append(report.Findings, Finding{
				Rule: "vendored drift", Path: path,
				Detail: fmt.Sprintf("differs from %s at %s (expected blob %s, "+
					"found %s); a vendored file is upstream's and is never "+
					"edited in place, so the fix is to re-sync the tree",
					component.Name, shortCommit(component.PinnedCommit),
					file.Blob, have),
			})
		}
	}

	// The other direction: a file that is there and is not in the manifest
	// is drift too. It is upstream's file that nobody pinned, or Zenith's
	// file in upstream's namespace that was never declared, and either way
	// nothing says what it should be.
	for _, path := range tree {
		if declared[path] {
			continue
		}
		if _, overlay := overlayFiles[path]; overlay {
			continue
		}
		report.Findings = append(report.Findings, Finding{
			Rule: "vendored extra", Path: env.path(path),
			Detail: "is in the vendored tree and is not in the manifest; " +
				"upstream does not ship it, so it is either a hand edit " +
				"or a file that was never declared",
		})
	}

	// The overlay files: Zenith's own, at upstream's paths. They are not in
	// the manifest because they are not upstream's, and they are not drift
	// because they are not upstream's. They are still required: a vendored
	// component that imports one cannot resolve it without it.
	for path, reason := range overlayFiles {
		if _, err := os.Stat(env.path(path)); err != nil {
			report.Findings = append(report.Findings, Finding{
				Rule: "vendored overlay", Path: env.path(path),
				Detail: "is missing; it is Zenith's file at an upstream path, " +
					"and " + reason + ", so the vendored components " +
					"cannot resolve it without it",
			})
			continue
		}
		report.Declared++
	}

	report.Findings = append(report.Findings, checkVendoredLicences(env, manifest)...)
	report.Modules, report.Carried, report.Packages = vendoredModuleReport(env, tree)
	return report
}

// vendoredFiles lists the files in the vendored tree, relative to the
// shell's own directory.
func vendoredFiles(env Env) []string {
	var files []string
	for _, root := range []string{"modules/common", "modules/ii", "services"} {
		base := env.path(root)
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// A directory that is not there is a tree without that
				// part of it, which is not a fault here.
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(env.path("."), path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			continue
		}
	}
	sort.Strings(files)
	return files
}

// vendoredLicences are the licence texts this project ships, and the git
// blob ids they have at the pinned upstream commits.
//
// The licences are not in the manifest's file list — a licence is not part
// of a component's files, it is the component's licence — so nothing else
// checks them. A licence file that is edited, truncated or replaced is a
// legal problem and not a build error, and the only thing that catches it
// is a check that says so.
//
// The ids are upstream's own: the vendor script copies each licence out of
// the pinned commit with `git show`, so these are the same bytes upstream
// ships. A re-pin that changes a licence changes these, and that is a
// change a person has to look at rather than one a script may absorb.
var vendoredLicences = map[string]string{
	"GPL-3.0-only": "f288702d2fa16d3cdf0035b15a9fcbc552cd88e7",
	"Apache-2.0":   "261eeb9e9f8b2b4b0d119366dda99c6fd7d35c64",
}

// licenceMarkers are the lines that identify a licence text.
//
// They are a second opinion on the blob id rather than a replacement for
// it: they say which licence the bytes actually are, so a report about a
// licence that is not the right one can say what it is instead.
var licenceMarkers = map[string]string{
	"GPL-3.0-only": "GNU GENERAL PUBLIC LICENSE",
	"Apache-2.0":   "Apache License",
}

// checkVendoredLicences checks each component's licence text.
//
// The licence travels with the component, and the manifest says which
// licence each component is under and where its text is. The text is
// compared against the upstream bytes: a licence that has drifted is a
// legal document nobody looked at.
func checkVendoredLicences(env Env, manifest VendorManifest) []Finding {
	var findings []Finding

	for _, component := range manifest.Components {
		spdx := component.SPDX
		if spdx == "" {
			spdx = component.Licence
		}
		expected, known := vendoredLicences[spdx]
		if !known {
			findings = append(findings, Finding{
				Rule: "vendored licence", Path: env.path(vendorManifestPath),
				Detail: fmt.Sprintf("%s is licensed %s, and this validator "+
					"does not know the text of that licence; the licence "+
					"cannot be checked", component.Name, spdx),
			})
			continue
		}

		path, ok := licencePath(env, component)
		if !ok {
			findings = append(findings, Finding{
				Rule: "vendored licence", Path: env.path(vendorManifestPath),
				Detail: fmt.Sprintf("%s names its licence file as %q, which "+
					"is not in the vendor directory beside the manifest",
					component.Name, component.LicenceFile),
			})
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			findings = append(findings, Finding{
				Rule: "vendored licence", Path: path,
				Detail: fmt.Sprintf("is missing; %s is licensed %s and the "+
					"licence travels with it", component.Name, spdx),
			})
			continue
		}

		have := blobID(content)
		if have == expected {
			continue
		}
		detail := fmt.Sprintf("is not the %s text as %s ships it (expected "+
			"blob %s, found %s); a licence is a legal document, and one "+
			"that has changed is a thing a person has to look at",
			spdx, component.Name, expected, have)
		if marker := licenceMarkers[spdx]; !strings.Contains(string(content), marker) {
			detail += fmt.Sprintf("; it does not even carry the %q line that "+
				"identifies the %s text", marker, spdx)
		}
		findings = append(findings, Finding{
			Rule: "vendored licence text", Path: path, Detail: detail,
		})
	}
	return findings
}

// licencePath is where a component's licence text is.
//
// The manifest records the licence file relative to the repository root
// ("shell/vendor/COPYING"), and the vendor script writes it next to the
// manifest itself. Resolving it against the manifest's own directory puts
// it in the same place in a checkout and in an installed tree, where the
// shell is at /usr/share/zenith/shell rather than at shell/.
func licencePath(env Env, component VendorComponent) (string, bool) {
	directory := filepath.Dir(component.LicenceFile)
	if filepath.Base(directory) != filepath.Base(filepath.Dir(vendorManifestPath)) {
		return "", false
	}
	return env.path(filepath.Join(
		filepath.Dir(vendorManifestPath), filepath.Base(component.LicenceFile))), true
}

// baseRuntimeModules are the QML modules that arrive with the Qt Quick
// runtime the image already carries.
//
// qml6-module-qtquick depends on qml6-module-qtqml, and that is where
// QtQml and QtQml.Models come from. They are not extra packages the image
// has to add, so reporting them would be reporting a dependency the image
// already has.
//
// QtCore is not in this list, and the reason is worth recording: the
// image's own package manifest says that qml6-module-qtcore is not a
// dependency of anything the image already installs, and names it
// explicitly. Leaning on a transitive dependency here would be leaning on
// how Qt happens to be packaged rather than on anything the image
// declares.
var baseRuntimeModules = map[string]bool{
	"QtQml":        true,
	"QtQml.Models": true,
}

// controlStyles are the Qt Quick Controls styles.
//
// A style is a plugin of the Controls module rather than a module of its
// own: QtQuick.Controls.Material is the Material style, and the archive
// ships it inside the package that carries QtQuick.Controls rather than in
// a package of its own. So a style is carried by the module it is a style
// of, and a module that is not a style has to be named by a package of its
// own.
var controlStyles = map[string]bool{
	"basic":        true,
	"fusion":       true,
	"imagine":      true,
	"material":     true,
	"universal":    true,
	"native":       true,
	"macos":        true,
	"windows":      true,
	"fluentwinui":  true,
	"fluentwinui3": true,
}

// imageModules are the QML modules the image carries.
//
// The closed list of modules the shell may import is the record of what the
// image carries: every Qt module on it is a package the image installs, and
// every Quickshell module on it is a module Quickshell itself provides.
func imageModules() map[string]bool {
	modules := map[string]bool{}
	for module := range allowedImports {
		if module == "qs" || strings.HasPrefix(module, "Quickshell") {
			continue
		}
		modules[module] = true
	}
	for module := range baseRuntimeModules {
		modules[module] = true
	}
	return modules
}

// vendoredModuleReport returns the QML modules the vendored tree imports
// that the image has to carry, and the ones of those the image's package
// manifest already declares.
//
// This is the import rule run over the vendored tree, and it is reported
// rather than failed. The rule is closed for Zenith's own code, and it
// stays closed: a vendored file importing a module the image does not carry
// is a fact about the vendored tree and about the image's package list, and
// the fix is a package in the image rather than an edit of a file that is
// not ours to edit.
func vendoredModuleReport(env Env, files []string) (needed, carried []string, packages bool) {
	_ = imageModules()
	declared, packages := declaredQMLModules(env.Root)

	seen := map[string]bool{}
	for _, file := range files {
		if !strings.HasSuffix(file, ".qml") {
			continue
		}
		body, err := source(env, file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(body, "\n") {
			match := importLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			module := match[1]
			if seen[module] {
				continue
			}
			seen[module] = true

			// The shell's own tree provides its own modules, and
			// Quickshell provides its own.
			if module == "qs" || strings.HasPrefix(module, "qs.") {
				continue
			}
			if module == "Quickshell" || strings.HasPrefix(module, "Quickshell.") {
				continue
			}

			needed = append(needed, module)
			if packages && moduleCarried(declared, module) {
				carried = append(carried, module)
			}
		}
	}
	sort.Strings(needed)
	sort.Strings(carried)
	return needed, carried, packages
}

// moduleCarried reports whether the image's package manifest declares a
// package that carries a QML module.
//
// The package is named after the module, with the dots written as dashes:
// QtQuick.Controls is carried by qml6-module-qtquick-controls. A style is
// carried by the module it is a style of, because the archive ships the
// styles inside the controls package rather than in packages of their own.
func moduleCarried(declared map[string]bool, module string) bool {
	if declared[moduleKey(module)] {
		return true
	}
	segments := strings.Split(module, ".")
	if len(segments) < 2 {
		return false
	}
	if !controlStyles[strings.ToLower(segments[len(segments)-1])] {
		return false
	}
	parent := strings.Join(segments[:len(segments)-1], ".")
	return declared[moduleKey(parent)]
}

// moduleKey folds a QML module name into the shape an Ubuntu package name
// carries it in.
//
// The archive names the package that carries a module after the module with
// its dots written as dashes: QtQuick.Controls is carried by
// qml6-module-qtquick-controls. The comparison is case-insensitive because
// the package names are lower case and the module names are not.
func moduleKey(module string) string {
	return strings.ToLower(strings.ReplaceAll(module, ".", "-"))
}

// declaredQMLModules returns the QML modules the image's package manifest
// declares a package for.
//
// The manifest is a YAML file this project controls, and the only thing
// needed from it is the list of package names: a package called
// qml6-module-<name> carries the QML module <name>.
//
// It is read with a narrow reader rather than a YAML library, for the same
// reason the manifest is: the control tools carry no YAML dependency, and
// one added for a list of names is one the project carries for every build
// afterwards.
//
// The second return value says whether the manifest was there at all. It is
// not on an installed machine — it describes how the image is built, and it
// is not shipped with it — and a report that said "these modules are
// missing" about a machine whose image carries them would be a lie.
func declaredQMLModules(root string) (map[string]bool, bool) {
	data, err := os.ReadFile(filepath.Join(root, "system", "packages.yaml"))
	if err != nil {
		return nil, false
	}

	modules := map[string]bool{}
	for _, name := range declaredPackages(string(data)) {
		if !strings.HasPrefix(name, "qml6-module-") {
			continue
		}
		modules[strings.TrimPrefix(name, "qml6-module-")] = true
	}
	return modules, true
}

// declaredPackages reads the package names out of the image's package
// manifest.
//
// The manifest has comments, block scalars and nested maps, and the only
// thing needed from it is the list of names under each `packages:` key. A
// list item is a line that starts with a dash and a name, and it belongs
// to the `packages:` list above it — the reader tracks which list it is in
// and how deep that list is, so a nested list of something else is not
// mistaken for a package.
func declaredPackages(body string) []string {
	var names []string
	inPackages := false
	depth := 0

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if strings.HasPrefix(trimmed, "- ") {
			if !inPackages || indent != depth+2 {
				continue
			}
			name := strings.TrimSpace(trimmed[2:])
			if name == "" {
				continue
			}
			names = append(names, name)
			continue
		}

		// A key. One at or above the list's own level ends the list.
		if inPackages && indent <= depth {
			inPackages = false
		}
		if !strings.HasSuffix(trimmed, ":") {
			continue
		}
		if strings.TrimSuffix(trimmed, ":") == "packages" {
			inPackages = true
			depth = indent
		}
	}
	return names
}

// shortCommit shortens a commit id for a message.
//
// The whole id is in the manifest; the message needs enough of it to
// identify the commit and to paste into a fetch.
func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
