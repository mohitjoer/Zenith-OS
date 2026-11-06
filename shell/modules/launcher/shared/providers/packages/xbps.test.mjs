import test from "node:test";
import assert from "node:assert/strict";
import { parseSearch, parseInstalled, rank } from "./xbps.js";

test("xbps parser parses search output", () => {
    const raw = "[-] vlc-3.0.21_1 VLC media player\n[*] htop-3.3.0_1 Interactive process viewer\n";
    const rows = parseSearch(raw);
    assert.equal(rows.length, 2);
    assert.equal(rows[0].name, "vlc");
    assert.equal(rows[0].version, "3.0.21_1");
    assert.equal(rows[0].description, "VLC media player");
    assert.equal(rows[0].installed, false);

    assert.equal(rows[1].name, "htop");
    assert.equal(rows[1].installed, true);
});

test("xbps parser parses installed output", () => {
    const raw = "ii htop-3.3.0_1 Interactive process viewer\nii git-2.48.1_1 Fast version control\n";
    const installed = parseInstalled(raw);
    assert.equal(installed["htop"], "3.3.0_1");
    assert.equal(installed["git"], "2.48.1_1");
    assert.equal(installed["vlc"], undefined);
});

test("xbps rank prioritizes exact match", () => {
    const rows = [
        { name: "vlc-plugin-base", description: "VLC base plugin" },
        { name: "vlc", description: "VLC media player" },
        { name: "libvlc", description: "VLC library" }
    ];
    const ranked = rank(rows, "vlc", 10);
    assert.equal(ranked[0].name, "vlc");
});
