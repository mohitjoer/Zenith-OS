// Turn the output of XBPS tools into package rows. Pure logic, node-tested.
var NAME = /^[a-zA-Z0-9][a-zA-Z0-9+._-]*$/;

// `xbps-query -Rs <term>` prints lines like:
// "[-] <pkgname>-<version> <short description>" (available)
// "[*] <pkgname>-<version> <short description>" (installed)
function parseSearch(raw) {
    var rows = [];
    var lines = String(raw || "").split("\n");
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line.length) continue;
        var parts = line.match(/^\[([*-])\]\s+([^\s]+)\s*(.*)$/);
        if (!parts) continue;
        var pkgver = parts[2];
        var desc = parts[3] || "";
        var lastDash = pkgver.lastIndexOf("-");
        var name = (lastDash > 0) ? pkgver.slice(0, lastDash) : pkgver;
        var ver = (lastDash > 0) ? pkgver.slice(lastDash + 1) : "";
        if (NAME.test(name)) {
            rows.push({ name: name, version: ver, description: desc, installed: parts[1] === "*" });
        }
    }
    return rows;
}

// `xbps-query -l` lists every installed package:
// "ii <pkgname>-<version> <short description>"
function parseInstalled(raw) {
    var installed = {};
    var lines = String(raw || "").split("\n");
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line.length) continue;
        var parts = line.match(/^ii\s+([^\s]+)/);
        if (parts) {
            var pkgver = parts[1];
            var lastDash = pkgver.lastIndexOf("-");
            var name = (lastDash > 0) ? pkgver.slice(0, lastDash) : pkgver;
            var ver = (lastDash > 0) ? pkgver.slice(lastDash + 1) : "installed";
            installed[name] = ver;
        }
    }
    return installed;
}

function words(term) {
    return String(term || "").split(/\s+/).filter(function (w) { return w.length > 0; });
}

function rank(rows, term, limit) {
    var t = String(term || "").toLowerCase().trim();
    function tier(r) {
        var n = r.name.toLowerCase();
        if (n === t) return 0;
        if (n.indexOf(t) === 0) return 1;
        return n.indexOf(t) >= 0 ? 2 : 3;
    }
    return rows.map(function (r, i) { return { r: r, t: tier(r), i: i }; })
        .sort(function (a, b) {
            return a.t - b.t || a.r.name.length - b.r.name.length || a.i - b.i;
        })
        .slice(0, limit)
        .map(function (x) { return x.r; });
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { parseSearch, parseInstalled, words, rank };
}
