// About panel: static poster copy (the WHAT / QUICK START / GO DEEPER / PRIVACY
// strings live here) with live facts from /api/about merged into THE PIECES
// table. Degrades to dim "unknown" facts when the daemon is absent; the copy
// still renders so the panel is never blank.

import { escapeHtml } from "./markdown.js";

const WHAT = [
  "Zenith Agent is the optional local assistant for Zenith.",
  "It keeps a maintained map of the machine so agents read the terrain instead of rediscovering it every session.",
  "The chat is answered by an agent engine you supply: any program that speaks the Agent Client Protocol, named by engine.command.",
];

const QUICK_START = [
  "Switch Zenith Agent on in Zenith Settings > Agent.",
  "Set engine.command in ~/.config/zenith/agent/config.json to your agent engine.",
  "Chat here or in the sidebar. With no engine, it says so and does nothing else.",
];

const GO_DEEPER = [
  ["zenith-agent -h", "daemon verbs"],
  ["zenith-agent status", "the engine, the vault and the daemon at a glance"],
  ["zenith-agent setup", "where the engine is set and how"],
  ["zenith-agent paths", "every path an agent can read"],
  ["prowl overview", "code intelligence on any repo"],
];

const PRIVACY =
  "Everything runs on this machine. The daemon binds 127.0.0.1 only, and nothing you do here leaves the box.";

async function getJSON(path) {
  const r = await fetch(path, { headers: { accept: "application/json" } });
  if (!r.ok) throw new Error(path + " -> " + r.status);
  return r.json();
}

// pieces(about) -> [{name, note, fact}] merging static one-liners with live
// facts. Missing facts fall back to a dim "unknown" so the table always fills.
function pieces(about) {
  about = about || {};
  const e = about.engine || {};
  const prowl = about.prowl || {};
  const dim = (s) => '<span class="dim">' + s + "</span>";
  const vault = about.vault ? escapeHtml(about.vault) : dim("not set");
  const daemon = about.version
    ? escapeHtml("v" + about.version) + " / :" + escapeHtml(String(about.port || 3600))
    : dim("offline");
  const engine = e.available
    ? escapeHtml((e.command || []).join(" "))
    : dim(escapeHtml(e.reason || "none configured"));
  const prowlFact = prowl.installed ? "installed" : dim("absent");
  return [
    { name: "vault", note: "the map agents read", fact: vault },
    { name: "daemon", note: "serves this dashboard + the API", fact: daemon },
    { name: "dashboard", note: "what you are looking at", fact: "this page" },
    { name: "engine", note: "the agent that answers the chat", fact: engine },
    { name: "prowl", note: "code intelligence over your repos", fact: prowlFact },
  ];
}

// ---- DOM layer (browser only) ----------------------------------------------

export function initAbout(root) {
  const body = root.querySelector("[data-about-body]");

  function render(about) {
    const what = WHAT.map((s) => "<p>" + escapeHtml(s) + "</p>").join("");
    const pieceRows = pieces(about)
      .map(
        (p) =>
          '<tr><th class="about-key">' +
          escapeHtml(p.name) +
          '</th><td class="about-note">' +
          escapeHtml(p.note) +
          '</td><td class="about-fact">' +
          p.fact +
          "</td></tr>"
      )
      .join("");
    const steps = QUICK_START.map((s, i) => {
      const btn =
        i === 1
          ? ' <button type="button" class="btn btn-primary about-setup" data-about-setup>SHOW ENGINE</button>'
          : "";
      return "<li>" + escapeHtml(s) + btn + "</li>";
    }).join("");
    const cmds = GO_DEEPER.map(
      (c) =>
        '<tr><td class="about-cmd">' +
        escapeHtml(c[0]) +
        '</td><td class="about-cmd-note">' +
        escapeHtml(c[1]) +
        "</td></tr>"
    ).join("");

    body.innerHTML =
      '<section class="about-sec"><em class="eyebrow">WHAT</em>' +
      '<div class="about-what">' +
      what +
      "</div></section>" +
      '<section class="about-sec"><em class="eyebrow">THE PIECES</em>' +
      '<table class="about-def"><tbody>' +
      pieceRows +
      "</tbody></table></section>" +
      '<section class="about-sec"><em class="eyebrow">QUICK START</em>' +
      '<ol class="about-steps">' +
      steps +
      "</ol></section>" +
      '<section class="about-sec"><em class="eyebrow">GO DEEPER</em>' +
      '<table class="about-cmds"><tbody>' +
      cmds +
      "</tbody></table></section>" +
      '<section class="about-sec"><em class="eyebrow">PRIVACY</em>' +
      '<p class="about-privacy">' +
      escapeHtml(PRIVACY) +
      "</p></section>";

    const setup = body.querySelector("[data-about-setup]");
    if (setup) setup.addEventListener("click", () => {
      location.hash = "#/agents";
    });
  }

  async function load() {
    try {
      render(await getJSON("/api/about"));
    } catch (err) {
      render(null);
    }
  }

  load();
}
