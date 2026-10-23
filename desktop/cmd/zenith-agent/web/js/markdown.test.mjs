import { test } from "node:test";
import assert from "node:assert/strict";
import { mdToHtml, escapeHtml } from "./markdown.js";

test("escapes html entities", () => {
  assert.equal(escapeHtml("a < b & c > d"), "a &lt; b &amp; c &gt; d");
});

test("hostile script input stays escaped", () => {
  const html = mdToHtml("<script>alert('x')</script>");
  assert.ok(!html.includes("<script>"), "raw script tag must not survive");
  assert.ok(html.includes("&lt;script&gt;"), "angle brackets escaped");
});

test("html comment-only lines are hidden, inline comments stay escaped text", () => {
  const html = mdToHtml("<!-- zenith:generated:begin -->\nbody\n<!-- zenith:generated:end -->");
  assert.equal(html, "<p>body</p>");
  assert.ok(mdToHtml("before <!-- x --> after").includes("&lt;!-- x --&gt;"));
});

test("headings render by level", () => {
  assert.equal(mdToHtml("# Title"), "<h1>Title</h1>");
  assert.equal(mdToHtml("### Deep"), "<h3>Deep</h3>");
  assert.equal(mdToHtml("##### TooDeep"), "<p>##### TooDeep</p>");
});

test("bold and italic and inline code", () => {
  assert.equal(mdToHtml("**b** and *i*"), "<p><strong>b</strong> and <em>i</em></p>");
  assert.equal(mdToHtml("use `x = 1` here"), "<p>use <code>x = 1</code> here</p>");
});

test("inline code content is escaped, not transformed", () => {
  const html = mdToHtml("`<b>*no*</b>`");
  assert.ok(html.includes("<code>&lt;b&gt;*no*&lt;/b&gt;</code>"));
});

test("fenced code block", () => {
  const html = mdToHtml("```\nline1\n<tag>\n```");
  assert.equal(html, "<pre><code>line1\n&lt;tag&gt;</code></pre>");
});

test("http links allowed, javascript scheme rejected", () => {
  assert.equal(
    mdToHtml("[go](https://x.io)"),
    '<p><a href="https://x.io" target="_blank" rel="noopener">go</a></p>',
  );
  assert.equal(mdToHtml("[anchor](#top)"), '<p><a href="#top">anchor</a></p>');
  const bad = mdToHtml("[x](javascript:alert(1))");
  assert.ok(!bad.includes("<a "), "javascript scheme must not become a link");
  assert.ok(bad.includes("[x]"), "raw text preserved for rejected link");
});

test("bare url becomes a target=_blank link", () => {
  const html = mdToHtml("see https://example.com/docs for more");
  assert.ok(
    html.includes('<a href="https://example.com/docs" target="_blank" rel="noopener">https://example.com/docs</a>'),
    "bare url linkified",
  );
});

test("trailing paren and period are peeled off a bare url", () => {
  const html = mdToHtml("(see https://example.com/x).");
  assert.ok(
    html.includes('<a href="https://example.com/x" target="_blank" rel="noopener">https://example.com/x</a>).'),
    "punctuation stays as text, href is clean",
  );
  assert.ok(!html.includes('href="https://example.com/x).'), "trailing chars not in href");
});

test("bare url inside inline code is left untouched", () => {
  const html = mdToHtml("run `curl https://example.com` now");
  assert.ok(html.includes("<code>curl https://example.com</code>"), "code content intact");
  assert.ok(!html.includes("<a "), "no link created inside code");
});

test("bare javascript scheme is never linkified", () => {
  const html = mdToHtml("javascript:alert(1) is inert");
  assert.ok(!html.includes("<a "), "only http(s) autolinks");
});

test("pipe table renders thead and tbody", () => {
  const html = mdToHtml("| A | B |\n| - | - |\n| 1 | 2 |");
  assert.ok(html.includes("<table>"));
  assert.ok(html.includes("<th>A</th><th>B</th>"));
  assert.ok(html.includes("<td>1</td><td>2</td>"));
});

test("horizontal rule and lists", () => {
  assert.equal(mdToHtml("---"), "<hr>");
  assert.equal(mdToHtml("- a\n- b"), "<ul><li>a</li><li>b</li></ul>");
  assert.equal(mdToHtml("1. a\n2. b"), "<ol><li>a</li><li>b</li></ol>");
});

test("blank line separates paragraphs", () => {
  assert.equal(mdToHtml("one\n\ntwo"), "<p>one</p>\n<p>two</p>");
});
