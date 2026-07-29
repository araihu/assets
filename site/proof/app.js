(() => {
  "use strict";

  const root = document.documentElement;
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

  const apply = () => {
    root.dataset.product = state.product;
    root.dataset.mode = state.mode;
    root.dataset.scheme = state.scheme;

    for (const [key, buttons] of Object.entries(controls)) {
      for (const button of buttons) {
        button.setAttribute("aria-pressed", String(button.dataset[key] === state[key]));
      }
    }

    for (const specimen of document.querySelectorAll("[data-proof-product]")) {
      const product = specimen.dataset.proofProduct;
      specimen.hidden = product !== "all" && product !== state.product;
    }

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

  apply();
})();
