package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// The one value Zenith renamed that a user's shell.json can still carry.
//
// FrameBars.js accepts the frame style "slate-frame" or "zenith-frame". Ryoku
// called the second "ryoku-frame", and a shell.json brought across from a Ryoku
// machine still says so; the shell then ignores it and falls back to the
// default, so the style the user picked is lost without a word. The keys
// themselves did not change, only this value, so this is the whole migration.
//
// materialize never rewrites a user's shell.json, which is why this is a doctor
// reconciler and not a shipped file.
const (
	retiredFrameStyle = "ryoku-frame"
	currentFrameStyle = "zenith-frame"
)

// renameFrameStyle rewrites frameBars.style when it names the retired style.
// Every other key comes back as its own bytes; anything else is a no-op, so a
// second run reads clean.
func renameFrameStyle(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(top["frameBars"], &frame); err != nil {
		return nil, false, nil
	}
	var style string
	if err := json.Unmarshal(frame["style"], &style); err != nil || style != retiredFrameStyle {
		return nil, false, nil
	}

	frame["style"], _ = json.Marshal(currentFrameStyle)
	top["frameBars"], _ = json.Marshal(frame)
	out, err := json.MarshalIndent(top, "", "    ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// reconcileShellConfig reports, and with apply performs, the rename in the
// shell.json at path. It returns what it found or did, or "" when the file is
// absent (the shell seeds it on first run) or already current.
//
// The file is replaced whole and keeps its mode: shell.json is 0600 on a real
// machine, and a rewrite that came back 0644 would widen it.
func reconcileShellConfig(path string, apply bool) (string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	migrated, changed, err := renameFrameStyle(raw)
	if err != nil {
		return "", fmt.Errorf("%s does not parse (%v); left alone", path, err)
	}
	if !changed {
		return "", nil
	}
	if !apply {
		return "shell.json: frameBars.style names " + retiredFrameStyle +
			", renamed " + currentFrameStyle + "; `zenith doctor --fix` renames it", nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	tmp := path + ".zenith-tmp"
	if err := os.WriteFile(tmp, migrated, info.Mode().Perm()); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return "shell.json: renamed frameBars.style " + retiredFrameStyle +
		" to " + currentFrameStyle, nil
}
