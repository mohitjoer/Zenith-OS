import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons
import "../../Singletons"
import "apt.js" as Apt
import "../requeststate.js" as RequestState
import ".."

// Package provider backed by APT: `apt-cache search` for the names and
// `dpkg-query` for what is installed. Routed by "/" is the actions panel;
// packages use explicit "install "/"remove "/"search " queries (matching inir)
// so a plain search never forks apt. Search is async + cached; installs/removes
// run in a terminal so apt's output and the pkexec prompt are visible. The
// provider registers only when apt-cache and dpkg-query exist.
Provider {
    id: packages

    providerId: "packages"
    prefix: ">"
    defaultProvider: false

    property bool available: false
    onAvailableChanged: {
        packages.cachedQuery = "";
        packages.cachedRows = [];
        packages.clearRequest();
        Dispatcher.notifyAsync();
    }
    property string cachedQuery: ""
    property var cachedRows: []
    // { name: version } of what dpkg has installed, read alongside each search.
    property var installed: ({})
    property string pendingQuery: ""
    property int pendingGeneration: 0
    property bool debounceReady: false
    property var requestState: RequestState.initial()
    property bool installedFinished: false
    property bool searchFinished: false

    // $@ is apt-get's own arguments; the terminal stays open to show the result.
    readonly property string script: 'pkexec apt-get "$@"; printf "\\nPress Enter to close. "; read -r _'

    function setRequestState(next) {
        if (next === packages.requestState)
            return false;
        var wasBusy = RequestState.isBusy(packages.requestState);
        var nowBusy = RequestState.isBusy(next);
        packages.requestState = next;
        if (wasBusy !== nowBusy)
            Dispatcher.setBusy("packages", nowBusy);
        return true;
    }

    function cancelProcesses() {
        if (dpkgProc.inFlight)
            dpkgProc.running = false;
        if (searchProc.inFlight)
            searchProc.running = false;
    }

    function clearRequest() {
        var next = RequestState.clear(packages.requestState);
        if (next === packages.requestState)
            return;
        packages.setRequestState(next);
        debounce.stop();
        packages.debounceReady = false;
        packages.cancelProcesses();
    }

    function invalidateSearch() {
        packages.cachedQuery = "";
        packages.cachedRows = [];
        packages.clearRequest();
    }

    function adoptCached(key) {
        var next = RequestState.adoptSettled(
            packages.requestState, key);
        if (next === packages.requestState)
            return;
        packages.setRequestState(next);
        debounce.stop();
        packages.debounceReady = false;
        packages.cancelProcesses();
    }

    function schedule(key) {
        var next = RequestState.begin(packages.requestState, key);
        if (next === packages.requestState)
            return;
        packages.setRequestState(next);
        packages.pendingQuery = key;
        packages.pendingGeneration = next.generation;
        packages.debounceReady = false;
        debounce.restart();
        packages.cancelProcesses();
    }

    function startPending() {
        if (!packages.debounceReady
                || dpkgProc.inFlight || searchProc.inFlight)
            return;
        var next = RequestState.markRunning(
            packages.requestState, packages.pendingQuery,
            packages.pendingGeneration);
        if (next === packages.requestState)
            return;
        packages.debounceReady = false;
        packages.setRequestState(next);
        packages.installedFinished = false;
        packages.searchFinished = false;

        dpkgProc.term = packages.pendingQuery;
        dpkgProc.requestGeneration = packages.pendingGeneration;
        dpkgProc.out = "";
        dpkgProc.inFlight = true;
        dpkgProc.didStart = false;
        searchProc.term = packages.pendingQuery;
        searchProc.requestGeneration = packages.pendingGeneration;
        searchProc.out = "";
        searchProc.inFlight = true;
        searchProc.didStart = false;
        dpkgProc.running = true;
        searchProc.running = true;
    }

    function finishRequest(key, generation) {
        if (!packages.installedFinished || !packages.searchFinished)
            return;
        packages.setRequestState(RequestState.settle(
            packages.requestState, key, generation));
    }

    // text after ">": "search x" / "install x" / "remove x", or a bare query that
    // defaults to search. So ">htop" searches, ">install htop" installs.
    function parseOp(text) {
        var m = String(text || "").match(/^(install|remove|search)\s+(.+)$/i);
        if (m)
            return { op: m[1].toLowerCase(), term: m[2].trim() };
        var t = String(text || "").trim();
        // a bare verb mid-typing (">install ") is an incomplete command, not
        // a search for the literal word; searching it flashed unrelated rows.
        if (/^(install|remove|search)$/i.test(t))
            return null;
        return t.length ? { op: "search", term: t } : null;
    }

    function rowFor(pkg, op) {
        var version = packages.installed[pkg.name];
        var verb = op === "remove" ? I18n.tr("Remove") : (version ? I18n.tr("Reinstall") : I18n.tr("Install"));
        return {
            id: "pkg:apt:" + pkg.name,
            title: pkg.name + (version ? "  " + version : ""),
            subtitle: "apt" + (version ? "  " + I18n.tr("(installed)") : "") + "  " + pkg.description,
            icon: "",
            type: "Package",
            score: 0,
            actions: [{
                id: op === "remove" ? "remove" : "install",
                name: verb,
                icon: "",
                execute: function () {
                    var args = op === "remove" ? ["remove", "-y", pkg.name]
                        : (version ? ["install", "--reinstall", "-y", pkg.name] : ["install", "-y", pkg.name]);
                    Spawn.run(["zenith-app", "terminal", "--", "sh", "-c", packages.script, "sh"].concat(args));
                    // the install/remove will change what a re-search should
                    // show; drop the cached rows so the next query refetches.
                    packages.invalidateSearch();
                }
            }]
        };
    }

    function query(text) {
        if (!packages.available) {
            packages.clearRequest();
            return [];
        }
        var p = parseOp(text);
        if (!p || p.term.length < 2) {
            packages.clearRequest();
            return [];
        }
        if (p.term === packages.cachedQuery) {
            packages.adoptCached(p.term);
            return packages.cachedRows.filter(function (pkg) {
                return p.op !== "remove" || packages.installed[pkg.name] !== undefined;
            }).map(function (pkg) { return packages.rowFor(pkg, p.op); });
        }
        packages.schedule(p.term);
        return [];
    }

    Timer {
        id: debounce
        interval: 120
        repeat: false
        onTriggered: {
            packages.debounceReady = true;
            packages.startPending();
        }
    }

    // What dpkg has installed (about 3000 lines, a few ms), read with every
    // search so a package installed in the meantime shows as installed.
    Process {
        id: dpkgProc
        property string term: ""
        property int requestGeneration: 0
        property bool inFlight: false
        property bool didStart: false
        property string out: ""
        command: ["dpkg-query", "-W", "-f", "${binary:Package}\t${db:Status-Abbrev}\t${Version}\n"]
        stdout: SplitParser {
            onRead: data => dpkgProc.out += data + "\n"
        }
        onStarted: dpkgProc.didStart = true
        onRunningChanged: {
            if (!running && dpkgProc.inFlight && !dpkgProc.didStart) {
                var key = dpkgProc.term;
                var generation = dpkgProc.requestGeneration;
                dpkgProc.inFlight = false;
                if (packages.requestState.phase === "running"
                        && RequestState.isCurrent(
                            packages.requestState, key, generation)) {
                    packages.installedFinished = true;
                    packages.finishRequest(key, generation);
                }
                packages.startPending();
            }
        }
        onExited: (code, status) => {
            var key = dpkgProc.term;
            var generation = dpkgProc.requestGeneration;
            dpkgProc.inFlight = false;
            var current = packages.requestState.phase === "running"
                && RequestState.isCurrent(
                    packages.requestState, key, generation);
            if (current) {
                packages.installedFinished = true;
                if (status === 0 && code === 0) {
                    packages.installed = Apt.parseInstalled(dpkgProc.out);
                    Dispatcher.notifyAsync();
                }
                packages.finishRequest(key, generation);
            }
            packages.startPending();
        }
    }

    Process {
        id: availProc
        command: ["sh", "-c", "command -v apt-cache >/dev/null 2>&1 && command -v dpkg-query >/dev/null 2>&1"]
        onExited: (code) => {
            packages.available = (code === 0);
            if (packages.available)
                Dispatcher.register(packages);
        }
    }

    Process {
        id: searchProc
        property string term: ""
        property int requestGeneration: 0
        property bool inFlight: false
        property bool didStart: false
        property string out: ""
        command: ["apt-cache", "search", "--"].concat(Apt.words(term))
        stdout: SplitParser {
            onRead: data => searchProc.out += data + "\n"
        }
        onStarted: searchProc.didStart = true
        onRunningChanged: {
            if (!running && searchProc.inFlight
                    && !searchProc.didStart) {
                var key = searchProc.term;
                var generation = searchProc.requestGeneration;
                searchProc.inFlight = false;
                if (packages.requestState.phase === "running"
                        && RequestState.isCurrent(
                            packages.requestState, key, generation)) {
                    packages.searchFinished = true;
                    packages.finishRequest(key, generation);
                }
                packages.startPending();
            }
        }
        onExited: (code, status) => {
            var key = searchProc.term;
            var generation = searchProc.requestGeneration;
            searchProc.inFlight = false;
            var current = packages.requestState.phase === "running"
                && RequestState.isCurrent(
                    packages.requestState, key, generation);
            if (current) {
                packages.searchFinished = true;
                if (status === 0 && code === 0) {
                    packages.cachedQuery = key;
                    packages.cachedRows = Apt.rank(Apt.parseSearch(searchProc.out), key, 30);
                    Dispatcher.notifyAsync();
                }
                packages.finishRequest(key, generation);
            }
            packages.startPending();
        }
    }

    Component.onCompleted: availProc.running = true
}
