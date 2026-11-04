// Dashboard entry point. Boots the router and each panel controller, and paints
// the overview daemon/engine status stamps from /api/status. Everything degrades
// to dim placeholders when the daemon is absent.

import { initRouter } from "./router.js";
import { initVitals } from "./vitals.js";
import { initVault } from "./vault.js";
import { initAgents } from "./agents.js";
import { initMemory } from "./memory.js";
import { initSkills } from "./skills.js";
import { initAbout } from "./about.js";
import { initCode } from "./code.js";
import { api } from "./api.js";
import "./chat.js";

function stamp(el, ok, okText, badText) {
  if (!el) return;
  el.textContent = ok ? okText : badText;
  el.className = "stamp " + (ok ? "stamp-ok" : "stamp-bad");
}

async function paintStatus() {
  try {
    const s = await api.status();
    stamp(document.querySelector("[data-s=daemon]"), s.running, "OK", "DOWN");
    const e = s.engine || {};
    stamp(document.querySelector("[data-s=engine]"), e.available, "OK", "NONE");
    const note = document.querySelector("[data-engine-note]");
    if (note) note.textContent = e.available ? "" : e.reason || "no agent engine configured";
  } catch (err) {
    stamp(document.querySelector("[data-s=daemon]"), false, "OK", "DOWN");
    stamp(document.querySelector("[data-s=engine]"), false, "OK", "NONE");
  }
}

function boot() {
  const started = {};
  initRouter((name) => {
    if (started[name]) return;
    started[name] = true;
    if (name === "vault") initVault(document.querySelector('[data-panel="vault"]'));
    else if (name === "memory") initMemory(document.querySelector('[data-panel="memory"]'));
    else if (name === "skills") initSkills(document.querySelector('[data-panel="skills"]'));
    else if (name === "about") initAbout(document.querySelector('[data-panel="about"]'));
    else if (name === "agents") initAgents(document.querySelector('[data-panel="agents"]'));
    else if (name === "chat") {
      const el = document.querySelector('[data-panel="chat"]');
      if (typeof window.initChat === "function") window.initChat(el);
    }
  });
  initVitals(document.querySelector('[data-panel="overview"]'));
  initCode(document.querySelector('[data-panel="overview"]'));
  paintStatus();
  setInterval(paintStatus, 5000);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", boot);
} else {
  boot();
}
