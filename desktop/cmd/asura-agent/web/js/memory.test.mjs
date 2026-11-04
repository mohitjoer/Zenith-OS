import { test } from "node:test";
import assert from "node:assert/strict";
import { buildGraphModel, layoutStep, bucketHeatmap } from "./memory.js";

function dist(a, b) {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

test("buildGraphModel tolerates an empty graph", () => {
  for (const input of [undefined, null, {}, { graph: {} }, { graph: { nodes: [], links: [] } }]) {
    const m = buildGraphModel(input);
    assert.deepEqual(m.nodes, []);
    assert.deepEqual(m.links, []);
  }
});

test("buildGraphModel normalizes nodes and drops dangling links", () => {
  const m = buildGraphModel({
    graph: {
      nodes: [{ id: "AGENTS.md", label: "AGENTS", group: "hub", size: 1197 }, { id: "a.md" }],
      links: [
        { source: "AGENTS.md", target: "a.md" },
        { source: "AGENTS.md", target: "ghost.md" }, // dangling -> dropped
        { source: "a.md", target: "a.md" }, // self -> dropped
      ],
    },
  });
  assert.equal(m.nodes.length, 2);
  assert.equal(m.nodes[1].label, "a.md"); // label defaults to id
  assert.equal(m.links.length, 1);
  assert.equal(m.links[0].target, "a.md");
});

test("layoutStep moves connected nodes closer over 50 ticks", () => {
  const nodes = [
    { id: "a", x: -250, y: 0, vx: 0, vy: 0, size: 1 },
    { id: "b", x: 250, y: 0, vx: 0, vy: 0, size: 1 },
  ];
  const links = [{ source: "a", target: "b" }];
  const opts = { width: 600, height: 420, center: { x: 0, y: 0 } };
  const before = dist(nodes[0], nodes[1]);
  for (let i = 0; i < 50; i++) layoutStep(nodes, links, opts);
  const after = dist(nodes[0], nodes[1]);
  assert.ok(after < before, "expected " + after + " < " + before);
  assert.ok(after < 200, "connected nodes should pull near the spring length, got " + after);
});

test("layoutStep respects fixed (dragged) nodes", () => {
  const nodes = [
    { id: "a", x: 0, y: 0, vx: 0, vy: 0, size: 1, fixed: true },
    { id: "b", x: 100, y: 0, vx: 0, vy: 0, size: 1 },
  ];
  layoutStep(nodes, [{ source: "a", target: "b" }], { center: { x: 300, y: 210 } });
  assert.equal(nodes[0].x, 0);
  assert.equal(nodes[0].y, 0);
});

test("layoutStep on no nodes returns zero energy", () => {
  assert.equal(layoutStep([], [], {}), 0);
});

test("bucketHeatmap builds a full weeks*7 grid ending today", () => {
  const m = bucketHeatmap([], 26, "2026-07-02"); // a Thursday
  assert.equal(m.weeks, 26);
  assert.equal(m.days.length, 26 * 7);
  assert.equal(m.days[0].dow, 0, "grid starts on a Sunday");
  const past = m.days.filter((d) => !d.future);
  assert.equal(past[past.length - 1].date, "2026-07-02", "last non-future cell is the end date");
});

test("bucketHeatmap fills gaps and applies provided counts", () => {
  const m = bucketHeatmap(
    [
      { date: "2026-07-02", count: 3 },
      { date: "2026-06-30", count: 1 },
    ],
    26,
    "2026-07-02"
  );
  const byDate = Object.fromEntries(m.days.map((d) => [d.date, d]));
  assert.equal(byDate["2026-07-02"].count, 3);
  assert.equal(byDate["2026-06-30"].count, 1);
  assert.equal(byDate["2026-07-01"].count, 0, "gap day filled with zero");
});

test("bucketHeatmap flags days after the end date as future", () => {
  const m = bucketHeatmap([{ date: "2026-07-05", count: 9 }], 26, "2026-07-02");
  const future = m.days.filter((d) => d.future);
  assert.ok(future.length > 0, "days past today exist in the trailing week");
  for (const f of future) assert.equal(f.count, 0, "future cells carry no count");
});
