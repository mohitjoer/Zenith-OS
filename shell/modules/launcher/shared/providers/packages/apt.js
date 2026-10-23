// Turn the output of the APT tools into package rows. Pure logic, node-tested
// without apt.

// Package names are handed to apt-get later, so only names that cannot be read
// as an option are accepted.
var NAME = /^[a-z0-9][a-z0-9+.:-]*$/;

// `apt-cache search` prints one "name - short description" line per package.
function parseSearch(raw) {
    var rows = [];
    var lines = String(raw || "").split("\n");
    for (var i = 0; i < lines.length; i++) {
        var cut = lines[i].indexOf(" - ");
        var name = (cut < 0 ? lines[i] : lines[i].slice(0, cut)).trim();
        if (NAME.test(name))
            rows.push({ name: name, description: cut < 0 ? "" : lines[i].slice(cut + 3).trim() });
    }
    return rows;
}

// `dpkg-query -W -f '${binary:Package}\t${db:Status-Abbrev}\t${Version}\n'`
// lists every package dpkg knows; "ii" is installed. Returns { name: version },
// the architecture qualifier of multi-arch packages dropped.
function parseInstalled(raw) {
    var installed = {};
    var lines = String(raw || "").split("\n");
    for (var i = 0; i < lines.length; i++) {
        var f = lines[i].split("\t");
        if (f.length >= 3 && f[1].indexOf("ii") === 0)
            installed[f[0].replace(/:[a-z0-9-]+$/, "")] = f[2].trim();
    }
    return installed;
}

// apt-cache takes its patterns as regular expressions; the typed words are literal.
function words(term) {
    return String(term || "").split(/\s+/).filter(function (w) { return w.length > 0; })
        .map(function (w) { return w.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"); });
}

// Name matches before description-only ones (exact, then prefix, then substring),
// shorter names first within a tier, capped.
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
