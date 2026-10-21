# The Hub

The settings application. A window that draws the pages `zenith config export` hands it and changes
nothing except through the tool that owns the settings.

It is the other configuration in the shell's own tree. `session/zenith-hub` runs
`quickshell -p /usr/share/zenith/shell/hub.qml`, and a Quickshell configuration's root is the
directory of the file it is started on — so the Hub's root is the shell's, and everything the shell
can import the Hub can import. That is what lets it be drawn with the same component kit as the
desktop, and it is why the entry file is at the shell's root rather than beside the files below.

Two windows, two entry files, one tree. The Hub reads the shell's theme and the shell's settings
wrapper as its own, because they are its own, and it still shares no state with the desktop shell: it
reads no schema and writes no dconf except through `zenith config set`.

## The shape of it

| File | What it is |
|---|---|
| `../hub.qml` | The window. The navigation rail, the registry read, and the four states the window can be in. It is one directory up, beside `shell.qml`, because it is an entry file. |
| `PageView.qml` | One page. A group of rows, and the row that chooses the editor for a key. |
| `Editor.qml` | The base every editor is built on: the value, the write, and the error. |
| `Zenith.qml` | The one writer. The queue of writes, and the outcome of each. |
| `GVariant.qml` | The dialect the tool speaks. How a value is quoted, unquoted, parsed and written. |
| `Controls/` | The five editors: `SwitchControl`, `SpinControl`, `ChoiceControl`, `TextControl`, `ListControl`. |

`Editor.qml` is at the top of this directory rather than beside the editors in `Controls/` because a
file in a subdirectory cannot name a type from the directory beside it by relative import. The
editors reach their base through the module that stands for this directory — `qs.hub` — and a module
is named by its directory, so the base has to be at the top of it.

`../hub.qml` is the one file of the Hub that is not here, and it has to be, because the engine
resolves `qs.modules.*` against the root of the configuration: an entry file under this directory
would put the root here, and every import the kit uses to reach itself would name nothing.

## Where the pages come from

Nowhere in this directory. The window runs `zenith config export`, which writes the page registry
as JSON, and draws what it is given: the pages, the keys on each page, the type of each key, the
permitted values of an enum, the range of a number, and the current value of everything.

That is the whole design. A Hub whose QML lists its own pages is a second copy of the registry, and
a second copy agrees with the first until somebody edits one of them — after which a page shows
settings it does not own, or a setting has no page at all, which is the exact fault the invariant
in `desktop/internal/hub` reports. Adding a setting to the Hub means adding a key to a schema and
putting it on a page in Go. Nothing in this directory has to be touched.

The registry is read once, at startup, and not watched. It changes when the package is upgraded,
and a settings window that has to be told about it is a settings window that can be running against
a stale list.

## The control API

An editor is an item with one required property:

```qml
Editor {
    required property var key
}
```

`key` is one entry of the export:

| Field | What it is |
|---|---|
| `schema`, `name` | The setting's identity, as the tool names it. |
| `type` | The GVariant type: `s`, `i`, `b`, `as`, or the name of an enum. |
| `control` | Which editor the setting is edited with: `switch`, `spin`, `choice`, `list`, `text`. |
| `value` | The current value, as a GVariant literal in text form. |
| `default` | The schema's default, in the same spelling. |
| `summary` | One line describing the setting. |
| `minimum`, `maximum`, `hasRange` | The bounds of a number, when the schema declares them. |
| `choices` | The nicks of an enum, in the schema's order. |

The editor is given its key as it is loaded:

```qml
Loader {
    id: control
    Component.onCompleted: control.setSource(
        view.editorFor(row.modelData),
        { key: row.modelData })
}
```

That is not a style. A required property has to be initialised while the object that declares it is
being created, and setting the key after the load is too late: the editor has already failed to be
created by then, and the loader ends in `Loader.Error` with nothing in it. The page gives the key
over at creation, so every binding in the editor is right from the first frame.

## The editors

Five, one for each kind of setting the schemas use. Each is an `Editor` with a name of its own, and
each says the same thing the tool would say if it were asked.

| Editor | What it edits | What it will not do |
|---|---|---|
| `SwitchControl` | A boolean. A track that is an outline over a well when the setting is off and a filled bar when it is on. | It will not read a boolean as `'true'`. The export writes a boolean bare, and a switch that looked for the quotes would show every boolean in the desktop as off. |
| `SpinControl` | An integer, with a field and two buttons. | It will not invent a range. A key that declares bounds is stepped between them and says so; a key that declares none is given none, and the tool decides what the schema accepts. |
| `ChoiceControl` | An enum, as a list of the schema's own nicks. | It will not show the first choice when the stored value is not one of them. It says what the value is instead, because a control that shows a value the setting does not have is showing a lie. |
| `TextControl` | A string, as a field. | It will not write what it was given. A GVariant string is quoted on the way in and on the way out, and a path that names a file that is not there says so under the field. |
| `ListControl` | A list of strings, as a row per item. | It will not split the list on commas. A GVariant tuple is parsed and written by the dialect, so an item with a comma in it and an item with an apostrophe in it both come back as the item that went in. |

## The dialect

Every value is a GVariant literal in text form, and the two directions do not spell the same value
the same way.

| Type | The export writes | The tool takes |
|---|---|---|
| `s` | `'file:///usr/share/zenith/backgrounds/galaxy.png'` | the same |
| `i` | `32` | the same |
| `b` | `true` | the same |
| `as` | `['workspaces', 'clock']` | the same |
| an enum | `'top'` | `top` |

The enum is the exception, and it is the one that matters. The export quotes the nick because that
is how GVariant spells a string, and the tool compares what it is given against the schema's nicks,
so a quoted nick is not a nick and the write is refused. Every enum write failed until the control
was changed to send the bare one.

`GVariant.qml` is where that rule lives, along with the quoting and the parsing. The list is parsed
rather than split, because a comma inside a quoted element belongs to the element and an apostrophe
inside one belongs to the element: a list edited as one line of comma-separated text cannot say
where one item ends and the next begins, and the mangled list is what gets written back.

## The write path

One writer: `Zenith.set(key, value)`, which runs `zenith config set` and nothing else. The editors
never touch dconf.

The writes are queued and run one at a time. A process that is given a second command while it is
running belongs to neither write, and the value on screen would be one of two.

Each write reports back, and the report is what the editor acts on:

- **Refused.** The tool's own message is shown under the control, and the editor goes back to the
  value the tool last accepted. A value on screen is a claim about the setting, and a refused write
  did not change the setting.
- **Accepted.** The value becomes the one the editor falls back to.

A write that failed quietly is a setting that looks changed and is not, and the tool's refusal is a
line on stderr that nobody reads. That is the whole reason the outcome is published rather than
thrown: a QML exception in a settings window takes the window down, and the user who just changed
a setting is left with no way to find out why it did not change.

## How it is drawn

The design system's rules, in the terms this directory uses them.

**Monochrome, and the shape carries the state.** A switch is an outline over a well when the
setting is off and a filled bar when it is on. An enum's choice is a hollow mark when it is not
selected and a filled one when it is. A selected page in the navigation is a filled pill with its
label one weight heavier. There is no colour anywhere in this directory except the two states a
grey cannot carry: the error, which is the tool's refusal, and the warning, which is a path that
names a file that is not there.

**Layers, not boxes.** The window is the canvas, the navigation rail is one layer above it, the page
and its group of settings are one layer above that, and a field inside a row is a well. Each step is
a fill and a radius — the kit's surfaces are flat layers, and the Hub is drawn with the kit rather
than beside it. One group holds every row on a page, with a hairline between rows, inset so it reads
as a separator inside the group rather than as an edge of it. The shape before this was a card per
setting, which turns a page into a page of boxes and makes the eye find the left edge again on every
row.

**The token set, and only the token set.** Every colour, radius and size comes from this shell's
own theme: the page through the kit's `Appearance`, which is the bridge onto `Theme`, and the editors
through `Theme` itself. The bridge is the same ink, the same surfaces and the same radius scale
under the names the vendored components already use, so the two are one vocabulary and not two. A
literal in this directory is a colour that survives a theme change without being noticed.

**Every interactive control carries a name and a role.** The switch is a switch, an enum's choice
is a radio button, a field is an editable text, and the buttons are buttons.

## Checking it

```sh
sh shell/shell_test.sh
```

loads both configurations, each named by its entry file: the desktop's `shell.qml` and the Hub's
`hub.qml`. `zenith doctor` reads the same tree for the things reading can see — that nothing names a
colour of its own, that every import is on the closed list, and that every command the tree runs is a
command the tool has.

To load the Hub itself, from a checkout:

```sh
zenith config export >/dev/null          # the schemas have to be installed
quickshell -p shell/hub.qml --no-color
```

The log has to say `Configuration Loaded` and must not carry an `ERROR:` line. The warnings in it
are the machine's: a branding file that is not installed, a compositor without a layer shell.
