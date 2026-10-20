pragma Singleton
pragma ComponentBehavior: Bound

// The applications on this machine, for the launcher.
//
// Quickshell has no catalogue of what is installed: it is a compositor shell,
// not a software centre, and it will not grow one. So the shell asks the tool
// that already reads the machine's files, and gets the same list `zenith apps`
// prints for a person.
//
// The list is read once, at start-up, by the shell's root and pushed in here.
// The root reads it because a process inside a singleton is a process that
// never runs: it is the one thing about a singleton that the engine does not
// make work, and a launcher that reads its own catalogue in a singleton is a
// launcher with nothing in it.
//
// The list changes when something is installed, and that is not something that
// happens while a session is up — a launcher that re-reads it on a timer is a
// shell that is never idle, and this shell owns the idle policy.

import Quickshell

Singleton {
    id: apps

    // The applications: id, name, icon, command, and whether the application
    // wants a terminal.
    property var applications: []
}
