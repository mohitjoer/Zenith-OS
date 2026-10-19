package adopt

import (
	"strings"
	"testing"

	"zenith/desktop/internal/portal"
)

// TestAdoptionSourcesAreTheContentThePackageShips keeps two tables that must
// agree about the same four files from drifting apart.
//
// Both exist for a reason. portal.registrations is what `zenith doctor` checks
// — the path a third-party scanner reads and the content Zenith puts behind it.
// registrationSources is what `zenith adopt` copies from when it writes those
// pointers on a machine adopting Zenith. Nothing in the code connects them, and
// the two had already drifted: adoption read the units from
// /usr/share/zenith/session/units/, a directory the package has never created,
// while the package ships them at /usr/share/zenith/units/. The plan would have
// installed a pointer to nothing, because a missing source is tolerated at plan
// time — the package is legitimately not installed yet on a clean machine — so
// the error would have surfaced at execute time, on somebody's machine, in the
// step that is supposed to leave their session working.
//
// A test is the only place that can see it: the paths are valid-looking strings
// and the tree they name does not exist until a package has been built.
func TestAdoptionSourcesAreTheContentThePackageShips(t *testing.T) {
	shipped := map[string]string{}
	for _, registration := range portal.Registrations() {
		if registration.Target == "" {
			continue
		}
		shipped[registration.Path] = registration.Target
	}
	if len(shipped) == 0 {
		t.Fatal("portal.Registrations() reported no content")
	}
	if len(registrationSources) == 0 {
		t.Fatal("adoption registers nothing")
	}

	for _, registration := range registrationSources {
		target, ok := shipped[registration.Path]
		if !ok {
			t.Errorf("adoption writes %s, which portal.CheckRegistrations does "+
				"not check; a registration only one of the two knows about is "+
				"one the other cannot keep honest", registration.Path)
			continue
		}
		if registration.MaterializedFrom != target {
			t.Errorf("adoption copies %s from %s, and the package ships it at %s",
				registration.Path, registration.MaterializedFrom, target)
		}
	}
}

// TestAdoptionWritesOnlyInsideItsOwnTree is the boundary rule in the one place
// that writes from the plan. Adoption materializes pointers; every source it
// copies from is content under /usr/share/zenith/, and a source anywhere else
// would make the adopting step a second writer of somebody else's files.
func TestAdoptionWritesOnlyInsideItsOwnTree(t *testing.T) {
	const ownTree = "/usr/share/zenith/"

	for _, registration := range registrationSources {
		if !strings.HasPrefix(registration.MaterializedFrom, ownTree) {
			t.Errorf("%s is copied from %s, which is not %s",
				registration.Path, registration.MaterializedFrom, ownTree)
		}
	}
}
