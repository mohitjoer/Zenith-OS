.pragma library

var rows = [
    // ── CONTROL BAR: STYLE & POSITION ──
    {
        "tab": "Bar",
        "group": "STYLE & POSITION",
        "key": "barStyle",
        "label": "Bar style",
        "desc": "Which desktop bar is active.",
        "ctl": "seg",
        "opts": ["qsbar", "kairos", "sumi"],
        "optLabels": { "qsbar": "Control Bar", "kairos": "Kairos", "sumi": "Sumi" },
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "STYLE & POSITION",
        "key": "qsbar.barPosition",
        "label": "Position",
        "desc": "Which screen edge the Control Bar docks to.",
        "ctl": "seg",
        "opts": ["top", "bottom"],
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "STYLE & POSITION",
        "key": "qsbar.barShellStyle",
        "label": "Form",
        "desc": "The shell geometry the bar takes.",
        "ctl": "chips",
        "opts": ["islands", "full", "fit", "dock", "notch"],
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "STYLE & POSITION",
        "key": "qsbar.barScale",
        "label": "Scale",
        "desc": "Bar scale, apart from display scaling.",
        "ctl": "slid",
        "lo": 1.0,
        "hi": 2.0,
        "step": 0.05,
        "pct": true,
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "STYLE & POSITION",
        "key": "qsbar.barAutoHide",
        "label": "Auto-hide",
        "desc": "Hide the bar until the screen edge is hovered.",
        "ctl": "sw",
        "src": "shell"
    },

    // ── CONTROL BAR: SURFACE & CORNERS ──
    {
        "tab": "Bar",
        "group": "SURFACE & CORNERS",
        "key": "qsbar.barBorderEnabled",
        "label": "Bar border",
        "desc": "Draw a fine 1px hairline around the bar shell.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "SURFACE & CORNERS",
        "key": "qsbar.barCornerRadius",
        "label": "Corner radius",
        "desc": "Roundness of bar outer corners.",
        "ctl": "step",
        "lo": 0,
        "hi": 40,
        "unit": "px",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "SURFACE & CORNERS",
        "key": "qsbar.panelTooltipBorderEnabled",
        "label": "Panel & tooltip borders",
        "desc": "Draw contact borders on popups and menus.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "SURFACE & CORNERS",
        "key": "qsbar.barShadowEnabled",
        "label": "Shadow depth",
        "desc": "Two-stop ambient elevation shadow under the bar.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "SURFACE & CORNERS",
        "key": "qsbar.barFrostEnabled",
        "label": "Frost backdrop",
        "desc": "Frosted glass blur behind the bar shell.",
        "ctl": "sw",
        "src": "shell"
    },

    // ── CONTROL BAR: ACCENT ──
    {
        "tab": "Bar",
        "group": "ACCENT",
        "key": "qsbar.barColor",
        "label": "Palette accent slot",
        "desc": "Which palette slot the bar draws its accent from, or follow wallpaper.",
        "ctl": "chips",
        "opts": ["accent", "color01", "color02", "color03", "color04", "color05", "color06", "color07", "color08", "color09", "color10", "color11", "color12", "color13", "color14", "color15", "color16"],
        "src": "shell"
    },

    // ── CONTROL BAR: GAPS ──
    {
        "tab": "Bar",
        "group": "GAPS",
        "key": "qsbar.barGapTop",
        "label": "Top gap",
        "desc": "Clearance between bar and top screen edge.",
        "ctl": "step",
        "lo": 0,
        "hi": 64,
        "unit": "px",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "GAPS",
        "key": "qsbar.barGapBottom",
        "label": "Bottom gap",
        "desc": "Clearance between bar and bottom screen edge.",
        "ctl": "step",
        "lo": 0,
        "hi": 64,
        "unit": "px",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "GAPS",
        "key": "qsbar.barGapLeft",
        "label": "Left gap",
        "desc": "Clearance between bar and left screen edge.",
        "ctl": "step",
        "lo": 0,
        "hi": 64,
        "unit": "px",
        "src": "shell"
    },
    {
        "tab": "Bar",
        "group": "GAPS",
        "key": "qsbar.barGapRight",
        "label": "Right gap",
        "desc": "Clearance between bar and right screen edge.",
        "ctl": "step",
        "lo": 0,
        "hi": 64,
        "unit": "px",
        "src": "shell"
    },

    // ── IDENTITY: LAUNCHER MARK & WORKSPACES ──
    {
        "tab": "Identity",
        "group": "LAUNCHER MARK",
        "key": "qsbar.launcherLogoMode",
        "label": "Mark style",
        "desc": "Display text wordmark or glyph icon in the launcher pill.",
        "ctl": "seg",
        "opts": ["text", "icon"],
        "optLabels": { "text": "Wordmark", "icon": "Glyph" },
        "src": "shell"
    },
    {
        "tab": "Identity",
        "group": "LAUNCHER MARK",
        "key": "qsbar.launcherLogoText",
        "label": "Wordmark",
        "desc": "Text wordmark in the launcher pill when style is Wordmark.",
        "ctl": "chips",
        "opts": ["zenith", "hyprland", "ubuntu", "linux", "nixos", "debian", "fedora", "gentoo", "void", "wayland", "sway", "gnome", "dwm"],
        "src": "shell"
    },
    {
        "tab": "Identity",
        "group": "LAUNCHER MARK",
        "key": "qsbar.launcherLogoIcon",
        "label": "Glyph",
        "desc": "Icon glyph in the launcher pill when style is Glyph.",
        "ctl": "chips",
        "opts": ["zenith", "hyprland", "arch", "grid", "spark", "power", "dragon", "mark", "nix", "branch", "rebel", "ubuntu", "debian", "fedora", "gentoo", "void", "tux", "github", "python", "rust", "go", "node", "vim", "neovim", "steam", "discord"],
        "src": "shell"
    },
    {
        "tab": "Identity",
        "group": "WORKSPACES",
        "key": "qsbar.workspaceMode",
        "label": "Workspaces display",
        "desc": "Show only the active workspace, or a fixed 1-5 / 1-10 indicator row.",
        "ctl": "seg",
        "opts": ["active", "5", "10"],
        "optLabels": { "active": "Active", "5": "1-5", "10": "1-10" },
        "src": "shell"
    },
    {
        "tab": "Identity",
        "group": "WORKSPACES",
        "key": "qsbar.workspaceStyle",
        "label": "Marker style",
        "desc": "Visual presentation of workspace indicators.",
        "ctl": "chips",
        "opts": ["default", "numbers", "magic", "kanji", "rings", "aurora", "pacman"],
        "optLabels": {
            "default": "Dots",
            "numbers": "Numbers",
            "magic": "Glyph",
            "kanji": "Kanji",
            "rings": "Frame",
            "aurora": "Aurora",
            "pacman": "Pacman"
        },
        "src": "shell"
    },

    // ── WIDGETS: TOGGLES ──
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.status",
        "label": "Status notifications",
        "desc": "Notification indicator and count.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.quick",
        "label": "Quick settings",
        "desc": "Quick settings flyout button.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.volume",
        "label": "Volume",
        "desc": "Audio volume and output device selector.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.brightness",
        "label": "Brightness",
        "desc": "Display brightness indicator.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.battery",
        "label": "Battery",
        "desc": "Battery charge and AC power status.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.power",
        "label": "Power profile",
        "desc": "System power performance profile switch.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.network",
        "label": "Network",
        "desc": "Wi-Fi and Ethernet connectivity status.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.bluetooth",
        "label": "Bluetooth",
        "desc": "Bluetooth connection and device status.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.weather",
        "label": "Weather",
        "desc": "Live temperature and condition forecast.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.media",
        "label": "Media player",
        "desc": "Currently playing track title and controls.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.mpris",
        "label": "MPRIS player",
        "desc": "Secondary MPRIS transport pill.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.cpu",
        "label": "CPU load",
        "desc": "Live CPU usage percentage meter.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.memory",
        "label": "Memory",
        "desc": "Live RAM usage and capacity meter.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.gpu",
        "label": "GPU monitor",
        "desc": "Discrete GPU usage percentage meter.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.cpuTemperature",
        "label": "Temperature",
        "desc": "Hardware thermal sensor reading.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.storage",
        "label": "Storage",
        "desc": "Root filesystem available disk space.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.layout",
        "label": "Keyboard layout",
        "desc": "Active input language and layout code.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "BUILT-IN WIDGETS",
        "key": "qsbar.widgets.claude",
        "label": "AI usage tools",
        "desc": "AI coding agent meter and tool launcher.",
        "ctl": "sw",
        "src": "shell"
    },

    // ── WIDGETS: OPTIONS ──
    {
        "tab": "Widgets",
        "group": "WIDGET OPTIONS",
        "key": "qsbar.barTemperatureSource",
        "label": "Temperature sensor source",
        "desc": "Which hardware probe drives the temperature reading.",
        "ctl": "seg",
        "opts": ["cpu", "gpu", "nvme"],
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "WIDGET OPTIONS",
        "key": "qsbar.audioBoost",
        "label": "Audio volume boost",
        "desc": "Allow volume slider to reach 150% maximum.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "WIDGET OPTIONS",
        "key": "qsbar.clock12h",
        "label": "12-hour clock",
        "desc": "Use 12-hour format with AM/PM instead of 24-hour time.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "WIDGET OPTIONS",
        "key": "qsbar.weatherImperial",
        "label": "Imperial weather units",
        "desc": "Display temperature in °F and wind in mph.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Widgets",
        "group": "WIDGET OPTIONS",
        "key": "qsbar.aiTools",
        "label": "AI assistant tools",
        "desc": "Comma-separated list of AI agent meters.",
        "ctl": "chips",
        "opts": ["claude,codex,opencode", "claude,codex", "claude", "codex", "opencode"],
        "src": "shell"
    },

    // ── DOCK: PLACEMENT & STYLE ──
    {
        "tab": "Dock",
        "group": "PLACEMENT & STYLE",
        "key": "dock.enabled",
        "label": "Enable dock",
        "desc": "Show dedicated application dock on screen edge.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Dock",
        "group": "PLACEMENT & STYLE",
        "key": "dock.edge",
        "label": "Edge placement",
        "desc": "Which screen edge the dock anchors to.",
        "ctl": "chips",
        "opts": ["auto", "top", "bottom", "left", "right"],
        "src": "shell"
    },
    {
        "tab": "Dock",
        "group": "PLACEMENT & STYLE",
        "key": "dock.style",
        "label": "Visual design style",
        "desc": "Geometric look and presentation of the dock band.",
        "ctl": "chips",
        "opts": ["islands", "rail", "ledger", "tanzaku", "seal"],
        "src": "shell"
    },

    // ── DOCK: BEHAVIOUR ──
    {
        "tab": "Dock",
        "group": "BEHAVIOUR",
        "key": "dock.autohide",
        "label": "Auto-hide",
        "desc": "Keep as a thin peek strip until hovered.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Dock",
        "group": "BEHAVIOUR",
        "key": "dock.magnify",
        "label": "Magnify on hover",
        "desc": "Smoothly scale up application icon under pointer.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Dock",
        "group": "BEHAVIOUR",
        "key": "dock.media",
        "label": "Media player chip",
        "desc": "Show compact playback status chip when media plays.",
        "ctl": "sw",
        "src": "shell"
    },

    // ── DOCK: SURFACE ──
    {
        "tab": "Dock",
        "group": "SURFACE",
        "key": "dock.frost",
        "label": "Frosted glass blur",
        "desc": "Blur wallpaper background behind dock band.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Dock",
        "group": "SURFACE",
        "key": "dock.shadow",
        "label": "Depth shadow",
        "desc": "Soft elevation shadow beneath dock band.",
        "ctl": "sw",
        "src": "shell"
    },
    {
        "tab": "Dock",
        "group": "SURFACE",
        "key": "dock.labels",
        "label": "Hover application labels",
        "desc": "Show tooltip with application name on pointer hover.",
        "ctl": "sw",
        "src": "shell"
    }
];
