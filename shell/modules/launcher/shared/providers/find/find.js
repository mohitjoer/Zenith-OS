// The command Find.qml runs. plocate matches names from the system database and
// knows neither type nor owner, so a short script narrows its answer the way fd
// was told to: under $HOME, no dot-directories or junk, files or directories by
// test, images or videos by extension. $1 is the test flag, $2 the query, $3 an
// extension filter. The database is rebuilt daily, so a file made today may not
// show.
var PIPELINE = [
    'plocate -i -b -l 5000 -- "$2" | while IFS= read -r p; do',
    'case $p in "$HOME"/*) ;; *) continue;; esac;',
    'case $p in */.*|*/node_modules/*|*/Trash/*) continue;; esac;',
    '[ "$1" "$p" ] && printf "%s\\n" "$p";',
    'done | grep -i -E "$3" | head -n 25'
].join(" ");

var EXTENSIONS = {
    image: "\\.(png|jpe?g|gif|webp|svg|bmp|avif)$",
    video: "\\.(mp4|mkv|webm|mov|avi|m4v)$"
};

function args(mode, query) {
    return ["sh", "-c", PIPELINE, "sh", mode === "folder" ? "-d" : "-f", query, EXTENSIONS[mode] || "."];
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { args };
}
