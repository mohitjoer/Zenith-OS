#!/usr/bin/env bash
# emit the enabled plugin set as one JSON array on stdout. each element merges
# a manifest.json with the user's placement from plugins.json:
#   { "id", "dir", "version", "manifest": {...}, "placement": {...} }
# sources, first wins on duplicate id:
#   $ZENITH_PLUGINS_DIR (dev override, colon-separated)
#   ~/.local/share/zenith/plugins
# placement + per-plugin settings live in ~/.config/zenith/plugins.json:
#   { "<id>": { "enabled": bool, "host": "...", "<host>": {...}, "key": "...",
#               "settings": {...} } }
# no entry or enabled=false = skipped. the shell only loads what the user
# actually turned on. --all lists every installed plugin (Settings wants that).
#
# ported from Ryoku's discover.sh. Ryoku also required a store receipt and a
# pinned content view for every installed plugin; here the folder with its
# manifest is the install, so that chain is gone.
#
# one pass, one jq. this script runs on every placement write -- every time a
# desktop widget is dragged, resized or recoloured -- and a per-plugin jq call
# was ~10 process spawns each, close to a second of CPU on a full desktop. the
# shell forks that in the middle of a drag and the whole desktop janks. so bash
# only globs the file set here and hands every manifest to a single jq by name.
set -euo pipefail

cfg_home="${XDG_CONFIG_HOME:-$HOME/.config}"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
installed_root="$data_home/zenith/plugins"

all=0
[ "${1:-}" = "--all" ] && all=1

shopt -s nullglob

# candidate manifests in precedence order, each with the plugin dir it lives in.
manifest_files=()
manifest_dirs=()
dirs=()
if [ -n "${ZENITH_PLUGINS_DIR:-}" ]; then
	IFS=':' read -r -a extra <<<"$ZENITH_PLUGINS_DIR"
	dirs+=("${extra[@]}")
fi
dirs+=("$installed_root")

for d in "${dirs[@]}"; do
	[ -d "$d" ] || continue
	for m in "$d"/*/manifest.json; do
		[ -f "$m" ] || continue
		manifest_files+=("$m")
		manifest_dirs+=("${m%/manifest.json}")
	done
done

# raw text rather than --slurpfile, which aborts the whole run on one bad file:
# a broken manifest parses to nothing and can never blank the listing the
# desktop is about to load.
rawfiles=()
i=0
for f in "${manifest_files[@]}"; do
	rawfiles+=(--rawfile "m$i" "$f")
	i=$((i + 1))
done

# missing, empty or unreadable reads as empty text and parses to nothing.
user_src=/dev/null
if [ -s "$cfg_home/zenith/plugins.json" ] && [ -r "$cfg_home/zenith/plugins.json" ]; then
	user_src="$cfg_home/zenith/plugins.json"
fi

# the path list travels as one separator-joined string: the record separator
# cannot appear in a real path, so the pairing survives spaces and newlines.
sep=$'\x1e'
printf -v manifest_dirs_raw '%s'"$sep" "${manifest_dirs[@]}"

jq -c -n "${rawfiles[@]}" \
	--argjson all "$all" \
	--rawfile user_text "$user_src" \
	--arg manifest_dirs "$manifest_dirs_raw" \
	'
	def text($k): $ARGS.named[$k] // "";
	def parsed($k): (text($k) | try fromjson catch null);
	def parts($k): (text($k) | split("\u001e") | map(select(length > 0)));
	def obj($v): if ($v | type) == "object" then $v else {} end;
	def str($v): ($v // "") | tostring;
	def at($o; $k): (if ($o | type) == "object" then ($o[$k] // null) else null end);

	# a placement is only on when it says so: anything else reads as off, the
	# way a lookup on a malformed placement would have.
	def on($user; $id): (str(at($user; $id) | if type == "object" then .enabled else null end)) == "true";

	(obj(parsed("user_text"))) as $user
	| parts("manifest_dirs") as $pdirs
	# walk the candidates in precedence order and keep the first entry per id,
	# enabled or not: a disabled dev-override copy still shadows the installed
	# one behind it, exactly as the shell would have resolved it.
	| reduce range(0; $pdirs | length) as $i (
		{ seen: {}, out: [] };
		(obj(parsed("m\($i)"))) as $man
		| (str($man.id)) as $id
		| if ($id | test("^[a-z0-9][a-z0-9-]*$") | not) or (.seen[$id] == true) then .
			else
				.seen[$id] = true
				| if ($all == 1 or on($user; $id)) | not then .
					else .out += [{ id: $id, dir: $pdirs[$i], version: str($man.version), manifest: $man,
						placement: (at($user; $id) // {}) }]
					end
			end
	)
	| .out
'
