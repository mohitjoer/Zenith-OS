import { test } from "node:test";
import assert from "node:assert/strict";
import { applyEvent, initialState } from "./chat.js";

function reduce(events) {
  return events.reduce(applyEvent, initialState());
}

test("agent_text appends to a single open message", () => {
  const s = reduce([
    { type: "agent_text", text: "Hel" },
    { type: "agent_text", text: "lo" },
  ]);
  const msgs = s.items.filter((i) => i.kind === "msg" && i.role === "agent");
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].text, "Hello");
  assert.equal(msgs[0].open, true);
});

test("turn_end closes the open message and clears busy", () => {
  const s = reduce([
    { type: "state", state: "busy" },
    { type: "agent_text", text: "hi" },
    { type: "turn_end", stopReason: "end_turn" },
  ]);
  const msg = s.items.find((i) => i.role === "agent");
  assert.equal(msg.open, false);
  assert.equal(s.busy, false);
});

test("text after turn_end starts a fresh message", () => {
  const s = reduce([
    { type: "agent_text", text: "one" },
    { type: "turn_end" },
    { type: "agent_text", text: "two" },
  ]);
  const msgs = s.items.filter((i) => i.role === "agent");
  assert.equal(msgs.length, 2);
  assert.deepEqual(msgs.map((m) => m.text), ["one", "two"]);
});

test("agent_thought accumulates on the open message", () => {
  const s = reduce([
    { type: "agent_thought", text: "plan " },
    { type: "agent_thought", text: "steps" },
    { type: "agent_text", text: "answer" },
  ]);
  const msg = s.items.find((i) => i.role === "agent");
  assert.equal(msg.thought, "plan steps");
  assert.equal(msg.text, "answer");
});

test("tool status transitions update the same card by id", () => {
  const s = reduce([
    { type: "tool", id: "t1", title: "Running ls", kind: "execute", status: "pending" },
    { type: "tool", id: "t1", status: "in_progress" },
    { type: "tool", id: "t1", status: "completed" },
  ]);
  const tools = s.items.filter((i) => i.kind === "tool");
  assert.equal(tools.length, 1);
  assert.equal(tools[0].status, "completed");
  assert.equal(tools[0].title, "Running ls", "title preserved across updates");
  assert.equal(tools[0].kind2, "execute", "kind preserved across updates");
});

test("distinct tool ids create distinct cards", () => {
  const s = reduce([
    { type: "tool", id: "t1", status: "pending" },
    { type: "tool", id: "t2", status: "pending" },
  ]);
  assert.equal(s.items.filter((i) => i.kind === "tool").length, 2);
});

test("permission queues then reply removes it", () => {
  let s = reduce([
    { type: "permission", requestId: "r1", title: "Run?", options: [{ id: "allow", name: "Allow", kind: "allow_once" }] },
  ]);
  assert.equal(s.permissions.length, 1);
  assert.equal(s.permissions[0].requestId, "r1");
  s = applyEvent(s, { type: "permission_reply", requestId: "r1" });
  assert.equal(s.permissions.length, 0);
});

test("state event sets banner and busy flag", () => {
  let s = applyEvent(initialState(), { type: "state", state: "busy", model: "engine-1" });
  assert.equal(s.banner.state, "busy");
  assert.equal(s.banner.model, "engine-1");
  assert.equal(s.busy, true);
  s = applyEvent(s, { type: "state", state: "dead", error: "no engine" });
  assert.equal(s.banner.state, "dead");
  assert.equal(s.banner.error, "no engine");
  assert.equal(s.busy, false);
});

test("user event appends a right-aligned user message", () => {
  const s = applyEvent(initialState(), { type: "user", text: "hey" });
  assert.equal(s.items.length, 1);
  assert.equal(s.items[0].role, "user");
  assert.equal(s.items[0].text, "hey");
});

test("applyEvent does not mutate the input state", () => {
  const s0 = initialState();
  const s1 = applyEvent(s0, { type: "user", text: "x" });
  assert.equal(s0.items.length, 0, "original items untouched");
  assert.notEqual(s0, s1);
});

test("unknown event returns state unchanged", () => {
  const s0 = initialState();
  const s1 = applyEvent(s0, { type: "mystery" });
  assert.equal(s0, s1);
});

test("models frame sets model list and current", () => {
  const s = applyEvent(initialState(), {
    type: "models",
    current: "openai-codex:gpt-5.5",
    models: [{ id: "openai-codex:gpt-5.5", name: "GPT 5.5", description: "flagship" }],
  });
  assert.equal(s.currentModel, "openai-codex:gpt-5.5");
  assert.equal(s.models.length, 1);
  assert.equal(s.models[0].name, "GPT 5.5");
});

test("commands frame stores the legend list", () => {
  const s = applyEvent(initialState(), {
    type: "commands",
    commands: [{ name: "help", description: "show help", hint: "" }],
  });
  assert.equal(s.commands.length, 1);
  assert.equal(s.commands[0].name, "help");
});

test("session_info frame captures id and title", () => {
  const s = applyEvent(initialState(), { type: "session_info", sessionId: "abc123", title: "Debugging" });
  assert.equal(s.session.id, "abc123");
  assert.equal(s.session.title, "Debugging");
});

test("usage frame stores size and used", () => {
  const s = applyEvent(initialState(), { type: "usage", size: 200000, used: 16042 });
  assert.equal(s.usage.size, 200000);
  assert.equal(s.usage.used, 16042);
});

test("history frame stores the session list", () => {
  const s = applyEvent(initialState(), {
    type: "history",
    sessions: [{ id: "s1", title: "One", updatedAt: "2026-07-02T00:00:00Z", cwd: "/tmp" }],
  });
  assert.equal(s.history.length, 1);
  assert.equal(s.history[0].title, "One");
});

test("replay_start clears the stream and sets replaying", () => {
  let s = reduce([
    { type: "user", text: "old" },
    { type: "agent_text", text: "prior" },
    { type: "turn_end" },
  ]);
  assert.equal(s.items.length, 2);
  s = applyEvent(s, { type: "replay_start" });
  assert.equal(s.items.length, 0, "stream cleared for replay");
  assert.equal(s.replaying, true);
});

test("during replay user_text appends a user message and turn_end keeps busy", () => {
  const s = reduce([
    { type: "state", state: "busy" },
    { type: "replay_start" },
    { type: "user_text", text: "hello from history" },
    { type: "agent_text", text: "a reply" },
    { type: "turn_end" },
    { type: "replay_end" },
  ]);
  const users = s.items.filter((i) => i.role === "user");
  const agents = s.items.filter((i) => i.role === "agent");
  assert.equal(users.length, 1);
  assert.equal(users[0].text, "hello from history");
  assert.equal(agents.length, 1);
  assert.equal(agents[0].open, false, "replayed agent turn is closed");
  assert.equal(s.busy, true, "turn_end during replay does not clear busy");
  assert.equal(s.replaying, false, "replay_end lowers the flag");
});

test("turn_end outside replay still clears busy", () => {
  const s = reduce([
    { type: "state", state: "busy" },
    { type: "agent_text", text: "hi" },
    { type: "turn_end" },
  ]);
  assert.equal(s.busy, false);
});

test("user event carries images through the reducer", () => {
  const s = applyEvent(initialState(), {
    type: "user", text: "look", images: [{ data: "AAA", mimeType: "image/jpeg" }],
  });
  assert.equal(s.items[0].images.length, 1);
  assert.equal(s.items[0].images[0].mimeType, "image/jpeg");
});

test("v2 fields survive an unrelated frame without mutation", () => {
  const base = applyEvent(initialState(), { type: "usage", size: 100, used: 10 });
  const next = applyEvent(base, { type: "agent_text", text: "x" });
  assert.equal(next.usage.size, 100, "usage preserved across frames");
  assert.equal(base.items.length, 0, "prior state not mutated");
});

test("activity tracks tools, thinking, writing, and clears at turn_end", () => {
  let s = reduce([
    { type: "state", state: "busy" },
    { type: "tool", id: "t1", title: "Reading desktop.md", status: "in_progress" },
  ]);
  assert.equal(s.activity, "Reading desktop.md");
  s = applyEvent(s, { type: "agent_thought", text: "hmm" });
  assert.equal(s.activity, "thinking");
  s = applyEvent(s, { type: "agent_text", text: "The config" });
  assert.equal(s.activity, "writing");
  s = applyEvent(s, { type: "tool", id: "t1", status: "completed" });
  assert.equal(s.activity, "writing", "completion keeps the last label");
  s = applyEvent(s, { type: "permission", requestId: "r1", title: "Run ls", options: [] });
  assert.equal(s.activity, "waiting for your approval");
  s = applyEvent(s, { type: "turn_end", stopReason: "end_turn" });
  assert.equal(s.activity, "");
});

test("activity stays quiet during replay", () => {
  const s = reduce([
    { type: "replay_start" },
    { type: "tool", id: "t1", title: "old tool", status: "in_progress" },
    { type: "agent_text", text: "old" },
  ]);
  assert.equal(s.activity, "");
});
