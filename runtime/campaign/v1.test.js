"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const runtimeSource = fs.readFileSync(path.join(__dirname, "v1.js"), "utf8");
const SVG_NS = "http://www.w3.org/2000/svg";

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((accept, decline) => {
    resolve = accept;
    reject = decline;
  });
  return { promise, resolve, reject };
}

class FakeEventTarget {
  constructor() {
    this.listeners = new Map();
  }

  addEventListener(type, listener) {
    const listeners = this.listeners.get(type) || [];
    listeners.push(listener);
    this.listeners.set(type, listeners);
  }

  dispatchEvent(event) {
    event.target = this;
    for (const listener of this.listeners.get(event.type) || []) {
      listener.call(this, event);
    }
    return true;
  }
}

class FakeElement extends FakeEventTarget {
  constructor(tagName, fixture) {
    super();
    this.tagName = tagName.toUpperCase();
    this.localName = tagName.toLowerCase();
    this.fixture = fixture;
    this.attributes = new Map();
    this.children = [];
    this.hidden = false;
    this.parentNode = null;
  }

  set src(value) {
    this.attributes.set("src", String(value));
    if (this.fixture.resourceLoads) {
      this.fixture.resourceLoads.push({ node: this, url: String(value), crossOrigin: this.getAttribute("crossorigin") });
    }
  }

  get src() {
    return this.getAttribute("src");
  }

  set href(value) {
    this.attributes.set("href", String(value));
    if (this.fixture.resourceLoads) {
      this.fixture.resourceLoads.push({ node: this, url: String(value), crossOrigin: this.getAttribute("crossorigin") });
    }
  }

  get href() {
    return this.getAttribute("href");
  }

  set crossOrigin(value) {
    if (value === null || value === undefined) {
      this.attributes.delete("crossorigin");
    } else {
      this.attributes.set("crossorigin", String(value));
    }
  }

  get crossOrigin() {
    return this.getAttribute("crossorigin");
  }

  set innerHTML(_) {
    throw new Error("runtime must not use innerHTML");
  }

  setAttribute(name, value) {
    const stringValue = String(value);
    this.attributes.set(name, stringValue);
    if (name === "src") {
      this.src = stringValue;
    }
    if (name === "href") {
      this.href = stringValue;
    }
    if (name === "data-theme") {
      this.fixture.rootState.theme = stringValue;
    }
    if (name === "data-theme-source") {
      this.fixture.rootState.themeSource = stringValue;
      this.fixture.notifyThemeSource();
    }
    if (name === "data-campaign") {
      this.fixture.rootState.campaign = stringValue;
    }
    if (this === this.fixture.root && name.startsWith("data-")) {
      this.fixture.recordRootMutation();
    }
  }

  getAttribute(name) {
    return this.attributes.has(name) ? this.attributes.get(name) : null;
  }

  hasAttribute(name) {
    return this.attributes.has(name);
  }

  removeAttribute(name) {
    this.attributes.delete(name);
    if (name === "data-campaign") {
      delete this.fixture.rootState.campaign;
    }
    if (this === this.fixture.root && name.startsWith("data-")) {
      this.fixture.recordRootMutation();
    }
  }

  appendChild(child) {
    child.parentNode = this;
    this.children.push(child);
    if (this === this.fixture.head && child.localName === "link") {
      this.fixture.styles.push(child);
      queueMicrotask(() => this.fixture.settleStyle(child));
    }
    return child;
  }

  remove() {
    if (!this.parentNode) {
      return;
    }
    this.parentNode.children = this.parentNode.children.filter((child) => child !== this);
    this.parentNode = null;
  }

  replaceChildren(...children) {
    this.children = children;
    for (const child of children) {
      child.parentNode = this;
    }
  }

  cloneNode(deep) {
    const clone = new FakeElement(this.localName, this.fixture);
    clone.namespaceURI = this.namespaceURI;
    clone.textContent = this.textContent;
    for (const [name, value] of this.attributes) {
      clone.attributes.set(name, value);
    }
    if (deep) {
      clone.replaceChildren(...this.children.map((child) => child.cloneNode(true)));
    }
    return clone;
  }
}

class FakeDocument extends FakeEventTarget {
  constructor(fixture) {
    super();
    this.fixture = fixture;
    this.baseURI = fixture.pageURL;
    this.documentElement = fixture.root;
    this.head = fixture.head;
    this.currentScript = { dataset: { channel: fixture.channelURL } };
  }

  querySelectorAll(selector) {
    this.fixture.queries.push(selector);
    return this.fixture.hooks.get(selector) || [];
  }

  createElement(name) {
    return new FakeElement(name, this.fixture);
  }

  createElementNS(namespace, name) {
    const element = new FakeElement(name, this.fixture);
    element.namespaceURI = namespace;
    return element;
  }

  importNode(node, deep) {
    return node.cloneNode(deep);
  }
}

const activeCampaign = {
  schemaVersion: 1,
  runtimeVersion: 1,
  release: "v0.2.0",
  source: "campaign",
  theme: {
    id: "araihu-halloween",
    cssUrl: "https://araihu.example/assets/releases/v0.2.0/themes/araihu-halloween.css",
  },
  campaign: {
    id: "halloween-2026",
    toggle: {
      enabledIcon: {
        id: "ui-hi-16-solid-sparkles",
        mode: "sprite",
        url: "https://araihu.example/assets/releases/v0.2.0/icons/ui/sprite.svg",
        spriteSymbol: "hi-16-solid-sparkles",
      },
      disabledIcon: {
        id: "ui-hi-16-solid-moon",
        mode: "asset",
        url: "https://araihu.example/assets/releases/v0.2.0/icons/ui/moon.svg",
      },
    },
    brand: {
      logo: {
        id: "araihu-logo-halloween",
        url: "https://araihu.example/assets/releases/v0.2.0/brand/araihu/logo/halloween.svg",
      },
      icon: {
        id: "araihu-icon-halloween",
        url: "https://araihu.example/assets/releases/v0.2.0/icons/brand/halloween.svg",
      },
    },
  },
  digest: "a".repeat(64),
};

const defaultChannel = {
  schemaVersion: 1,
  runtimeVersion: 1,
  release: "v0.2.0",
  source: "default",
  theme: {
    id: "araihu",
    cssUrl: "https://araihu.example/assets/releases/v0.2.0/themes/araihu.css",
  },
  digest: "b".repeat(64),
};

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function channelAt(origin) {
  return JSON.parse(JSON.stringify(activeCampaign).replaceAll("https://araihu.example", origin));
}

function runtimeFixture(options = {}) {
  const fixture = {
    pageURL: options.pageURL || "https://app.araihu.example/",
    channelURL: options.channelURL || "https://araihu.example/assets/releases/current",
    channel: clone(options.channel || activeCampaign),
    baselineLogo: "https://app.araihu.example/assets/logo.svg",
    baselineIcon: "https://app.araihu.example/assets/favicon.svg",
    events: [],
    fetches: [],
    images: [],
    mutations: [],
    observers: [],
    queries: [],
    resourceLoads: [],
    rootState: {
      theme: options.theme || "minimal",
      themeSource: options.themeSource || "default",
    },
    storage: new Map(Object.entries(options.storage || {})),
    styles: [],
    styleGate: options.pendingStyle ? deferred() : null,
    imageGate: options.pendingImages ? deferred() : null,
    toggleImageGate: options.pendingToggleImage ? deferred() : null,
    channelGate: null,
    hooks: new Map(),
    failure: options.failure || "",
    reducedMotion: Boolean(options.reducedMotion),
  };

  fixture.recordRootMutation = () => {};
  fixture.notifyThemeSource = () => {};
  fixture.root = new FakeElement("html", fixture);
  fixture.head = new FakeElement("head", fixture);
  fixture.brandLogo = new FakeElement("img", fixture);
  fixture.brandLogo.src = fixture.baselineLogo;
  fixture.brandIcon = new FakeElement("link", fixture);
  fixture.brandIcon.href = fixture.baselineIcon;
  if (options.baselineCrossOrigin !== undefined) {
    fixture.brandLogo.setAttribute("crossorigin", options.baselineCrossOrigin);
    fixture.brandIcon.setAttribute("crossorigin", options.baselineCrossOrigin);
  }
  fixture.toggle = new FakeElement("button", fixture);
  fixture.toggle.hidden = true;
  fixture.toggleIcon = new FakeElement("span", fixture);
  fixture.baselineToggleNode = new FakeElement("span", fixture);
  fixture.baselineToggleNode.setAttribute("data-baseline-icon", "true");
  fixture.toggleIcon.replaceChildren(fixture.baselineToggleNode);
  fixture.hooks.set('[data-asset-brand="logo"]', [fixture.brandLogo]);
  fixture.hooks.set('[data-asset-brand="icon"]', [fixture.brandIcon]);
  fixture.hooks.set("[data-campaign-toggle]", [fixture.toggle]);
  fixture.hooks.set("[data-campaign-toggle-icon]", [fixture.toggleIcon]);

  fixture.root.setAttribute("data-theme", fixture.rootState.theme);
  fixture.root.setAttribute("data-theme-source", fixture.rootState.themeSource);
  fixture.mutations.length = 0;

  fixture.recordRootMutation = () => {
    fixture.mutations.push({
      theme: fixture.rootState.theme,
      campaign: fixture.rootState.campaign,
    });
  };
  fixture.notifyThemeSource = () => {
    const notify = () => {
      for (const observer of fixture.observers) {
        observer([{ attributeName: "data-theme-source", target: fixture.root }]);
      }
    };
    if (fixture.delayedObserver) {
      setImmediate(notify);
    } else {
      queueMicrotask(notify);
    }
  };
  fixture.settleStyle = (link) => {
    if (fixture.failure === "css") {
      link.onerror(new Error("server stylesheet detail"));
      return;
    }
    const finish = () => link.onload();
    if (fixture.styleGate) {
      fixture.styleGate.promise.then(finish);
    } else {
      finish();
    }
  };

  class FakeImage {
    constructor() {
      this.crossOrigin = "";
      fixture.images.push(this);
    }

    set src(value) {
      this._src = value;
    }

    get src() {
      return this._src;
    }

    decode() {
      if (fixture.failure === "image") {
        return Promise.reject(new Error("server image detail"));
      }
      if (fixture.toggleImageGate && this._src.endsWith("/moon.svg")) {
        return fixture.toggleImageGate.promise;
      }
      return fixture.imageGate ? fixture.imageGate.promise : Promise.resolve();
    }
  }

  class FakeMutationObserver {
    constructor(callback) {
      this.callback = callback;
    }

    observe(target, init) {
      fixture.observers.push(this.callback);
    }
  }

  class FakeDOMParser {
    parseFromString(text, mediaType) {
      if (fixture.failure === "sprite-parse") {
        return {
          querySelector(selector) {
            return selector === "parsererror" ? {} : null;
          },
          getElementById() {
            return null;
          },
        };
      }
      const symbolID = /<symbol id="([^"]+)"/.exec(text)?.[1];
      const symbol = new FakeElement("symbol", fixture);
      symbol.namespaceURI = fixture.failure === "sprite-namespace" ? "urn:not-svg" : SVG_NS;
      symbol.setAttribute("id", symbolID || "");
      symbol.setAttribute("viewBox", "0 0 16 16");
      const pathNode = new FakeElement("path", fixture);
      pathNode.namespaceURI = symbol.namespaceURI;
      pathNode.setAttribute("d", "M1 1h14v14z");
      symbol.appendChild(pathNode);
      return {
        querySelector() {
          return null;
        },
        getElementById(id) {
          return id === symbolID ? symbol : null;
        },
      };
    }
  }

  const document = new FakeDocument(fixture);
  const window = {
    document,
    location: new URL(fixture.pageURL),
    URL,
    Image: FakeImage,
    DOMParser: FakeDOMParser,
    MutationObserver: FakeMutationObserver,
    CustomEvent: class {
      constructor(type, init) {
        this.type = type;
        this.detail = init.detail;
      }
    },
    matchMedia(query) {
      return { matches: fixture.reducedMotion };
    },
    localStorage: {
      getItem(key) {
        return fixture.storage.has(key) ? fixture.storage.get(key) : null;
      },
      setItem(key, value) {
        if (options.storageSetError) {
          throw options.storageSetError;
        }
        fixture.storage.set(key, String(value));
      },
      removeItem(key) {
        if (options.storageRemoveError) {
          throw options.storageRemoveError;
        }
        fixture.storage.delete(key);
      },
    },
    fetch: async (url, init) => {
      fixture.fetches.push({ url: String(url), init });
      if (fixture.failure === "channel") {
        throw new Error("server fetch detail");
      }
      if (String(url).endsWith("sprite.svg")) {
        if (fixture.failure === "sprite-fetch") {
          return { ok: false, status: 500 };
        }
        return {
          ok: true,
          text: async () => '<svg xmlns="http://www.w3.org/2000/svg"><symbol id="hi-16-solid-sparkles" viewBox="0 0 16 16"><path d="M1 1h14v14z"/></symbol><symbol id="not-selected"><script/></symbol></svg>',
        };
      }
      const channelDocument = clone(fixture.channel);
      if (fixture.channelGate) {
        await fixture.channelGate.promise;
      }
      return {
        ok: true,
        json: async () => clone(channelDocument),
      };
    },
  };
  document.addEventListener = FakeEventTarget.prototype.addEventListener;
  document.dispatchEvent = function (event) {
    fixture.events.push({ type: event.type, detail: event.detail });
    return FakeEventTarget.prototype.dispatchEvent.call(this, event);
  };
  window.window = window;

  fixture.start = async () => {
    vm.runInNewContext(runtimeSource, {
      window,
      document,
      URL,
      Promise,
      Object,
      Array,
      String,
      Error,
      TypeError,
      RegExp,
      queueMicrotask,
    });
    fixture.runtime = window.AraiHuCampaign;
    return fixture.runtime.refresh();
  };
  fixture.refresh = () => fixture.runtime.refresh();
  fixture.resolveTheme = () => fixture.styleGate.resolve();
  fixture.resolveImages = () => fixture.imageGate.resolve();
  fixture.deferToggleImage = () => {
    fixture.toggleImageGate = deferred();
  };
  fixture.resolveToggleImage = () => fixture.toggleImageGate.resolve();
  fixture.deferChannel = () => {
    fixture.channelGate = deferred();
  };
  fixture.resolveChannel = () => fixture.channelGate.resolve();
  fixture.clickToggle = () => fixture.toggle.dispatchEvent({ type: "click" });
  fixture.setThemeSource = (source, theme) => {
    if (theme) {
      fixture.root.setAttribute("data-theme", theme);
    }
    fixture.root.setAttribute("data-theme-source", source);
  };
  return fixture;
}

test("explicit preference prevents campaign mutation", async () => {
  const fixture = runtimeFixture({ themeSource: "preference" });
  await fixture.start();

  assert.equal(fixture.rootState.theme, "minimal");
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  assert.equal(fixture.rootState.campaign, undefined);
  assert.deepEqual(fixture.events, []);
});

test("theme and images finish before atomic mutation", async () => {
  const fixture = runtimeFixture({ pendingStyle: true, pendingImages: true });
  const pending = fixture.start();
  await new Promise((resolve) => setImmediate(resolve));

  assert.deepEqual(fixture.mutations, []);
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  fixture.resolveTheme();
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(fixture.mutations, []);
  fixture.resolveImages();
  await pending;

  assert.deepEqual(fixture.mutations.at(-1), {
    theme: "araihu-halloween",
    campaign: "halloween-2026",
  });
  assert.equal(fixture.brandLogo.src, activeCampaign.campaign.brand.logo.url);
});

test("preference selected during preload cancels the pending campaign commit", async () => {
  const fixture = runtimeFixture({ pendingStyle: true, pendingImages: true });
  const pending = fixture.start();
  await new Promise((resolve) => setImmediate(resolve));
  fixture.setThemeSource("preference", "user-night");
  fixture.resolveTheme();
  fixture.resolveImages();
  await pending;

  assert.equal(fixture.rootState.theme, "user-night");
  assert.equal(fixture.rootState.themeSource, "preference");
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  assert.equal(fixture.head.children.length, 0);
});

test("invalid channel versions and URLs preserve baseline with stable errors", async (t) => {
  const cases = [
    ["schema-version", { schemaVersion: 2 }, "schema-version"],
    ["runtime-version", { runtimeVersion: 2 }, "runtime-version"],
    ["channel-http", {}, "channel-url", { channelURL: "http://araihu.example/assets/releases/current" }],
    ["asset-other-origin", { theme: { ...activeCampaign.theme, cssUrl: "https://evil.example/assets/releases/v0.2.0/theme.css" } }, "asset-url"],
    ["asset-wrong-path", { theme: { ...activeCampaign.theme, cssUrl: "https://araihu.example/themes/theme.css" } }, "asset-url"],
  ];
  for (const [name, patch, code, options = {}] of cases) {
    await t.test(name, async () => {
      const channel = Object.assign(clone(activeCampaign), patch);
      const fixture = runtimeFixture({ ...options, channel });
      await fixture.start();
      assert.equal(fixture.rootState.theme, "minimal");
      assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
      const error = fixture.events.at(-1);
      assert.equal(error.type, "araihu:campaign:error");
      assert.deepEqual(Object.keys(error.detail).sort(), ["code"]);
      assert.equal(error.detail.code, code);
    });
  }
});

test("HTTP is accepted only for a local channel and same local asset origin", async () => {
  const fixture = runtimeFixture({
    pageURL: "http://localhost:8080/",
    channelURL: "http://localhost:8080/assets/releases/current",
    channel: channelAt("http://localhost:8080"),
  });
  await fixture.start();

  assert.equal(fixture.rootState.theme, "araihu-halloween");
  assert.equal(fixture.rootState.themeSource, "campaign", JSON.stringify(fixture.events));
  assert.equal(fixture.events.at(-1).type, "araihu:campaign:applied");
});

test("channel, CSS, image, and sprite failures preserve baseline", async (t) => {
  for (const [failure, code] of [
    ["channel", "channel-fetch"],
    ["css", "theme-load"],
    ["image", "image-load"],
    ["sprite-fetch", "sprite-fetch"],
    ["sprite-parse", "sprite-parse"],
    ["sprite-namespace", "sprite-parse"],
  ]) {
    await t.test(failure, async () => {
      const fixture = runtimeFixture({ failure });
      await fixture.start();
      assert.equal(fixture.rootState.theme, "minimal");
      assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
      assert.equal(fixture.brandIcon.href, fixture.baselineIcon);
      assert.equal(fixture.rootState.campaign, undefined);
      assert.equal(fixture.events.at(-1).detail.code, code);
      assert.equal(fixture.head.children.length, 0);
    });
  }
});

test("image failure cancels a stylesheet whose load remains pending", async () => {
  const fixture = runtimeFixture({ failure: "image", pendingStyle: true });
  await fixture.start();

  assert.equal(fixture.events.at(-1).detail.code, "image-load");
  assert.equal(fixture.head.children.length, 0);
  assert.equal(fixture.rootState.theme, "minimal");
});

test("campaign applies direct assets, selected sprite, anonymous requests, and bounded events", async () => {
  const fixture = runtimeFixture();
  await fixture.start();

  assert.equal(fixture.runtime.version, 1);
  assert.equal(fixture.rootState.themeSource, "campaign", JSON.stringify(fixture.events));
  assert.equal(fixture.brandLogo.crossOrigin, "anonymous");
  assert.equal(fixture.brandIcon.crossOrigin, "anonymous");
  assert.equal(fixture.brandIcon.href, activeCampaign.campaign.brand.icon.url);
  assert.equal(fixture.toggle.hidden, false);
  assert.equal(fixture.toggle.getAttribute("aria-pressed"), "true");
  assert.equal(fixture.toggleIcon.children.length, 1);
  const svg = fixture.toggleIcon.children[0];
  assert.equal(svg.namespaceURI, SVG_NS);
  assert.equal(svg.localName, "svg");
  assert.equal(svg.children.length, 1);
  assert.equal(svg.children[0].localName, "path");
  assert.equal(fixture.fetches.every((request) => request.init.credentials === "omit"), true);
  assert.equal(fixture.styles[0].crossOrigin, "anonymous");
  assert.equal(fixture.images.every((image) => image.crossOrigin === "anonymous"), true);
  assert.equal(fixture.queries.every((selector) => [
    '[data-asset-brand="logo"]',
    '[data-asset-brand="icon"]',
    "[data-campaign-toggle]",
    "[data-campaign-toggle-icon]",
  ].includes(selector)), true);
  assert.deepEqual(fixture.events.map((event) => event.type), [
    "araihu:campaign:before-apply",
    "araihu:campaign:applied",
  ]);
  assert.deepEqual(Object.keys(fixture.events[0].detail).sort(), ["campaign", "code", "reducedMotion"]);
});

test("active campaign toggle restores baseline and persists campaign-specific opt-out", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  fixture.clickToggle();
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(fixture.rootState.theme, "minimal");
  assert.equal(fixture.rootState.themeSource, "campaign-opt-out");
  assert.equal(fixture.rootState.campaign, "halloween-2026");
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  assert.equal(fixture.storage.get("araihu.assets.campaign.v1.optout.halloween-2026"), "1");
  assert.equal(fixture.toggle.getAttribute("aria-pressed"), "false");
  assert.equal(fixture.toggleIcon.children[0].localName, "img");
  assert.equal(fixture.toggleIcon.children[0].crossOrigin, "anonymous");
  assert.equal(fixture.toggleIcon.children[0].src, activeCampaign.campaign.toggle.disabledIcon.url);
  assert.equal(fixture.events.at(-1).type, "araihu:campaign:restored");
});

test("brand CORS attribute presence and value restore exactly before baseline URLs", async (t) => {
  for (const [name, baselineCrossOrigin, expected] of [
    ["absent", undefined, null],
    ["empty", "", ""],
    ["anonymous", "anonymous", "anonymous"],
  ]) {
    await t.test(name, async () => {
      const fixture = runtimeFixture({ baselineCrossOrigin });
      await fixture.start();
      fixture.clickToggle();
      await new Promise((resolve) => setImmediate(resolve));

      assert.equal(fixture.brandLogo.getAttribute("crossorigin"), expected);
      assert.equal(fixture.brandIcon.getAttribute("crossorigin"), expected);
      const logoRestore = fixture.resourceLoads.filter((load) =>
        load.node === fixture.brandLogo && load.url === fixture.baselineLogo).at(-1);
      const iconRestore = fixture.resourceLoads.filter((load) =>
        load.node === fixture.brandIcon && load.url === fixture.baselineIcon).at(-1);
      assert.equal(logoRestore.crossOrigin, expected);
      assert.equal(iconRestore.crossOrigin, expected);
    });
  }
});

test("storage write and remove exceptions expose only fixed toggle error codes", async (t) => {
  await t.test("setItem", async () => {
    const fixture = runtimeFixture({ storageSetError: { code: 22, server: "private" } });
    await fixture.start();
    fixture.clickToggle();
    await new Promise((resolve) => setImmediate(resolve));

    assert.equal(fixture.rootState.themeSource, "campaign");
    assert.equal(fixture.events.at(-1).type, "araihu:campaign:error");
    assert.equal(fixture.events.at(-1).detail.code, "toggle-failed");
    assert.deepEqual(Object.keys(fixture.events.at(-1).detail), ["code"]);
  });

  await t.test("removeItem", async () => {
    const fixture = runtimeFixture({
      storage: { "araihu.assets.campaign.v1.optout.halloween-2026": "1" },
      storageRemoveError: { code: "asset-url", server: "private" },
    });
    await fixture.start();
    fixture.clickToggle();
    await new Promise((resolve) => setImmediate(resolve));

    assert.equal(fixture.rootState.themeSource, "campaign-opt-out");
    assert.equal(fixture.events.at(-1).type, "araihu:campaign:error");
    assert.equal(fixture.events.at(-1).detail.code, "toggle-failed");
    assert.deepEqual(Object.keys(fixture.events.at(-1).detail), ["code"]);
  });
});

test("saved campaign opt-out wins on reload without applying campaign", async () => {
  const fixture = runtimeFixture({
    storage: { "araihu.assets.campaign.v1.optout.halloween-2026": "1" },
  });
  await fixture.start();

  assert.equal(fixture.rootState.theme, "minimal");
  assert.equal(fixture.rootState.themeSource, "campaign-opt-out");
  assert.equal(fixture.rootState.campaign, "halloween-2026");
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  assert.equal(fixture.toggle.getAttribute("aria-pressed"), "false");
  assert.equal(fixture.events.length, 0);
  assert.equal(await fixture.refresh(), false);
});

test("preference selected during saved opt-out preload retains ownership", async () => {
  const fixture = runtimeFixture({
    storage: { "araihu.assets.campaign.v1.optout.halloween-2026": "1" },
    pendingToggleImage: true,
  });
  const pending = fixture.start();
  await new Promise((resolve) => setImmediate(resolve));
  fixture.setThemeSource("preference", "user-night");
  fixture.resolveToggleImage();
  await pending;

  assert.equal(fixture.rootState.theme, "user-night");
  assert.equal(fixture.rootState.themeSource, "preference");
  assert.equal(fixture.rootState.campaign, undefined);
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
});

test("preference observer restores owned brand without overwriting selected theme", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  fixture.setThemeSource("preference", "user-night");
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(fixture.rootState.theme, "user-night");
  assert.equal(fixture.rootState.themeSource, "preference");
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  assert.equal(fixture.rootState.campaign, undefined);

  fixture.setThemeSource("default", "minimal");
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(fixture.rootState.theme, "araihu-halloween");
  assert.equal(fixture.rootState.themeSource, "campaign");
});

test("expired channel restores active campaign baseline", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  fixture.channel = clone(defaultChannel);
  await fixture.refresh();

  assert.equal(fixture.rootState.theme, "minimal");
  assert.equal(fixture.rootState.themeSource, "default");
  assert.equal(fixture.rootState.campaign, undefined);
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
  assert.equal(fixture.toggle.hidden, true);
  assert.equal(fixture.events.at(-1).type, "araihu:campaign:restored");
  assert.equal(fixture.events.at(-1).detail.code, "campaign-inactive");
});

test("runtime source restoration does not trigger a second channel refresh", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  const before = fixture.fetches.filter((request) => request.url.endsWith("/current")).length;
  fixture.delayedObserver = true;
  fixture.channel = clone(defaultChannel);
  await fixture.refresh();
  await new Promise((resolve) => setImmediate(resolve));
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(fixture.fetches.filter((request) => request.url.endsWith("/current")).length, before + 1);
  assert.equal(fixture.rootState.themeSource, "default");
});

test("refresh during active refresh performs a newer fetch and applies its state", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  const before = fixture.fetches.filter((request) => request.url.endsWith("/current")).length;
  fixture.deferChannel();
  fixture.channel = clone(activeCampaign);
  const firstRefresh = fixture.refresh();
  await new Promise((resolve) => setImmediate(resolve));
  fixture.channel = clone(defaultChannel);
  const newerRefresh = fixture.refresh();
  fixture.resolveChannel();
  await Promise.all([firstRefresh, newerRefresh]);

  assert.equal(fixture.fetches.filter((request) => request.url.endsWith("/current")).length, before + 2);
  assert.equal(fixture.rootState.themeSource, "default");
  assert.equal(fixture.rootState.campaign, undefined);
});

test("two sequential toggle intents restore then reapply campaign", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  fixture.clickToggle();
  fixture.clickToggle();
  await new Promise((resolve) => setImmediate(resolve));
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(fixture.rootState.themeSource, "campaign");
  assert.equal(fixture.rootState.campaign, "halloween-2026");
  assert.equal(fixture.storage.has("araihu.assets.campaign.v1.optout.halloween-2026"), false);
  assert.equal(fixture.events.filter((event) => event.type === "araihu:campaign:restored").length, 1);
  assert.equal(fixture.events.filter((event) => event.type === "araihu:campaign:applied").length, 2);
});

test("refresh requested during toggle runs afterward and applies latest channel", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  const before = fixture.fetches.filter((request) => request.url.endsWith("/current")).length;
  fixture.deferToggleImage();
  fixture.clickToggle();
  fixture.channel = clone(activeCampaign);
  const firstRefresh = fixture.refresh();
  fixture.channel = clone(defaultChannel);
  const latestRefresh = fixture.refresh();
  fixture.resolveToggleImage();
  await Promise.all([firstRefresh, latestRefresh]);

  assert.equal(fixture.rootState.themeSource, "default");
  assert.equal(fixture.rootState.campaign, undefined);
  assert.equal(fixture.storage.get("araihu.assets.campaign.v1.optout.halloween-2026"), "1");
  assert.equal(fixture.fetches.filter((request) => request.url.endsWith("/current")).length, before + 2);
  assert.equal(fixture.events.some((event) => event.detail.code === "toggle-failed"), false);
});

test("toggle requested during refresh runs after refreshed state", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  const before = fixture.fetches.filter((request) => request.url.endsWith("/current")).length;
  fixture.deferChannel();
  const refresh = fixture.refresh();
  await new Promise((resolve) => setImmediate(resolve));
  fixture.clickToggle();
  fixture.resolveChannel();
  await refresh;
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(fixture.rootState.themeSource, "campaign-opt-out");
  assert.equal(fixture.storage.get("araihu.assets.campaign.v1.optout.halloween-2026"), "1");
  assert.equal(fixture.fetches.filter((request) => request.url.endsWith("/current")).length, before + 1);
});

test("alternating queued intents preserve their serialized order", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  const before = fixture.fetches.filter((request) => request.url.endsWith("/current")).length;
  fixture.deferChannel();
  const activeRefresh = fixture.refresh();
  await new Promise((resolve) => setImmediate(resolve));
  fixture.clickToggle();
  const queuedRefresh = fixture.refresh();
  fixture.clickToggle();
  fixture.resolveChannel();
  await Promise.all([activeRefresh, queuedRefresh]);
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(fixture.rootState.themeSource, "campaign");
  assert.equal(fixture.storage.has("araihu.assets.campaign.v1.optout.halloween-2026"), false);
  assert.equal(fixture.fetches.filter((request) => request.url.endsWith("/current")).length, before + 2);
});

test("preference selected during toggle preload cancels opt-out mutation", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  fixture.deferToggleImage();
  fixture.clickToggle();
  fixture.setThemeSource("preference", "user-night");
  fixture.resolveToggleImage();
  await fixture.refresh();

  assert.equal(fixture.rootState.theme, "user-night");
  assert.equal(fixture.rootState.themeSource, "preference");
  assert.equal(fixture.storage.has("araihu.assets.campaign.v1.optout.halloween-2026"), false);
  assert.equal(fixture.events.some((event) => event.detail.code === "toggle-failed"), false);
});

test("refresh atomically replaces a changed active campaign document", async () => {
  const fixture = runtimeFixture();
  await fixture.start();
  fixture.channel = clone(activeCampaign);
  fixture.channel.digest = "c".repeat(64);
  fixture.channel.theme.id = "araihu-halloween-night";
  fixture.channel.theme.cssUrl = "https://araihu.example/assets/releases/v0.2.0/themes/araihu-halloween-night.css";
  fixture.channel.campaign.brand.logo.url = "https://araihu.example/assets/releases/v0.2.0/brand/araihu/logo/halloween-night.svg";
  await fixture.refresh();

  assert.equal(fixture.rootState.theme, "araihu-halloween-night");
  assert.equal(fixture.brandLogo.src, fixture.channel.campaign.brand.logo.url);
  assert.equal(fixture.rootState.themeSource, "campaign");
});

test("lifecycle hooks expose reduced-motion preference without runtime animation", async () => {
  const fixture = runtimeFixture({ reducedMotion: true });
  await fixture.start();

  assert.equal(fixture.events[0].detail.reducedMotion, true);
  assert.equal(fixture.events[1].detail.reducedMotion, true);
  assert.equal(fixture.root.getAttribute("style"), null);
});
