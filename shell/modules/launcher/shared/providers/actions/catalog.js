// The system-action catalog: each entry is a command the launcher can fire,
// wired to a real Asura helper (asura-shell, asura-cmd-*). Data only,
// so the set is validated in a node test and the provider stays a thin mapper.
// `exec` is an argv array run with execDetached. Categories group the action-mode
// tabs (System / Appearance / Tools / Media / Settings). An optional `caps` names
// a window-manager capability the action needs; the provider hides the entry on a
// compositor that lacks it (a tool that is not even installed there).

var CATALOG = [
    { id: "lock-screen",      name: "Lock Screen",        category: "System",     icon: "lock",        exec: ["asura-shell", "lock"] },
    { id: "open-clipboard",   name: "Clipboard History",  category: "System",     icon: "clipboard",   exec: ["asura-shell", "menu", "quick-settings#clipboard"] },
    { id: "open-sysinfo",     name: "System Info",        category: "System",     icon: "info",        exec: ["asura-shell", "menu", "quick-settings"] },
    { id: "open-toolkit",     name: "Control Deck",       category: "System",     icon: "grid",        exec: ["asura-shell", "menu", "quick-settings"] },
    { id: "toggle-caffeine",  name: "Keep Awake",         category: "System",     icon: "coffee",      exec: ["asura-cmd-caffeine"] },
    { id: "toggle-game-mode", name: "Game Mode",          category: "System",     icon: "gamepad",     exec: ["asura-cmd-game-mode"], caps: "liveConfigEval" },
    { id: "mirror-displays",  name: "Mirror Displays",    category: "System",     icon: "monitor",     exec: ["asura-monitor", "toggle"], caps: "outputMirror" },

    { id: "next-wallpaper",   name: "Next Wallpaper",     category: "Appearance", icon: "image",       exec: ["asura-wallpaper", "wallpaper", "next"] },
    { id: "pick-wallpaper",   name: "Wallpaper Picker",   category: "Appearance", icon: "image-multi", exec: ["asura-wallpaper", "wallpaper", "ui"] },
    { id: "toggle-nightlight",name: "Night Light",        category: "Appearance", icon: "moon",        exec: ["asura-cmd-nightlight"], caps: "nightLight" },

    { id: "screenshot",       name: "Screenshot",         category: "Tools",      icon: "camera",      exec: ["asura", "ipc", "call", "asura", "toggleScreenshot"] },
    { id: "screen-record",    name: "Screen Record",      category: "Tools",      icon: "video",       exec: ["asura-cmd-screenrecord"] },
    { id: "color-picker",     name: "Color Picker",       category: "Tools",      icon: "eyedropper",  exec: ["asura-cmd-color-picker"] },
    { id: "ocr",              name: "OCR Text Grab",      category: "Tools",      icon: "text-scan",   exec: ["asura-cmd-ocr"] },
    { id: "qr-scan",          name: "Scan QR Code",       category: "Tools",      icon: "qr",          exec: ["asura-cmd-qr-scan"] },
    { id: "google-lens",      name: "Google Lens",        category: "Tools",      icon: "image-search",exec: ["asura-cmd-google-lens"] },

    { id: "media-play-pause", name: "Play / Pause",       category: "Media",      icon: "play",        exec: ["playerctl", "play-pause"] },
    { id: "media-next",       name: "Next Track",         category: "Media",      icon: "skip-next",   exec: ["playerctl", "next"] },
    { id: "media-previous",   name: "Previous Track",     category: "Media",      icon: "skip-prev",   exec: ["playerctl", "previous"] },
    { id: "recognize-music",  name: "Recognize Music",    category: "Media",      icon: "music",       exec: ["asura-cmd-songrec"] },

    { id: "open-settings",    name: "Asura Settings",     category: "Settings",   icon: "settings",    exec: ["asura-shell", "hub", "open"] },
    { id: "keybind-legend",   name: "Keybind Reference",  category: "Settings",   icon: "keyboard",    exec: ["asura", "ipc", "call", "asura", "toggleCheatsheet"] },
    { id: "reload-shell",     name: "Reload Shell",       category: "Settings",   icon: "refresh",     exec: ["asura-shell", "reload"] }
];

var CATEGORIES = ["All", "System", "Appearance", "Tools", "Media", "Settings"];

// The distinct commands the catalog runs, so the provider can leave out the
// actions whose command is not installed.
function commands() {
    var seen = {};
    return CATALOG.map(function (a) { return a.exec[0]; }).filter(function (c) {
        return seen[c] ? false : (seen[c] = true);
    });
}

// Validate the catalog: unique ids, required fields, a known category, and a
// non-empty exec argv. Returns an array of problem strings (empty = valid).
function validate(catalog) {
    var problems = [];
    var seen = {};
    var cats = {};
    for (var i = 1; i < CATEGORIES.length; i++) cats[CATEGORIES[i]] = true;
    for (var j = 0; j < catalog.length; j++) {
        var a = catalog[j];
        if (!a.id) { problems.push("entry " + j + " missing id"); continue; }
        if (seen[a.id]) problems.push("duplicate id: " + a.id);
        seen[a.id] = true;
        if (!a.name) problems.push(a.id + " missing name");
        if (!cats[a.category]) problems.push(a.id + " has unknown category: " + a.category);
        if (!Array.isArray(a.exec) || a.exec.length === 0) problems.push(a.id + " has empty exec");
    }
    return problems;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { CATALOG, CATEGORIES, validate, commands };
}
