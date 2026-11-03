// Quickshell wl_output screencopy capture backend selector.
function shouldUseGrim(setting) {
    return setting === "1";
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { shouldUseGrim };
}
