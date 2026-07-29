(() => {
  "use strict";

  const root = document.documentElement;
  const summary = document.querySelector("#evidence-summary");
  const controls = {
    product: document.querySelectorAll("[data-product]"),
    mode: document.querySelectorAll("[data-mode]"),
    scheme: document.querySelectorAll("[data-scheme]"),
  };
  const defaults = {};
  const allowed = {};
  const state = {};
  const query = new URLSearchParams(window.location.search);

  for (const [key, buttons] of Object.entries(controls)) {
    const values = Array.from(buttons, (button) => button.dataset[key]);
    defaults[key] = values[0];
    allowed[key] = new Set(values);
    const requested = query.get(key);
    state[key] = allowed[key].has(requested) ? requested : defaults[key];
  }

  const matches = (evidence) => {
    const product = evidence.dataset.proofProduct;
    const surface = evidence.dataset.proofSurface;
    const appearance = evidence.dataset.proofAppearance;
    return (state.product === "all" || product === "all" || product === state.product) &&
      (state.mode === "all" || surface === state.mode) &&
      (state.scheme === "all" || appearance === state.scheme);
  };

  const apply = () => {
    root.dataset.product = state.product;
    root.dataset.mode = state.mode;
    root.dataset.scheme = state.scheme;

    for (const [key, buttons] of Object.entries(controls)) {
      for (const button of buttons) {
        button.setAttribute("aria-pressed", String(button.dataset[key] === state[key]));
      }
    }

    const evidence = Array.from(document.querySelectorAll("[data-evidence]"));
    const visible = evidence.filter(matches);
    for (const specimen of evidence) {
      specimen.hidden = !matches(specimen);
    }
    summary.textContent = visible.length === 0
      ? "No catalog evidence matches this detail filter. Reset to inspect all products."
      : `${visible.length} of ${evidence.length} catalog evidence records shown. Family comparison remains visible.`;

    const next = new URL(window.location.href);
    for (const [key, value] of Object.entries(state)) {
      next.searchParams.set(key, value);
    }
    history.replaceState(null, "", next);
  };

  for (const [key, buttons] of Object.entries(controls)) {
    for (const button of buttons) {
      button.addEventListener("click", () => {
        state[key] = button.dataset[key];
        apply();
      });
    }
  }

  document.querySelector("[data-reset]").addEventListener("click", () => {
    for (const key of Object.keys(state)) {
      state[key] = defaults[key];
    }
    apply();
  });

  apply();
})();
