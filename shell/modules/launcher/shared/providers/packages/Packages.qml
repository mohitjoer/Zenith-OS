import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons
import "../../Singletons"
import "xbps.js" as Xbps
import "../requeststate.js" as RequestState
import ".."

// Package provider backed by XBPS: `xbps-query -Rs` for the names and
// `xbps-query -l` for what is installed. Routed by "/" is the actions panel;
// packages use explicit "install "/"remove "/"search " queries (matching inir)
// so a plain search never forks xbps. Search is async + cached; installs/removes
// run in a terminal so xbps's output and the pkexec prompt are visible. The
// provider registers when xbps-query exists.
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
    // { name: version } of what xbps has installed, read alongside each search.
    property var installed: ({})
    property string pendingQuery: ""
    property int pendingGeneration: 0
    property bool debounceReady: false
    property var requestState: RequestState.initial()
    property bool installedFinished: false
    property bool searchFinished: false

    readonly property string installScript: 'pkexec xbps-install -y "$@"; printf "\\nPress Enter to close. "; read -r _'
    readonly property string removeScript: 'pkexec xbps-remove -y "$@"; printf "\\nPress Enter to close. "; read -r _'

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
        if (installedProc.inFlight)
            installedProc.running = false;
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
                || installedProc.inFlight || searchProc.inFlight)
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

        installedProc.term = packages.pendingQuery;
        installedProc.requestGeneration = packages.pendingGeneration;
        installedProc.out = "";
        installedProc.inFlight = true;
        installedProc.didStart = false;
        searchProc.term = packages.pendingQuery;
        searchProc.requestGeneration = packages.pendingGeneration;
        searchProc.out = "";
        searchProc.inFlight = true;
        searchProc.didStart = false;
        installedProc.running = true;
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
        if (/^(install|remove|search)$/i.test(t))
            return null;
        return t.length ? { op: "search", term: t } : null;
    }

    function rowFor(pkg, op) {
        var version = packages.installed[pkg.name];
        var verb = op === "remove" ? I18n.tr("Remove") : (version ? I18n.tr("Reinstall") : I18n.tr("Install"));
        return {
            id: "pkg:xbps:" + pkg.name,
            title: pkg.name + (version ? "  " + version : ""),
            subtitle: (version ? I18n.tr("(installed)") + "  " : "") + pkg.description,
            icon: "",
            type: "Package",
            score: 0,
            actions: [{
                id: op === "remove" ? "remove" : "install",
                name: verb,
                icon: "",
                execute: function () {
                    var script = op === "remove" ? packages.removeScript : packages.installScript;
                    Spawn.run(["asura-app", "terminal", "--", "sh", "-c", script, "sh", pkg.name]);
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

        var op = packages.parseOp(text);
        if (!op) {
            packages.clearRequest();
            return [];
        }

        if (packages.cachedQuery === op.term) {
            packages.adoptCached(op.term);
            return packages.cachedRows.map(function (pkg) {
                return packages.rowFor(pkg, op.op);
            });
        }

        packages.schedule(op.term);
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

    // Installed packages via xbps-query -l
    Process {
        id: installedProc
        property string term: ""
        property int requestGeneration: 0
        property bool inFlight: false
        property bool didStart: false
        property string out: ""
        command: ["xbps-query", "-l"]
        stdout: SplitParser {
            onRead: data => installedProc.out += data + "\n"
        }
        onStarted: installedProc.didStart = true
        onRunningChanged: {
            if (!running && installedProc.inFlight && !installedProc.didStart) {
                var key = installedProc.term;
                var generation = installedProc.requestGeneration;
                installedProc.inFlight = false;
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
            var key = installedProc.term;
            var generation = installedProc.requestGeneration;
            installedProc.inFlight = false;
            var current = packages.requestState.phase === "running"
                && RequestState.isCurrent(
                    packages.requestState, key, generation);
            if (current) {
                packages.installedFinished = true;
                if (status === 0 && code === 0) {
                    packages.installed = Xbps.parseInstalled(installedProc.out);
                    Dispatcher.notifyAsync();
                }
                packages.finishRequest(key, generation);
            }
            packages.startPending();
        }
    }

    Process {
        id: availProc
        command: ["sh", "-c", "command -v xbps-query >/dev/null 2>&1"]
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
        command: ["xbps-query", "-Rs", term]
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
                    packages.cachedRows = Xbps.rank(Xbps.parseSearch(searchProc.out), key, 30);
                    Dispatcher.notifyAsync();
                }
                packages.finishRequest(key, generation);
            }
            packages.startPending();
        }
    }

    Component.onCompleted: availProc.running = true
}
