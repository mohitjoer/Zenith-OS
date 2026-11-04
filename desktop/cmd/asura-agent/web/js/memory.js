// Memory panel: the vault seen as data. Pure helpers (buildGraphModel,
// layoutStep, bucketHeatmap) are node-tested; the DOM + canvas layer only wakes
// when a document exists and degrades to dim placeholders when the daemon is
// absent. The force-directed graph runs a rAF loop that pauses once settled or
// when the panel is hidden.

import { escapeHtml } from "./markdown.js";

const MONO = "'JetBrains Mono', monospace";
const MONTHS = ["J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"];

// ---- pure helpers (node-testable) ------------------------------------------

// buildGraphModel(apiJson) -> {nodes, links}. Accepts the whole /api/memory
// payload or a bare {nodes, links} graph. Tolerates a missing/empty graph and
// drops links whose endpoints are absent. Seeds deterministic ring positions so
// a standalone caller gets a sane starting layout.
export function buildGraphModel(apiJson) {
  const graph = apiJson && apiJson.graph ? apiJson.graph : apiJson || {};
  const rawNodes = Array.isArray(graph.nodes) ? graph.nodes : [];
  const rawLinks = Array.isArray(graph.links) ? graph.links : [];
  const R = 140;
  const nodes = rawNodes.map((n, i) => {
    const a = (i / Math.max(1, rawNodes.length)) * Math.PI * 2;
    const id = String(n.id != null ? n.id : i);
    return {
      id,
      label: String(n.label != null ? n.label : id),
      group: n.group || "",
      size: Number.isFinite(n.size) ? n.size : 1,
      x: Math.cos(a) * R,
      y: Math.sin(a) * R,
      vx: 0,
      vy: 0,
      fixed: false,
    };
  });
  const ids = new Set(nodes.map((n) => n.id));
  const links = rawLinks
    .map((l) => ({ source: String(l.source), target: String(l.target) }))
    .filter((l) => l.source !== l.target && ids.has(l.source) && ids.has(l.target));
  return { nodes, links };
}

// layoutStep(nodes, links, opts) -> kinetic energy. One physics tick, mutating
// node x/y/vx/vy in place: O(n^2) repulsion, spring links toward a rest length,
// center gravity, then velocity damping. Fixed nodes (dragged) hold position.
// Links may reference node ids (resolved here) or node objects directly.
export function layoutStep(nodes, links, opts) {
  opts = opts || {};
  const width = opts.width || 600;
  const height = opts.height || 420;
  const repulsion = opts.repulsion != null ? opts.repulsion : 1600;
  const spring = opts.spring != null ? opts.spring : 0.02;
  const springLength = opts.springLength != null ? opts.springLength : 70;
  const gravity = opts.gravity != null ? opts.gravity : 0.012;
  const damping = opts.damping != null ? opts.damping : 0.85;
  const maxV = opts.maxVelocity != null ? opts.maxVelocity : 40;
  const cx = opts.center ? opts.center.x : width / 2;
  const cy = opts.center ? opts.center.y : height / 2;
  if (!nodes || !nodes.length) return 0;

  const byId = {};
  for (const n of nodes) {
    byId[n.id] = n;
    n._fx = 0;
    n._fy = 0;
  }

  for (let i = 0; i < nodes.length; i++) {
    const a = nodes[i];
    for (let j = i + 1; j < nodes.length; j++) {
      const b = nodes[j];
      let dx = a.x - b.x;
      let dy = a.y - b.y;
      let d2 = dx * dx + dy * dy;
      if (d2 < 0.01) {
        dx = Math.random() - 0.5;
        dy = Math.random() - 0.5;
        d2 = dx * dx + dy * dy + 0.01;
      }
      const dist = Math.sqrt(d2);
      const f = repulsion / d2;
      const ux = dx / dist;
      const uy = dy / dist;
      a._fx += ux * f;
      a._fy += uy * f;
      b._fx -= ux * f;
      b._fy -= uy * f;
    }
  }

  for (const l of links) {
    const s = typeof l.source === "object" ? l.source : byId[l.source];
    const t = typeof l.target === "object" ? l.target : byId[l.target];
    if (!s || !t) continue;
    const dx = t.x - s.x;
    const dy = t.y - s.y;
    const dist = Math.sqrt(dx * dx + dy * dy) || 0.01;
    const f = spring * (dist - springLength);
    const ux = dx / dist;
    const uy = dy / dist;
    s._fx += ux * f;
    s._fy += uy * f;
    t._fx -= ux * f;
    t._fy -= uy * f;
  }

  let energy = 0;
  for (const n of nodes) {
    n._fx += (cx - n.x) * gravity;
    n._fy += (cy - n.y) * gravity;
    if (n.fixed) {
      n.vx = 0;
      n.vy = 0;
      continue;
    }
    n.vx = (n.vx + n._fx) * damping;
    n.vy = (n.vy + n._fy) * damping;
    if (n.vx > maxV) n.vx = maxV;
    else if (n.vx < -maxV) n.vx = -maxV;
    if (n.vy > maxV) n.vy = maxV;
    else if (n.vy < -maxV) n.vy = -maxV;
    n.x += n.vx;
    n.y += n.vy;
    energy += n.vx * n.vx + n.vy * n.vy;
  }
  return energy;
}

function parseDay(s) {
  if (!s) return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(s));
  if (!m) return null;
  return new Date(Date.UTC(+m[1], +m[2] - 1, +m[3]));
}
function todayUTC() {
  const n = new Date();
  return new Date(Date.UTC(n.getFullYear(), n.getMonth(), n.getDate()));
}
function addDays(d, n) {
  return new Date(d.getTime() + n * 86400000);
}
function isoDay(d) {
  return d.toISOString().slice(0, 10);
}

// bucketHeatmap(entries, weeks, endDate) -> {weeks, days}. GitHub-style grid of
// weeks*7 cells, chronological (Sunday-first per column), aligned so the last
// non-future cell is endDate (defaults to today). Fills gaps with count 0 and
// flags cells past endDate as future. endDate is injectable for tests.
export function bucketHeatmap(entries, weeks, endDate) {
  weeks = weeks || 26;
  const counts = {};
  if (Array.isArray(entries)) {
    for (const e of entries) {
      if (e && e.date) counts[e.date] = (counts[e.date] || 0) + (Number(e.count) || 0);
    }
  }
  const end = parseDay(endDate) || todayUTC();
  const gridEnd = addDays(end, 6 - end.getUTCDay()); // Saturday of end's week
  const total = weeks * 7;
  const gridStart = addDays(gridEnd, -(total - 1)); // a Sunday
  const days = [];
  for (let i = 0; i < total; i++) {
    const d = addDays(gridStart, i);
    const ds = isoDay(d);
    const future = d.getTime() > end.getTime();
    days.push({
      date: ds,
      count: future ? 0 : counts[ds] || 0,
      dow: i % 7,
      week: Math.floor(i / 7),
      future,
    });
  }
  return { weeks, days };
}

function heatLevel(c) {
  if (c <= 0) return "0";
  if (c === 1) return "1";
  if (c <= 3) return "2";
  return "3";
}

function trunc(s, n) {
  s = String(s == null ? "" : s);
  return s.length > n ? s.slice(0, n - 1) + "\u2026" : s;
}

async function getJSON(path) {
  const r = await fetch(path, { headers: { accept: "application/json" } });
  if (!r.ok) throw new Error(path + " -> " + r.status);
  return r.json();
}

// ---- DOM + canvas layer (browser only) -------------------------------------

export function initMemory(root) {
  const tilesEl = root.querySelector("[data-mem-tiles]");
  const canvas = root.querySelector("[data-mem-graph]");
  const heatEl = root.querySelector("[data-mem-heatmap]");
  const ctx = canvas ? canvas.getContext("2d") : null;
  const reduce =
    typeof matchMedia !== "undefined" &&
    matchMedia("(prefers-reduced-motion: reduce)").matches;

  const C = readColors();
  const size = { w: 600, h: 420 };
  const center = { x: 300, y: 210 };
  let model = null;
  let byId = {};
  let adj = {};
  let lastFetch = 0;
  let loading = false;
  let raf = 0;
  let warmup = 0;
  let settled = false;
  let settleFrames = 0;
  let hover = null;
  let drag = null;
  const SETTLE = 0.6;
  const SETTLE_FRAMES = 20;

  function readColors() {
    const cs = getComputedStyle(document.documentElement);
    const g = (name, fb) => (cs.getPropertyValue(name) || "").trim() || fb;
    return {
      ink: g("--ink", "#e8d8c9"),
      red: g("--red", "#C94E44"),
      tan: g("--tan", "#CDA47B"),
      teal: g("--teal", "#3E6868"),
      slate: g("--slate", "#4b607f"),
      orange: g("--orange", "#f3701e"),
      dim: g("--ink-dim", "#8f8378"),
      line: g("--line", "rgba(232,216,201,.18)"),
      paper: g("--paper", "#0e0d0b"),
    };
  }

  function nodeStyle(n) {
    switch (n.group) {
      case "hub":
        return { fill: C.red, ring: null };
      case "generated":
        return { fill: C.tan, ring: null };
      case "memory":
        return { fill: C.teal, ring: null };
      case "journal":
        return { fill: C.slate, ring: null };
      default:
        return { fill: C.ink, ring: null };
    }
  }

  function radius(n) {
    const base = 5 + Math.log((n.size || 1) + 1) * 2;
    const r = Math.max(5, Math.min(22, base));
    return n.group === "hub" ? r * 1.5 : r;
  }

  // ---- tiles / heatmap ----

  function tile(accent, label, main, sub, isStamp) {
    const body = isStamp
      ? '<div class="mem-tile-stamp">' + main + "</div>"
      : '<b class="stat-num mem-tile-num">' + escapeHtml(main) + "</b>";
    const subEl = isStamp
      ? sub
      : '<span class="stat-sub">' + escapeHtml(sub) + "</span>";
    return (
      '<div class="stat mem-tile" data-accent="' +
      accent +
      '"><span class="stat-corner" aria-hidden="true"></span>' +
      '<em class="eyebrow">' +
      label +
      "</em>" +
      body +
      subEl +
      "</div>"
    );
  }

  function renderTiles(data) {
    const learned = data.learned || {};
    const nodes = ((data.graph || {}).nodes || []).length;
    tilesEl.innerHTML =
      tile("vermillion", "VAULT FILES", String(nodes), "markdown notes and maps") +
      tile("teal", "AGENT NOTES", String(learned.vaultNotes || 0), "in memory/") +
      tile("tan", "JOURNAL DAYS", String(learned.journalDays || 0), "active");
  }

  function renderHeatmap(entries) {
    const m = bucketHeatmap(entries, 26);
    let months = "";
    let prev = -1;
    for (let c = 0; c < m.weeks; c++) {
      const d = m.days[c * 7];
      const mo = new Date(d.date + "T00:00:00Z").getUTCMonth();
      let lbl = "";
      if (mo !== prev) {
        lbl = MONTHS[mo];
        prev = mo;
      }
      months += '<span class="mem-hm-mon">' + lbl + "</span>";
    }
    let cells = "";
    for (const day of m.days) {
      if (day.future) {
        cells += '<span class="mem-cell" data-level="e"></span>';
        continue;
      }
      cells +=
        '<span class="mem-cell" data-level="' +
        heatLevel(day.count) +
        '" title="' +
        day.date +
        ": " +
        day.count +
        '"></span>';
    }
    heatEl.innerHTML =
      '<div class="mem-hm-months">' +
      months +
      '</div><div class="mem-hm-grid">' +
      cells +
      "</div>";
  }

  // ---- graph canvas ----

  function buildIndex() {
    byId = {};
    adj = {};
    for (const n of model.nodes) {
      byId[n.id] = n;
      adj[n.id] = new Set([n.id]);
    }
    for (const l of model.links) {
      if (adj[l.source] && adj[l.target]) {
        adj[l.source].add(l.target);
        adj[l.target].add(l.source);
      }
    }
  }

  function neighbors(id) {
    return adj[id] || new Set([id]);
  }

  function resize() {
    if (!canvas || !ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth || (canvas.parentElement && canvas.parentElement.clientWidth) || 600;
    const h = canvas.clientHeight || 420;
    size.w = w;
    size.h = h;
    center.x = w / 2;
    center.y = h / 2;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    if (model) {
      if (settled || reduce) draw();
      else wake();
    }
  }

  function seedPositions() {
    const n = model.nodes.length;
    const R = Math.min(size.w, size.h) * 0.32;
    model.nodes.forEach((nd, i) => {
      const a = (i / Math.max(1, n)) * Math.PI * 2;
      const spread = R * (0.6 + 0.4 * ((i % 5) / 5));
      nd.x = center.x + Math.cos(a) * spread;
      nd.y = center.y + Math.sin(a) * spread;
      nd.vx = 0;
      nd.vy = 0;
    });
  }

  function drawEmpty(msg) {
    if (!ctx) return;
    ctx.clearRect(0, 0, size.w, size.h);
    ctx.fillStyle = C.dim;
    ctx.font = "11px " + MONO;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(msg, size.w / 2, size.h / 2);
  }

  function draw() {
    if (!ctx) return;
    ctx.clearRect(0, 0, size.w, size.h);
    const nodes = model ? model.nodes : [];
    if (!nodes.length) {
      drawEmpty("no notes yet");
      return;
    }
    const hoverSet = hover ? neighbors(hover) : null;

    ctx.save();
    ctx.setLineDash([3, 3]);
    ctx.lineWidth = 1;
    ctx.strokeStyle = C.line;
    for (const l of model.links) {
      const s = byId[l.source];
      const t = byId[l.target];
      if (!s || !t) continue;
      const touches = hover && (l.source === hover || l.target === hover);
      ctx.globalAlpha = hoverSet ? (touches ? 0.9 : 0.08) : 0.5;
      ctx.beginPath();
      ctx.moveTo(s.x, s.y);
      ctx.lineTo(t.x, t.y);
      ctx.stroke();
    }
    ctx.restore();

    const showAll = nodes.length <= 80;
    for (const n of nodes) {
      const st = nodeStyle(n);
      const r = radius(n);
      const dim = hoverSet && !hoverSet.has(n.id);
      ctx.globalAlpha = dim ? 0.25 : 1;
      ctx.beginPath();
      ctx.arc(n.x, n.y, r, 0, Math.PI * 2);
      ctx.fillStyle = st.fill;
      ctx.fill();
      if (st.ring) {
        ctx.lineWidth = 2;
        ctx.strokeStyle = st.ring;
        ctx.stroke();
      }
      if (showAll || (hoverSet && hoverSet.has(n.id))) {
        ctx.globalAlpha = dim ? 0.25 : 0.85;
        ctx.fillStyle = C.dim;
        ctx.font = "10px " + MONO;
        ctx.textAlign = "center";
        ctx.textBaseline = "top";
        ctx.fillText(trunc(n.label, 16), n.x, n.y + r + 3);
      }
    }
    ctx.globalAlpha = 1;
  }

  function frame() {
    raf = 0;
    if (!isVisible()) return;
    const energy = layoutStep(model.nodes, model.links, {
      width: size.w,
      height: size.h,
      center,
    });
    draw();
    if (drag) {
      raf = requestAnimationFrame(frame);
      return;
    }
    if (warmup > 0) {
      warmup--;
      raf = requestAnimationFrame(frame);
      return;
    }
    if (energy < SETTLE) settleFrames++;
    else settleFrames = 0;
    if (settleFrames > SETTLE_FRAMES) {
      settled = true;
      return;
    }
    raf = requestAnimationFrame(frame);
  }

  function wake() {
    if (reduce) return;
    if (!isVisible()) return;
    if (!model || !model.nodes.length) {
      drawEmpty("no notes yet");
      return;
    }
    if (!raf) raf = requestAnimationFrame(frame);
  }

  function stop() {
    if (raf) {
      cancelAnimationFrame(raf);
      raf = 0;
    }
  }

  function isVisible() {
    return !root.hidden && document.visibilityState !== "hidden";
  }

  // ---- pointer interactions ----

  function pt(e) {
    const rect = canvas.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }
  function hit(p) {
    if (!model) return null;
    for (let i = model.nodes.length - 1; i >= 0; i--) {
      const n = model.nodes[i];
      if (Math.hypot(p.x - n.x, p.y - n.y) <= radius(n) + 4) return n;
    }
    return null;
  }

  if (canvas) {
    canvas.addEventListener("pointerdown", (e) => {
      const p = pt(e);
      const n = hit(p);
      if (!n) return;
      drag = { node: n, moved: false, sx: p.x, sy: p.y };
      n.fixed = true;
      n.x = p.x;
      n.y = p.y;
      try {
        canvas.setPointerCapture(e.pointerId);
      } catch (_) {
        /* noop */
      }
      settled = false;
      settleFrames = 0;
      wake();
    });
    canvas.addEventListener("pointermove", (e) => {
      const p = pt(e);
      if (drag) {
        drag.node.x = p.x;
        drag.node.y = p.y;
        if (Math.hypot(p.x - drag.sx, p.y - drag.sy) > 4) drag.moved = true;
        settled = false;
        settleFrames = 0;
        wake();
        return;
      }
      const n = hit(p);
      const id = n ? n.id : null;
      if (id !== hover) {
        hover = id;
        canvas.style.cursor = n ? "pointer" : "default";
        if (settled) draw();
      }
    });
    const endDrag = (e) => {
      if (!drag) return;
      const d = drag;
      drag = null;
      d.node.fixed = false;
      try {
        canvas.releasePointerCapture(e.pointerId);
      } catch (_) {
        /* noop */
      }
      if (!d.moved && d.node.id.slice(-3) === ".md") location.hash = "#/vault";
      settled = false;
      settleFrames = 0;
      wake();
    };
    canvas.addEventListener("pointerup", endDrag);
    canvas.addEventListener("pointercancel", endDrag);
    canvas.addEventListener("pointerleave", () => {
      if (hover && !drag) {
        hover = null;
        if (settled) draw();
      }
    });
    if (typeof ResizeObserver !== "undefined") {
      new ResizeObserver(() => resize()).observe(canvas);
    }
  }

  // ---- data flow ----

  function applyData(data) {
    renderTiles(data);
    renderHeatmap(data.heatmap || []);
    model = buildGraphModel(data);
    buildIndex();
    resize();
    seedPositions();
    if (!model.nodes.length) {
      drawEmpty("no notes yet");
      return;
    }
    if (reduce) {
      for (let i = 0; i < 120; i++) {
        layoutStep(model.nodes, model.links, { width: size.w, height: size.h, center });
      }
      settled = true;
      draw();
      return;
    }
    warmup = 60;
    settled = false;
    settleFrames = 0;
    wake();
  }

  async function fetchMemory(force) {
    if (loading) return;
    const now = Date.now();
    if (!force && model && now - lastFetch < 60000) return;
    loading = true;
    try {
      const data = await getJSON("/api/memory");
      lastFetch = Date.now();
      applyData(data);
    } catch (err) {
      if (!model) {
        tilesEl.innerHTML = '<p class="dim">memory unavailable, start the daemon</p>';
        heatEl.innerHTML = '<p class="dim">no activity yet</p>';
        drawEmpty("the daemon is offline");
      }
    } finally {
      loading = false;
    }
  }

  function onRoute() {
    if (isVisible()) {
      fetchMemory(false);
      wake();
    } else {
      stop();
    }
  }

  addEventListener("hashchange", onRoute);
  document.addEventListener("visibilitychange", onRoute);
  resize();
  drawEmpty("loading...");
  onRoute();
}
