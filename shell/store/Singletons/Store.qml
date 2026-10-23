pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import qs.Ui.Singletons

Singleton {
    id: root

    property var catalog: ({})
    property var categories: []
    property var items: []
    property bool loading: false
    property bool offline: false
    property string generatedAt: ""
    readonly property bool refreshing: loading && items.length > 0
    property string error: ""
    property string busyKey: ""
    property string installStage: ""
    property string installError: ""
    property string installErrorKey: ""
    property string _catalogOutput: ""
    property string _catalogError: ""
    property string _installError: ""
    property bool _clearBusyAfterRefresh: false
    property var _queue: []
    property bool updateAvailable: false
    property string revision: ""
    property bool _forced: false
    property string _op: "install"            // which flow installProc is running
    property bool _cancelled: false
    // Packages matching the typed query anywhere in the archive, beyond the
    // Discover shelf the catalogue carries. App.qml folds them into the search.
    property var found: []
    property string _searchQuery: ""
    property string _searchOutput: ""

    function itemKey(item) {
        return item ? String(item.category || "") + ":" + String(item.id || "") : "";
    }

    function category(id) {
        for (let i = 0; i < categories.length; i++)
            if (categories[i].id === id)
                return categories[i];
        return null;
    }

    function refresh(force) {
        if (catalogProc.running)
            return;
        root._forced = force === true;
        loading = true;
        error = "";
        _catalogOutput = "";
        _catalogError = "";
        catalogProc.command = force ? ["zenith-store", "catalog", "--refresh"] : ["zenith-store", "catalog"];
        catalogProc.running = true;
    }

    // The archive is too large to carry in the catalogue, so a typed query is
    // asked of apt directly. Debounced, and a new query replaces the last.
    function search(query) {
        root._searchQuery = String(query || "").trim();
        searchProc.running = false;
        if (root._searchQuery.length < 2) {
            searchTimer.stop();
            root.found = [];
            return;
        }
        searchTimer.restart();
    }

    // Stop every child process before the window tears down, so no fetch is in
    // flight when the QML engine is destroyed -- a shutdown-crash guard.
    function shutdown() {
        catalogProc.running = false;
        installProc.running = false;
        searchProc.running = false;
    }

    function install(item, dither, components) {
        // A paused item, or one written for another window manager, stays listed
        // but never downloads: install and update are refused here so no UI path
        // (button, keyboard, or accessibility) can start a fetch. Remove is a
        // separate flow and stays allowed.
        if (!item || busyKey !== "" || item.downloadPaused === true || item.unavailable === true)
            return;
        busyKey = itemKey(item);
        root._op = "install";
        installStage = "FETCHING";
        installError = "";
        installErrorKey = "";
        _installError = "";
        installProc.command = ["zenith-store", "install", String(item.category), String(item.id)];
        installProc.running = true;
    }

    // The backend stops a download at once. Once dpkg has begun it finishes the
    // transaction instead, because a package half-installed is worse than a wait;
    // either way the process exits and the catalogue is read again.
    function cancel() {
        if (busyKey === "" || !installProc.running)
            return;
        root._cancelled = true;
        root.installStage = "CANCELLING";
        root._queue = [];
        installProc.running = false;
    }

    // One {"stage","percent"} line per step from `zenith-store install|remove`.
    function _progress(line) {
        try {
            const step = JSON.parse(line);
            if (step.stage)
                root.installStage = step.percent >= 0 ? step.stage + " " + step.percent + "%" : step.stage;
        } catch (e) {
            // not a progress line
        }
    }

    // remove uninstalls an installed product, whatever its category (every
    // provider implements remove). Single-flight like install and allowed even
    // for a download-paused item, whose installed copy can still be taken off.
    function remove(item) {
        if (!item || busyKey !== "" || item.installed !== true && item.active !== true
                && item.enabled !== true && Number(item.installedCount || 0) <= 0)
            return;
        busyKey = itemKey(item);
        root._op = "remove";
        installStage = "REMOVING";
        installError = "";
        installErrorKey = "";
        _installError = "";
        installProc.command = ["zenith-store", "remove", String(item.category), String(item.id)];
        installProc.running = true;
    }

    function clearInstallError(item) {
        if (installErrorKey !== itemKey(item))
            return;
        installError = "";
        installErrorKey = "";
        installStage = "";
    }

    function retryInstall(item, dither, components) {
        clearInstallError(item);
        install(item, dither, components);
    }

    // installAll queues every not-yet-installed item and installs them one at a
    // time (install is single-flight): each completion pumps the next. The batch
    // stops on the first failure so the error stays visible.
    function installAll(list) {
        var q = [];
        var src = Array.isArray(list) ? list : [];
        for (var i = 0; i < src.length; i++)
            if (src[i] && src[i].installed !== true && src[i].downloadPaused !== true && src[i].unavailable !== true)
                q.push(src[i]);
        _queue = q;
        _pumpQueue();
    }
    function _pumpQueue() {
        if (busyKey !== "" || _queue.length === 0)
            return;
        var next = _queue[0];
        _queue = _queue.slice(1);
        install(next);
    }

    function openSettings(item) {
        if (!item)
            return;
        Quickshell.execDetached(["zenith-store", "settings", String(item.category), String(item.id)]);
    }

    Component.onCompleted: refresh(false)

    Process {
        id: catalogProc
        stdout: StdioCollector { onStreamFinished: root._catalogOutput = text }
        stderr: StdioCollector { onStreamFinished: root._catalogError = text }
        onExited: code => {
            root.loading = false;
            if (code !== 0) {
                root.error = root._catalogError.trim() || I18n.tr("Catalogue failed");
                if (root._clearBusyAfterRefresh) {
                    root._clearBusyAfterRefresh = false;
                    root.busyKey = "";
                }
                return;
            }
            try {
                const next = JSON.parse(root._catalogOutput);
                if (!Array.isArray(next.categories) || !Array.isArray(next.items))
                    throw new Error("catalogue arrays are missing");
                root.catalog = next;
                root.categories = next.categories.slice();
                root.items = next.items.slice();
                root.offline = next.offline === true;
                root.generatedAt = String(next.generatedAt || "");
                root.error = "";
                root.revision = String(next.revision || "");
                if (root._forced)
                    root.updateAvailable = false;
                // Hits held from before an install or removal still carry the old
                // installed state.
                if (root._searchQuery.length >= 2)
                    searchTimer.restart();
                if (root._clearBusyAfterRefresh) {
                    root._clearBusyAfterRefresh = false;
                    root.installStage = "COMPLETE";
                    root.busyKey = "";
                }
            } catch (e) {
                root.error = I18n.tr("Invalid catalogue: %1").arg(e);
                if (root._clearBusyAfterRefresh) {
                    root._clearBusyAfterRefresh = false;
                    root.busyKey = "";
                }
            }
            root._pumpQueue();
        }
    }

    Process {
        id: installProc
        stdout: SplitParser { onRead: line => root._progress(line) }
        stderr: StdioCollector { onStreamFinished: root._installError = text }
        onRunningChanged: if (running) root.installStage = root._op === "remove" ? "REMOVING" : "INSTALLING"
        onExited: code => {
            const cancelled = root._cancelled;
            root._cancelled = false;
            if (code !== 0 && cancelled) {
                root.installStage = "";
                root.busyKey = "";
                root.refresh(true);
                return;
            }
            if (code !== 0) {
                root.installStage = "FAILED";
                root.installError = root._installError.trim()
                        || (root._op === "remove" ? I18n.tr("Removal failed") : I18n.tr("Installation failed"));
                root.installErrorKey = root.busyKey;
                root.busyKey = "";
                root._queue = [];
                return;
            }
            root.installStage = "VERIFYING";
            root._clearBusyAfterRefresh = true;
            root.refresh(true);
        }
    }

    Timer {
        id: searchTimer
        interval: 350
        onTriggered: {
            root._searchOutput = "";
            searchProc.command = ["zenith-store", "search", root._searchQuery];
            searchProc.running = true;
        }
    }

    Process {
        id: searchProc
        stdout: StdioCollector { onStreamFinished: root._searchOutput = text }
        onExited: code => {
            if (code !== 0)
                return;
            try {
                root.found = JSON.parse(root._searchOutput).items || [];
            } catch (e) {
                root.found = [];
            }
        }
    }
}
