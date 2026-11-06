.pragma library

// AddonsPage as data. Generated from the page it replaces: the placement rows
// are what its detail view exposes for an installed plugin, the declared-rows
// entry records that a plugin's manifest can add its own settings, and the
// management row names the commands that really run — `asura plugin install`
// and `asura plugin remove`, which the store's plugins category delegates to.
// Descriptions are written by hand; the inventory carries engineering notes,
// which are not user copy.

var rows = [
    {
        "tab": "detail",
        "group": "Placement",
        "key": "<pluginId>.enabled",
        "label": "Enabled",
        "desc": "Runs the add-on; off keeps it installed but idle",
        "ctl": "sw",
        "src": "plugins.json (via `asura-plugins-place <id> enabled <true|false>`)"
    },
    {
        "tab": "detail",
        "group": "Placement",
        "key": "<pluginId>.host",
        "label": "Show as",
        "desc": "Where it appears: popout, wallpaper tile or bar glyph",
        "ctl": "seg",
        "src": "plugins.json (via `asura-plugins-place <id> host <hostName>`)",
        "opts": [
            "framePopout",
            "desktopWidget",
            "topbarGlyph",
            "<any"
        ]
    },
    {
        "tab": "detail",
        "group": "(plugin-declared, from manifest.metadata.settings[].group - group headers are rendered by PluginSettingsForm itself, one per distinct `group` string, in schema order; fields with group \"\" get no header)",
        "key": "<pluginId>.settings.<field.key>",
        "label": "(plugin-declared, field.label, falling back to field.key)",
        "desc": "Each add-on defines its own; applied live",
        "ctl": "custom",
        "src": "plugins.json (via `asura-plugins-place <id> settings <json>`, one single-key object per change, jq-merged into the existing settings object)",
        "unit": "none"
    },
    {
        "tab": "Plugins",
        "group": "Management",
        "key": "",
        "label": "Update / Remove",
        "desc": "Refreshes or removes it; placement stays as set",
        "ctl": "action",
        "src": "`asura plugin install <repo> --yes` (update) / `asura plugin remove <id>` (remove); `asura-store install|remove plugins <id>` delegates to them"
    }
];
