import { test } from "node:test";
import assert from "node:assert/strict";
import { filterCategories } from "./skills.js";

const cats = [
  {
    name: "software-development",
    skills: [
      { name: "test-driven-development", description: "write tests first" },
      { name: "systematic-debugging", description: "root-cause failures" },
    ],
  },
  {
    name: "writing",
    skills: [{ name: "writing-plans", description: "plan before code" }],
  },
];

test("blank query returns every category unchanged", () => {
  for (const q of ["", "   ", null, undefined]) {
    const out = filterCategories(cats, q);
    assert.equal(out.length, 2);
    assert.equal(out[0].skills.length, 2);
  }
});

test("filters by name substring, case-insensitive", () => {
  const out = filterCategories(cats, "DEBUG");
  assert.equal(out.length, 1);
  assert.equal(out[0].name, "software-development");
  assert.equal(out[0].skills.length, 1);
  assert.equal(out[0].skills[0].name, "systematic-debugging");
});

test("filters by description substring", () => {
  const out = filterCategories(cats, "plan before");
  assert.equal(out.length, 1);
  assert.equal(out[0].name, "writing");
});

test("drops categories with zero matches", () => {
  const out = filterCategories(cats, "zzz-nothing");
  assert.deepEqual(out, []);
});

test("tolerates missing skills arrays and non-array input", () => {
  assert.deepEqual(filterCategories(null, "x"), []);
  assert.deepEqual(filterCategories([{ name: "empty" }], "x"), []);
});
