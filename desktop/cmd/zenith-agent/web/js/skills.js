// Skills panel: shipped and yours counts, and live-filtered collapsible groups.
// filterCategories is a pure node-tested helper; the DOM layer only wakes when a
// document exists and degrades to dim placeholders when the daemon is absent.

import { escapeHtml } from "./markdown.js";

// filterCategories(categories, query) -> categories with only matching skills,
// dropping groups that end up empty. Case-insensitive substring over skill name
// + description. Empty/blank query returns every category unchanged.
export function filterCategories(categories, query) {
  const cats = Array.isArray(categories) ? categories : [];
  const q = String(query == null ? "" : query).trim().toLowerCase();
  if (!q) return cats.slice();
  const out = [];
  for (const c of cats) {
    const skills = (c.skills || []).filter((s) => {
      const hay = ((s.name || "") + " " + (s.description || "")).toLowerCase();
      return hay.indexOf(q) !== -1;
    });
    if (skills.length) out.push({ name: c.name, skills });
  }
  return out;
}

async function getJSON(path) {
  const r = await fetch(path, { headers: { accept: "application/json" } });
  if (!r.ok) throw new Error(path + " -> " + r.status);
  return r.json();
}

// ---- DOM layer (browser only) ----------------------------------------------

export function initSkills(root) {
  const countsEl = root.querySelector("[data-skills-counts]");
  const groupsEl = root.querySelector("[data-skills-groups]");
  const searchEl = root.querySelector("[data-skills-search]");
  let categories = [];
  let debounce = 0;

  function countStamp(cls, label, n, note) {
    return (
      '<div class="skills-count">' +
      '<span class="stamp ' +
      cls +
      '">' +
      label +
      " " +
      (Number.isFinite(n) ? n : 0) +
      "</span>" +
      (note ? '<span class="skills-count-note">' + note + "</span>" : "") +
      "</div>"
    );
  }

  function renderCounts(counts) {
    counts = counts || {};
    countsEl.innerHTML =
      countStamp("stamp-idle", "SHIPPED", counts.shipped, "with Zenith") +
      countStamp("stamp-vermillion", "YOURS", counts.yours, "in the vault's skills/");
  }

  function skillRow(s) {
    const origin =
      s.origin === "yours"
        ? '<span class="stamp stamp-vermillion">' + escapeHtml(s.origin.toUpperCase()) + "</span>"
        : "";
    const ver = s.version
      ? '<span class="skills-ver">v' + escapeHtml(s.version) + "</span>"
      : "";
    return (
      '<div class="skills-row">' +
      '<span class="skills-name">' +
      escapeHtml(s.name || s.dir || "?") +
      "</span>" +
      '<span class="skills-desc dim">' +
      escapeHtml(s.description || "") +
      "</span>" +
      '<span class="skills-tags">' +
      ver +
      origin +
      "</span></div>"
    );
  }

  function renderGroups(cats, open) {
    if (!cats.length) {
      groupsEl.innerHTML = '<p class="dim">no skills match</p>';
      return;
    }
    groupsEl.innerHTML = cats
      .map((c) => {
        const rows = (c.skills || []).map(skillRow).join("");
        return (
          "<details class=\"skills-group\"" +
          (open ? " open" : "") +
          "><summary><span class=\"skills-cat\">" +
          escapeHtml(c.name || "misc") +
          '</span><span class="skills-cat-n dim">' +
          (c.skills || []).length +
          "</span></summary>" +
          '<div class="skills-rows">' +
          rows +
          "</div></details>"
        );
      })
      .join("");
  }

  function applyFilter() {
    const q = searchEl ? searchEl.value : "";
    renderGroups(filterCategories(categories, q), !!q.trim());
  }

  if (searchEl) {
    searchEl.addEventListener("input", () => {
      clearTimeout(debounce);
      debounce = setTimeout(applyFilter, 120);
    });
  }

  async function load() {
    try {
      const data = await getJSON("/api/skills");
      renderCounts(data.counts);
      categories = data.categories || [];
      renderGroups(categories, false);
    } catch (err) {
      countsEl.innerHTML = '<p class="dim">skills unavailable, start the daemon</p>';
      groupsEl.innerHTML = '<p class="dim">no skills yet</p>';
    }
  }

  load();
}
