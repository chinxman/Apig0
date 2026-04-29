(function() {
  const state = {
    gatewayOrigin: window.location.origin,
    currentUser: null,
    currentRole: null,
    pendingChallenge: null,
    setupStatus: null,
    paused: false,
    autoScroll: true,
    uptimeBase: 0,
    uptimeOffset: 0,
    totalReq: 0,
    totalErr: 0,
    svcData: {},
    evtSource: null,
    reconnectTimer: null,
    serviceCatalog: [],
    openAIServiceCatalog: [],
    availableServices: [],
    portalSelectedService: "",
    portalMethod: "GET",
    portalPath: "/",
    portalBody: "",
    portalToken: "",
    portalAssignedTokenPrefix: "",
    portalHasAssignedToken: false,
    portalAssignedTokenType: "standard",
    portalAssignedOpenAIService: "",
    portalAssignedBackendLabel: "",
    portalAssignedAllowedModels: [],
    portalAssignedAllowedProviders: [],
    portalPendingDeliveries: [],
    portalHeadersText: "",
    portalHistory: [],
    adminServices: [],
    serviceSecretStorage: null,
    serviceSecretMeta: {},
    adminTokens: [],
    auditTrail: [],
    editingServiceName: null,
    activeSvcPanel: null,
    setupMode: "temporary",
    setupVaultType: "file",
    setupServiceSecretMode: "file",
    upgradeVaultType: "file",
    upgradeServiceSecretMode: "file",
    maxLogRows: 300
  };

  const dom = {
    id(id) {
      return document.getElementById(id);
    },
    qs(selector, root = document) {
      return root.querySelector(selector);
    },
    qsa(selector, root = document) {
      return Array.from(root.querySelectorAll(selector));
    },
    show(el, display = "") {
      if (typeof el === "string") el = this.id(el);
      if (el) el.style.display = display;
    },
    hide(el) {
      if (typeof el === "string") el = this.id(el);
      if (el) el.style.display = "none";
    },
    text(el, value) {
      if (typeof el === "string") el = this.id(el);
      if (el) el.textContent = value;
    },
    html(el, value) {
      if (typeof el === "string") el = this.id(el);
      if (el) el.innerHTML = value;
    }
  };

  function csrfToken() {
    const match = document.cookie.match("(?:^|; )apig0_csrf=([^;]*)");
    return match ? decodeURIComponent(match[1]) : "";
  }

  async function parseResponse(resp) {
    const contentType = resp.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
      try {
        return await resp.json();
      } catch {
        return null;
      }
    }
    try {
      return await resp.text();
    } catch {
      return null;
    }
  }

  const api = {
    async request(path, options = {}) {
      const method = options.method || "GET";
      const headers = Object.assign({}, options.headers || {});
      if (options.csrf) {
        headers["X-CSRF-Token"] = csrfToken();
      }
      const fetchOptions = {method, headers};
      if (options.body !== undefined) {
        headers["Content-Type"] = "application/json";
        fetchOptions.body = JSON.stringify(options.body);
      }
      const resp = await fetch(state.gatewayOrigin + path, fetchOptions);
      const data = await parseResponse(resp);
      return {ok: resp.ok, status: resp.status, data, response: resp};
    },
    get(path, options = {}) {
      return this.request(path, Object.assign({}, options, {method: "GET"}));
    },
    post(path, body, options = {}) {
      return this.request(path, Object.assign({}, options, {method: "POST", body}));
    },
    put(path, body, options = {}) {
      return this.request(path, Object.assign({}, options, {method: "PUT", body}));
    },
    delete(path, options = {}) {
      return this.request(path, Object.assign({}, options, {method: "DELETE"}));
    }
  };

  const util = {
    escHtml(value) {
      const div = document.createElement("div");
      div.textContent = value == null ? "" : String(value);
      return div.innerHTML;
    },
    escAttr(value) {
      return String(value == null ? "" : value)
        .replace(/&/g, "&amp;")
        .replace(/'/g, "&#39;")
        .replace(/"/g, "&quot;");
    },
    fmtNum(value) {
      return new Intl.NumberFormat().format(value || 0);
    },
    statusClass(status) {
      if (status >= 500) return "s5xx";
      if (status >= 400) return "s4xx";
      if (status >= 300) return "s3xx";
      return "s2xx";
    },
    latencyClass(ms) {
      if (ms < 120) return "latency-fast";
      if (ms < 500) return "latency-mid";
      return "latency-slow";
    },
    fmtUptime(sec) {
      if (sec < 60) return sec + "s";
      if (sec < 3600) return Math.floor(sec / 60) + "m " + (sec % 60) + "s";
      const hours = Math.floor(sec / 3600);
      const minutes = Math.floor((sec % 3600) / 60);
      return hours + "h " + minutes + "m";
    },
    getCheckedValues(nodes) {
      return Array.from(nodes).filter((node) => node.checked).map((node) => node.value);
    },
    isoInputValue(raw) {
      if (!raw) return "";
      const date = new Date(raw);
      if (Number.isNaN(date.getTime())) return "";
      return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
    },
    displayDateTime(raw) {
      if (!raw) return '<span class="service-muted">-</span>';
      const date = new Date(raw);
      if (Number.isNaN(date.getTime())) return '<span class="service-muted">-</span>';
      return this.escHtml(date.toLocaleString());
    },
    setNotice(id, msg, type, timeout = 4000) {
      const el = dom.id(id);
      if (!el) return;
      el.textContent = msg;
      el.className = "notice visible " + type;
      if (msg) {
        setTimeout(() => el.classList.remove("visible"), timeout);
      }
    }
  };

  window.App = {
    state,
    dom,
    api,
    util,
    actions: {},
    auth: {},
    portal: {},
    setup: {},
    monitor: {},
    services: {},
    users: {},
    tokens: {},
    rateLimits: {},
    navigation: {}
  };
})();
