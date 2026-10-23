// Agents panel: one row per detected coding agent (name, file, wired + skill
// state, WIRE/UNWIRE). Below the list, POINT ANY AGENT shows every path Zenith Agent
// exposes plus a COPY AGENT SNIPPET for an agent it does not wire directly, and
// AGENT ENGINE shows which program answers the chat (or says plainly that none
// is configured). All best effort: an absent daemon degrades to dim placeholders.

import { api } from "./api.js";
import { escapeHtml } from "./markdown.js";

export function initAgents(root) {
  const listEl = root.querySelector("[data-agents-list]");
  const connectEl = root.querySelector("[data-connect]");
  const engineEl = root.querySelector("[data-engine]");
  let snippet = "";

  function row(a) {
    const stateChip = a.wired
      ? '<span class="stamp stamp-ok">WIRED</span>'
      : a.present
        ? '<span class="stamp stamp-idle">UNWIRED</span>'
        : '<span class="stamp stamp-bad">ABSENT</span>';
    const skill = a.skillWired ? '<span class="stamp stamp-ok">SKILL</span>' : "";
    const action = a.wired
      ? '<button class="btn btn-ghost" data-act="unwire" data-id="' + escapeHtml(a.id) + '">UNWIRE</button>'
      : '<button class="btn btn-primary" data-act="wire" data-id="' + escapeHtml(a.id) + '"' +
        (a.present ? "" : " disabled") + ">WIRE</button>";
    return (
      '<div class="agent-row" data-id="' + escapeHtml(a.id) + '">' +
      '<div class="agent-id"><span class="agent-name">' + escapeHtml(a.name) + "</span>" +
      '<span class="agent-file">' + escapeHtml(a.file || "") + "</span></div>" +
      '<div class="agent-state">' + skill + stateChip + action + "</div></div>"
    );
  }

  function pathRow(label, path, owner, ok) {
    return (
      '<div class="manifest-row' + (ok ? "" : " absent") + '">' +
      '<span class="mlabel">' + escapeHtml(label) + "</span>" +
      '<span class="mowner' + (owner === "yours" ? " mowner-yours" : "") + '">' + escapeHtml(owner || "") + "</span>" +
      '<span class="mpath">' + escapeHtml(path || "not installed") + "</span></div>"
    );
  }

  function renderConnect(m) {
    snippet = m.snippet || "";
    const rows = [
      pathRow("skill", m.skill && m.skill.path, "read-only", m.skill && m.skill.exists),
      pathRow("prowl", m.prowl && m.prowl.path, "tool", m.prowl && m.prowl.exists),
    ].concat((m.vault || []).map((v) => pathRow(v.label, v.path, v.owner, v.exists)));
    connectEl.innerHTML =
      '<h3 class="sub-title">POINT ANY AGENT</h3>' +
      '<p class="dim">Using an agent Zenith Agent does not wire for you? Point it at these, or copy a ready-made instruction block and paste it into its config.</p>' +
      rows.join("") +
      '<button class="btn btn-primary manifest-copy" data-act="copy">COPY AGENT SNIPPET</button>';
  }

  function renderEngine(e) {
    e = e || {};
    const state = e.available
      ? '<span class="stamp stamp-ok">READY</span>'
      : '<span class="stamp stamp-bad">' + (e.configured ? "UNAVAILABLE" : "NONE") + "</span>";
    const line = e.available ? (e.command || []).join(" ") : e.reason || "no agent engine configured";
    engineEl.innerHTML =
      '<h3 class="sub-title">AGENT ENGINE</h3>' +
      '<p class="dim">The program that answers the chat: any agent that speaks the Agent Client Protocol over stdio. Set <code>' +
      escapeHtml(e.key || "engine.command") + "</code> in <code>" + escapeHtml(e.config || "") + "</code>.</p>" +
      '<div class="manifest-row"><span class="mlabel">engine</span>' + state +
      '<span class="mpath">' + escapeHtml(line) + "</span></div>";
  }

  async function load() {
    try {
      const list = await api.agents();
      listEl.innerHTML = (list || []).map(row).join("");
    } catch (err) {
      listEl.innerHTML = '<p class="dim">agents unavailable, start the daemon</p>';
    }
    try {
      const m = await api.manifest();
      renderConnect(m);
      renderEngine(m.engine);
    } catch (e) {
      connectEl.innerHTML = "";
      engineEl.innerHTML = "";
    }
  }

  listEl.addEventListener("click", async (e) => {
    const btn = e.target.closest("[data-act]");
    if (!btn || btn.disabled) return;
    btn.disabled = true;
    try {
      await (btn.dataset.act === "wire" ? api.wire(btn.dataset.id) : api.unwire(btn.dataset.id));
    } catch (err) { /* reload reflects real state */ }
    await load();
  });

  connectEl.addEventListener("click", async (e) => {
    const btn = e.target.closest('[data-act="copy"]');
    if (!btn) return;
    try {
      await navigator.clipboard.writeText(snippet);
      btn.textContent = "COPIED";
      setTimeout(() => { btn.textContent = "COPY AGENT SNIPPET"; }, 1500);
    } catch (err) { /* clipboard blocked; snippet is also `zenith-agent paths` */ }
  });

  load();
}
