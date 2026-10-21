package shell

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The vendored tree is checked against the manifest it was pinned from.
//
// The tests below are about the two halves of that: the check has to notice
// a file that has drifted, and it has to stay silent on a tree that has
// not. A check that only ever fires is as useless as one that never does.

// licenceText is the repository's own GPL-3.0 text.
//
// A fixture with an invented licence would fail the licence check for a
// reason that has nothing to do with what the test is about, so the real
// file is used.
func licenceText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "vendor", "COPYING"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// manifestText writes a manifest for a set of vendored files.
//
// It is the shape the vendor script writes, with the blob ids computed the
// same way, so that a fixture is a tree the vendor script could have
// produced.
func manifestText(t *testing.T, files map[string]string) string {
	t.Helper()

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("schema: 1\n")
	out.WriteString("components:\n")
	out.WriteString("  - name: ii\n")
	out.WriteString("    upstream_name: end-4/dots-hyprland\n")
	out.WriteString("    upstream_url: https://github.com/end-4/dots-hyprland.git\n")
	out.WriteString("    upstream_path: dots/.config/quickshell/ii\n")
	out.WriteString("    pinned_commit: 2f2a376fdb2ad738bf55f693dd0b474b64bb0ab8\n")
	out.WriteString("    licence: GPL-3.0-only\n")
	out.WriteString("    licence_file: shell/vendor/COPYING\n")
	out.WriteString("    copyright: Copyright (C) the end-4/dots-hyprland contributors\n")
	out.WriteString("    spdx: GPL-3.0-only\n")
	out.WriteString("    notices: shell/vendor/notices.md\n")
	out.WriteString("    files:\n")
	for _, name := range names {
		out.WriteString("      - path: " + name + "\n")
		out.WriteString("        blob: " + blobID([]byte(files[name])) + "\n")
	}
	return out.String()
}

// vendorTree writes a shell tree with a vendored component in it.
//
// The overlay files are written too, because they are Zenith's own files at
// upstream's paths and the tree is not whole without them.
func vendorTree(t *testing.T, vendored map[string]string) Env {
	t.Helper()

	files := map[string]string{}
	for name, body := range vendored {
		files[name] = body
	}
	for path := range overlayFiles {
		files[path] = "import Quickshell\nItem {}\n"
	}
	files["vendor/COPYING"] = licenceText(t)
	files["vendor/manifest.yaml"] = manifestText(t, vendored)

	return tree(t, files)
}

// findingRules lists the rules a finding came from.
func findingRules(findings []Finding) []string {
	var out []string
	for _, finding := range findings {
		out = append(out, finding.Rule)
	}
	return out
}

// TestTheVendoredTreeHasNotDrifted runs the drift check against the
// repository's own vendored tree.
//
// This is the check that makes the tree trustworthy. Every other rule in
// this package assumes the vendored files are upstream's, byte for byte;
// a file that has been edited in place breaks that assumption quietly, and
// the first symptom is a component that behaves slightly differently from
// the one the pin says it is.
func TestTheVendoredTreeHasNotDrifted(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell", EntryPoint: "shell.qml"}
	report := CheckVendored(env)

	if !report.Manifested {
		t.Fatal("the repository has no vendored manifest")
	}
	for _, finding := range report.Findings {
		t.Errorf("%s %s: %s", finding.Rule, finding.Path, finding.Detail)
	}
	if report.Files == 0 {
		t.Error("no vendored file was checked; the check is passing over " +
			"the tree rather than over it")
	}

	// The count has to be the number of files that were compared, and not
	// the number of lines in the manifest: a declared override is Zenith's
	// file and has no upstream blob to match.
	manifest, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "vendor", "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	listed := 0
	for _, component := range parsed.Components {
		listed += len(component.Files)
	}
	overrides := 0
	for _, component := range parsed.Components {
		for _, file := range component.Files {
			if _, overlay := overlayFiles[file.Path]; overlay {
				overrides++
			}
		}
	}
	if report.Files != listed-overrides {
		t.Errorf("%d file(s) checked, and the manifest lists %d file(s) of "+
			"which %d are declared overrides; the count has to be what was "+
			"compared", report.Files, listed, overrides)
	}
}

// TestTheManifestIsRead covers the manifest reader against the real
// manifest.
//
// The reader is hand-written, so it is the one part of this that can be
// wrong in a way that reads as a clean tree: a manifest it misreads is a
// manifest it checks nothing against.
func TestTheManifestIsRead(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "vendor", "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := parseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != 1 {
		t.Errorf("schema = %d, want 1", manifest.Schema)
	}
	if len(manifest.Components) == 0 {
		t.Fatal("the manifest declares no components")
	}

	for _, component := range manifest.Components {
		if component.Name == "" {
			t.Error("a component has no name")
		}
		if component.PinnedCommit == "" {
			t.Errorf("%s has no pinned commit", component.Name)
		}
		if component.SPDX == "" {
			t.Errorf("%s has no SPDX identifier", component.Name)
		}
		if component.LicenceFile == "" {
			t.Errorf("%s has no licence file", component.Name)
		}
		if len(component.Files) == 0 {
			t.Errorf("%s lists no files", component.Name)
		}
		for _, file := range component.Files {
			if file.Path == "" || file.Blob == "" {
				t.Errorf("%s lists a file with no path or no blob", component.Name)
			}
			if len(file.Blob) != 40 {
				t.Errorf("%s: %s has blob %q, which is not a git blob id",
					component.Name, file.Path, file.Blob)
			}
		}
	}
}

// TestAVendoredFileThatDiffersIsFound covers the drift itself.
//
// The failure this exists for is a hand edit of a vendored file: the file
// still loads, still draws, and still looks right, and the project has
// quietly stopped shipping upstream's version of it.
func TestAVendoredFileThatDiffersIsFound(t *testing.T) {
	original := "import Quickshell\nItem {}\n"
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": original,
	})
	// The file on disk is edited; the manifest still records upstream's.
	if err := os.WriteFile(filepath.Join(env.Root, "shell", "modules", "common", "Widget.qml"),
		[]byte("import Quickshell\nItem { /* edited by hand */ }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := CheckVendored(env)
	found := false
	for _, finding := range report.Findings {
		if finding.Rule == "vendored drift" && strings.Contains(finding.Path, "Widget.qml") {
			found = true
		}
	}
	if !found {
		t.Errorf("a vendored file that was edited was not found: %v",
			findingRules(report.Findings))
	}
}

// TestAMissingVendoredFileIsFound covers the other half of the drift: a
// file the manifest lists that is not in the tree.
//
// The component that imports it fails to load, and the failure is at run
// time on the machine, with nothing in the journal saying which file.
func TestAMissingVendoredFileIsFound(t *testing.T) {
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": "import Quickshell\nItem {}\n",
	})
	if err := os.Remove(filepath.Join(env.Root, "shell", "modules", "common", "Widget.qml")); err != nil {
		t.Fatal(err)
	}

	report := CheckVendored(env)
	found := false
	for _, finding := range report.Findings {
		if finding.Rule == "vendored absent" && strings.Contains(finding.Path, "Widget.qml") {
			found = true
		}
	}
	if !found {
		t.Errorf("a missing vendored file was not found: %v",
			findingRules(report.Findings))
	}
}

// TestAFileNobodyDeclaredIsFound covers drift in the other direction.
//
// A file in the vendored tree that the manifest does not list is either
// upstream's file that nobody pinned or a file somebody added without
// saying so. Either way nothing says what it should be, and the next sync
// deletes it.
func TestAFileNobodyDeclaredIsFound(t *testing.T) {
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": "import Quickshell\nItem {}\n",
	})
	extra := filepath.Join(env.Root, "shell", "modules", "common", "Extra.qml")
	if err := os.WriteFile(extra, []byte("import Quickshell\nItem {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := CheckVendored(env)
	found := false
	for _, finding := range report.Findings {
		if finding.Rule == "vendored extra" && strings.Contains(finding.Path, "Extra.qml") {
			found = true
		}
	}
	if !found {
		t.Errorf("a file nobody declared was not found: %v",
			findingRules(report.Findings))
	}
}

// TestTheOverlayFilesAreRequired covers the files Zenith supplies.
//
// They are Zenith's, not upstream's, so they are not in the manifest and
// they are not drift. They are still required: a vendored component that
// imports one cannot resolve it without it, and the failure is at run time
// on the machine.
func TestTheOverlayFilesAreRequired(t *testing.T) {
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": "import Quickshell\nItem {}\n",
	})
	missing := filepath.Join(env.Root, "shell", "modules", "common", "Config.qml")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}

	report := CheckVendored(env)
	found := false
	for _, finding := range report.Findings {
		if finding.Rule == "vendored overlay" && strings.Contains(finding.Path, "Config.qml") {
			found = true
		}
	}
	if !found {
		t.Errorf("a missing overlay was not found: %v", findingRules(report.Findings))
	}
}

// TestTheOverlayFilesAreNotDrift covers the other half: an overlay that is
// present and is not upstream's is not a finding.
//
// An overlay is Zenith's file. It differs from upstream by definition — it
// is the whole point of it — so a check that reported it as drift would
// report every overlay, every time.
func TestTheOverlayFilesAreNotDrift(t *testing.T) {
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": "import Quickshell\nItem {}\n",
	})

	report := CheckVendored(env)
	for _, finding := range report.Findings {
		if _, overlay := overlayFiles[filepath.Base(finding.Path)]; overlay {
			t.Errorf("an overlay was reported as drift: %s", finding)
		}
	}
	if report.Declared != len(overlayFiles) {
		t.Errorf("%d declared override(s) found, want %d", report.Declared,
			len(overlayFiles))
	}
}

// TestTheLicenceTextIsChecked covers the licences.
//
// The licences travel with the components and are not in the manifest, so
// nothing else looks at them. A licence file that has been edited is a
// legal document nobody looked at, and it is not a build error.
func TestTheLicenceTextIsChecked(t *testing.T) {
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": "import Quickshell\nItem {}\n",
	})

	// The repository's own text passes.
	report := CheckVendored(env)
	for _, finding := range report.Findings {
		if strings.HasPrefix(finding.Rule, "vendored licence") {
			t.Errorf("the licence was reported as wrong: %s", finding)
		}
	}

	// A licence that has been truncated is not the licence.
	licence := filepath.Join(env.Root, "shell", "vendor", "COPYING")
	if err := os.WriteFile(licence, []byte("GNU GENERAL PUBLIC LICENSE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report = CheckVendored(env)
	found := false
	for _, finding := range report.Findings {
		if finding.Rule == "vendored licence text" {
			found = true
		}
	}
	if !found {
		t.Errorf("a licence that is not the upstream text was not found: %v",
			findingRules(report.Findings))
	}
}

// TestTheLicenceFilesAreTheUpstreamTexts runs the licence check against the
// repository's own licence files.
//
// The bytes are the ones upstream ships at the pinned commits, and the
// check compares against them byte for byte.
func TestTheLicenceFilesAreTheUpstreamTexts(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell"}
	for spdx, expected := range vendoredLicences {
		path, ok := licencePath(env, VendorComponent{
			LicenceFile: "shell/vendor/" + licenceFileName(spdx),
			SPDX:        spdx,
		})
		if !ok {
			t.Errorf("%s: the licence file is not in the vendor directory", spdx)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %s", spdx, err)
			continue
		}
		if have := blobID(data); have != expected {
			t.Errorf("%s: %s has blob %s, want %s", spdx, path, have, expected)
		}
	}
}

// licenceFileName is the file a licence travels in.
func licenceFileName(spdx string) string {
	if spdx == "Apache-2.0" {
		return "LICENSE.apache-2.0"
	}
	return "COPYING"
}

// TestTheBlobIdIsGits covers the blob computation.
//
// The manifest's ids are git's, and git computes them as the SHA-1 of the
// content with a header that says how long the content is. Both values
// below are git's own, for content anybody can check with
// `git hash-object`.
func TestTheBlobIdIsGits(t *testing.T) {
	for _, test := range []struct {
		content string
		want    string
	}{
		{"", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
	} {
		if got := blobID([]byte(test.content)); got != test.want {
			t.Errorf("blobID(%q) = %s, want %s", test.content, got, test.want)
		}
	}
}

// TestTheVendoredImportsAreReportedNotFailed covers the import rule over
// the vendored tree.
//
// The rule is closed for Zenith's own code and it stays closed. But the
// vendored tree is upstream's, and upstream's components import modules
// this image does not carry yet. That is a fact about the image's package
// list rather than a fault in a file nobody may edit, so it is reported.
func TestTheVendoredImportsAreReportedNotFailed(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell", EntryPoint: "shell.qml"}

	report := CheckVendored(env)
	if len(report.Modules) == 0 {
		t.Fatal("no modules were reported; the vendored tree imports several " +
			"the image does not carry")
	}

	// The modules the vendored tree needs the image to carry.
	//
	// org.kde.kirigami is not among them: it was, until the widget that
	// imported it was replaced with one that does not need a KDE
	// framework, and the point of the report is what the tree needs now.
	// QtPositioning is here because the vendored weather service imports
	// it.
	for _, module := range []string{
		"Qt5Compat.GraphicalEffects",
		"Qt.labs.folderlistmodel",
		"Qt.labs.synchronizer",
		"QtCore",
		"QtPositioning",
		"QtQuick.Controls.Material",
		"QtQuick.Effects",
		"QtQuick.Shapes",
	} {
		if !contains(report.Modules, module) {
			t.Errorf("%s is not in the report: %v", module, report.Modules)
		}
	}

	// The shell's own modules and Quickshell's are provided by the tree
	// that imports them, so they are not the image's business.
	for _, module := range report.Modules {
		if module == "qs" || strings.HasPrefix(module, "qs.") {
			t.Errorf("%s is the shell's own module and is not the image's "+
				"business", module)
		}
		if strings.HasPrefix(module, "Quickshell") {
			t.Errorf("%s is Quickshell's own module and is not the image's "+
				"business", module)
		}
	}

	// And it is reported, not failed: the import rule is not raised
	// against a vendored file.
	for _, finding := range Check(env) {
		if finding.Rule == "import" {
			t.Errorf("a vendored file was reported against the import rule: %s",
				finding)
		}
	}
}

// contains reports whether a list holds a value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// TestTheVendoredTreeIsNotHeldToZenithsRules covers the scope of the
// rules.
//
// A vendored file is upstream's, and the shell's rules are about Zenith's
// own code. Upstream's Material palette, upstream's glyphs and upstream's
// absolute paths are all correct in upstream's tree, and a rule that
// reported them would be reporting somebody else's code as this project's
// problem — and would be reporting it in a way that cannot be fixed, since
// the fix is an edit to a file that is not ours to edit.
func TestTheVendoredTreeIsNotHeldToZenithsRules(t *testing.T) {
	env := vendorTree(t, map[string]string{
		"modules/common/Widget.qml": "import QtQuick\nimport SomeUserModule\n" +
			"Item { color: \"#4c8bf5\"; source: \"/etc/sddm.conf.d/10-zenith.conf\" }\n",
	})
	env.EntryPoint = "shell.qml"
	if err := os.WriteFile(filepath.Join(env.Root, "shell", "shell.qml"),
		[]byte("import Quickshell\nShellRoot {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, finding := range Check(env) {
		t.Errorf("%s %s: %s", finding.Rule, finding.Path, finding.Detail)
	}
}

// houseRules are the rules that are about Zenith's own code.
//
// They are listed here so that the scope test below can say which rules
// must never name a file in the vendored tree. A rule added to the package
// later is not in this list, and the test says nothing about it — which is
// why the list is here and not only in the rules themselves.
var houseRules = map[string]bool{
	"import":               true,
	"hard-coded colour":    true,
	"glyph":                true,
	"glyph escape":         true,
	"glyph range":          true,
	"unknown command":      true,
	"boundary":             true,
	"property assignment":  true,
	"readonly assignment":  true,
	"widget catalogue":     true,
	"settings read":        true,
	"settings unreachable": true,
	"unused setting":       true,
	"schema id":            true,
}

// TestNoRuleNamesAVendoredFile runs the rules over the repository's own
// tree and checks that none of them names a file in the vendored tree.
//
// The tree is large and upstream's, and the rules are many. The one
// property that matters is the scope: a rule about Zenith's own code must
// never name a file that is upstream's.
func TestNoRuleNamesAVendoredFile(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell", EntryPoint: "shell.qml", OwnsSettings: true}

	base := env.path(".")
	for _, finding := range Check(env) {
		if !houseRules[finding.Rule] {
			continue
		}
		relative, err := filepath.Rel(base, finding.Path)
		if err != nil {
			continue
		}
		if isVendored(relative) {
			t.Errorf("a rule about Zenith's own code named a vendored file: "+
				"%s %s: %s", finding.Rule, finding.Path, finding.Detail)
		}
	}
}

// TestTheOverlayListMatchesTheVendorScript keeps the two lists of
// overrides together.
//
// The vendor script's EXCLUDE_FILES is the list of paths the script does
// not take from upstream, and the validator's overlay list is the same
// list. Two copies of one list drift, and the drift is invisible until a
// sync overwrites a file Zenith wrote — which is how a hand edit gets into
// a tree that is supposed to be byte-identical to upstream.
func TestTheOverlayListMatchesTheVendorScript(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "vendor", "vendor.sh"))
	if err != nil {
		t.Fatal(err)
	}

	excluded := vendorScriptExcludes(string(script))
	if excluded == nil {
		t.Fatal("the vendor script has no EXCLUDE_FILES list")
	}

	for _, path := range excluded {
		if _, overlay := overlayFiles[path]; !overlay {
			t.Errorf("the vendor script excludes %q and the validator does "+
				"not know about it; the two lists have to agree", path)
		}
	}
	for path := range overlayFiles {
		if !contains(excluded, path) {
			t.Errorf("the validator knows %q as an override and the vendor "+
				"script does not exclude it; the two lists have to agree", path)
		}
	}
}

// vendorScriptExcludes reads the EXCLUDE_FILES list out of the vendor
// script.
//
// The script is the only thing that writes into the vendored paths, so its
// list of the paths it does not write is the list of the paths Zenith
// supplies.
func vendorScriptExcludes(script string) []string {
	const marker = "EXCLUDE_FILES=\""
	start := strings.Index(script, marker)
	if start < 0 {
		return nil
	}
	rest := script[start+len(marker):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(rest[:end], "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// TestTheOverlayListStaysSmall stops the overlay list becoming the
// loophole the drift check was added to prevent.
//
// Every entry in it is a file the drift check does not compare, and every
// entry in it is a file that the vendor script does not take from upstream.
// A list that grows is a tree that is less and less upstream's, and it
// grows one file at a time.
func TestTheOverlayListStaysSmall(t *testing.T) {
	if len(overlayFiles) > 6 {
		t.Errorf("there are %d declared overrides; the list is a short one "+
			"of the paths Zenith supplies rather than takes", len(overlayFiles))
	}
	for path, reason := range overlayFiles {
		if reason == "" {
			t.Errorf("%s is an override with no recorded reason", path)
		}
	}
}

// TestThePackageManifestReaderFindsTheQMLPackages covers the narrow reader
// against the repository's own package manifest.
//
// The reader exists to answer one question — which QML modules the image
// carries — and a reader that quietly answers it wrong reports a
// dependency the image has as a gap, and hides one it does not have.
func TestThePackageManifestReaderFindsTheQMLPackages(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "system", "packages.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	names := declaredPackages(string(data))
	for _, want := range []string{
		"qml6-module-qtquick",
		"qml6-module-qtquick-controls",
		"qml6-module-qtquick-layouts",
	} {
		if !contains(names, want) {
			t.Errorf("%s was not found in the package manifest", want)
		}
	}
	// The excluded packages are not installed packages, and reading them as
	// installed ones would say the image carries things it deliberately
	// does not.
	for _, unwanted := range []string{"gnome-shell", "gdm3"} {
		if contains(names, unwanted) {
			t.Errorf("%s is in the excluded list and was read as an "+
				"installed package", unwanted)
		}
	}
}

// TestTheModuleAndThePackageNameFoldTogether covers the mapping between a
// QML module and the package that carries it.
//
// The archive names the package after the module with its dots written as
// dashes, and the comparison is case-insensitive because the package names
// are lower case and the module names are not.
func TestTheModuleAndThePackageNameFoldTogether(t *testing.T) {
	for _, test := range []struct{ module, packageName string }{
		{"QtQuick", "qml6-module-qtquick"},
		{"QtQuick.Controls", "qml6-module-qtquick-controls"},
		{"QtQuick.Effects", "qml6-module-qtquick-effects"},
		{"QtQuick.Shapes", "qml6-module-qtquick-shapes"},
		{"Qt5Compat.GraphicalEffects", "qml6-module-qt5compat-graphicaleffects"},
		{"Qt.labs.synchronizer", "qml6-module-qt-labs-synchronizer"},
		{"org.kde.kirigami", "qml6-module-org-kde-kirigami"},
	} {
		suffix := strings.TrimPrefix(test.packageName, "qml6-module-")
		if got := moduleKey(test.module); got != suffix {
			t.Errorf("moduleKey(%q) = %q, want %q", test.module, got, suffix)
		}
	}
}

// TestTheCarriedModulesAreTheOnesTheImageDeclares covers the comparison
// the doctor's report rests on.
//
// The report says which QML modules the image does not carry yet, and it
// says it by comparing the vendored tree's imports against the image's
// package manifest. A comparison that is wrong in either direction is a
// report that is wrong in both.
func TestTheCarriedModulesAreTheOnesTheImageDeclares(t *testing.T) {
	env := Env{Root: "../../..", Dir: "shell"}

	modules, carried, packages := vendoredModuleReport(env, vendoredFiles(env))
	if !packages {
		t.Fatal("the image's package manifest was not read")
	}
	if len(modules) == 0 {
		t.Fatal("no modules were reported")
	}

	// Nothing the image declares is reported as a gap, and nothing the
	// image does not declare is reported as carried.
	declared, _ := declaredQMLModules(env.Root)
	for _, module := range carried {
		if !moduleCarried(declared, module) {
			t.Errorf("%s is reported as carried and no declared package "+
				"carries it", module)
		}
	}
	for _, module := range modules {
		inManifest := moduleCarried(declared, module)
		inCarried := contains(carried, module)
		if inManifest != inCarried {
			t.Errorf("%s: carried by the manifest = %t, reported as "+
				"carried = %t", module, inManifest, inCarried)
		}
	}
}

// TestTheControlsStyleIsCarriedByTheControlsPackage covers the one module
// the archive does not name a package for.
//
// The styles are plugins of the Controls module, and the archive ships them
// inside the package that carries it: qml6-module-qtquick-controls carries
// Basic, Fusion, Imagine, Material and Universal. A report that said the
// image was missing a package for QtQuick.Controls.Material would be asking
// for a package that does not exist.
func TestTheControlsStyleIsCarriedByTheControlsPackage(t *testing.T) {
	declared := map[string]bool{"qtquick-controls": true}
	if !moduleCarried(declared, "QtQuick.Controls.Material") {
		t.Error("QtQuick.Controls.Material is not carried by the controls " +
			"package; it is the Material style of that module")
	}
	if moduleCarried(declared, "QtQuick.Controls.Basic") != true {
		t.Error("a style is not carried by the controls package")
	}
	// A module that is not a style is not carried by the controls package
	// either, and neither is a module of a different tree.
	if moduleCarried(declared, "QtQuick.Effects") {
		t.Error("the controls package was taken to carry the effects module")
	}
	if moduleCarried(map[string]bool{"qtquick": true}, "QtQuick.Controls") {
		t.Error("the quick package was taken to carry the controls module")
	}
}
