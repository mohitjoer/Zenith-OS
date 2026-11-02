// Hash router: maps #/<panel> to the visible panel and the active nav item, and
// plays a clip-path wipe on swap (skipped under prefers-reduced-motion).

const PANELS = ["overview", "vault", "memory", "skills", "agents", "chat", "about"];
const reduce = () =>
  typeof matchMedia !== "undefined" &&
  matchMedia("(prefers-reduced-motion: reduce)").matches;

function current() {
  const h = location.hash.replace(/^#\/?/, "");
  return PANELS.includes(h) ? h : "overview";
}

export function initRouter(onChange) {
  const panels = document.querySelectorAll("[data-panel]");
  const links = document.querySelectorAll("[data-nav]");

  function show(name) {
    panels.forEach((p) => {
      const active = p.dataset.panel === name;
      p.hidden = !active;
      if (active && !reduce()) {
        p.classList.remove("wipe-in");
        void p.offsetWidth; // restart the animation
        p.classList.add("wipe-in");
        // A throttled tab can freeze the animation at t=0 with the panel
        // fully clipped (and unclickable). Drop the class once it ends, and
        // unconditionally after its duration, so the wipe can never wedge.
        const clear = () => p.classList.remove("wipe-in");
        p.addEventListener("animationend", clear, { once: true });
        setTimeout(clear, 700);
      }
    });
    links.forEach((l) => l.classList.toggle("active", l.dataset.nav === name));
    if (onChange) onChange(name);
  }

  addEventListener("hashchange", () => show(current()));
  show(current());
}
