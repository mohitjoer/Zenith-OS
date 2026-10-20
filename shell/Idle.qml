pragma ComponentBehavior: Bound

// Idle policy: when the session locks.
//
// Three timers, dim, blank, lock, each fed by the same activity signal. Not one
// timer with three thresholds — three, because each is independent and a user
// who wants a blank screen but no auto-lock is a normal thing to want, and a
// single timer cannot express it.
//
// The policy lives here and not in hypridle because the shell is already the
// idle implementation. Two idle daemons both deciding when to lock is a machine
// that locks at a time neither of them was asked for.

import Quickshell
import Quickshell.Io
import QtQuick

Item {
    id: idle

    // Every input resets every timer. Not "the last timer" and not "each timer
    // independently": a user who is typing has not been idle for 300 seconds,
    // and a policy that let the lock timer run while they typed is a policy
    // that locks people out mid-sentence.
    property bool active: false

    // The three timeouts, in seconds, with 0 meaning never.
    readonly property int dimAfter: Config.idleDimSeconds
    readonly property int blankAfter: Config.idleBlankSeconds
    readonly property int lockAfter: Config.idleLockSeconds

    // Set while the session is locked, so a notification that arrives after
    // locking does not wake the screen back up.
    property bool locked: false

    Connections {
        target: idle
        function onActiveChanged() {
            if (idle.active) idle.bump()
        }
    }

    function bump() {
        locked = false
        dimTimer.restart()
        blankTimer.restart()
        lockTimer.restart()
    }

    Timer {
        id: dimTimer
        // A timer with interval 0 and running false does nothing at all, which
        // is what "never" should mean. It must not be left running, or it fires
        // on the next event loop turn.
        interval: idle.dimAfter * 1000
        repeat: false
        onTriggered: idle.dimmed = true
    }

    Timer {
        id: blankTimer
        interval: idle.blankAfter * 1000
        repeat: false
        onTriggered: idle.blanked = true
    }

    Timer {
        id: lockTimer
        interval: idle.lockAfter * 1000
        repeat: false

        // The one that matters. Locking twice execs a second locker over the
        // first, and which one is in front depends on which started second.
        onTriggered: {
            if (idle.locked) return
            idle.locked = true
            Config.lock()
        }
    }

    property bool dimmed: false
    property bool blanked: false

    // Re-lock after unlocking, because the timer that fired is spent. Without
    // this, a session unlocks and then never locks again for the rest of the
    // login, which is the opposite of what the policy said.
    function onUnlocked() {
        locked = false
        bump()
    }
}
