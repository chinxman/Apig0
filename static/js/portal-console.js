(function(App) {
  const {state, dom, util} = App;
  const historyKey = "apig0.portal.terminal.history";
  const terminalLines = [];
  const commandHistory = [];
  let commandHistoryIndex = 0;
  let copyModalTimer = 0;
  let copyModalValue = "";
  let copyModalTitle = "Copied";

  function setNotice(msg, type, timeout) {
    util.setNotice("portal-generator-notice", msg, type, timeout);
  }

  function compactPreview(value, limit = 96) {
    const singleLine = String(value || "").replace(/\s+/g, " ").trim();
    if (!singleLine) return "";
    return singleLine.length > limit
      ? singleLine.slice(0, limit - 1) + "…"
      : singleLine;
  }

  function closeCopyModal() {
    const modal = dom.id("portal-copy-modal-bg");
    if (modal) modal.classList.remove("visible");
    if (copyModalTimer) {
      window.clearTimeout(copyModalTimer);
      copyModalTimer = 0;
    }
  }

  function copyModalReminder(kind, value) {
    const preview = String(value || "");
    if (kind === "endpoint") {
      return "This is only the endpoint URL. Add the token separately in curl, Postman, Bruno, or whichever client you use.";
    }
    if (preview.includes(keyShownOncePlaceholder()) || preview.includes("<paste-token>")) {
      return "Replace the token placeholder with the token you were given. Raw gateway keys are shown once and are not hosted in this web UI.";
    }
    return "This preview already includes the token currently loaded in this browser session. Handle that copied command carefully.";
  }

  function openCopyModal(kind, value) {
    copyModalValue = String(value || "");
    copyModalTitle = kind === "endpoint" ? "Endpoint Ready" : "Command Ready";
    dom.text("portal-copy-modal-title", copyModalTitle);
    dom.text("portal-copy-modal-copy", kind === "endpoint"
      ? "The endpoint is copied. Keep it as a fallback if you do not want the full curl command."
      : "The generated command is copied and ready to paste.");
    dom.text("portal-copy-modal-value", copyModalValue);
    dom.text("portal-copy-modal-reminder", copyModalReminder(kind, copyModalValue));
    const modal = dom.id("portal-copy-modal-bg");
    if (modal) modal.classList.add("visible");
    if (copyModalTimer) window.clearTimeout(copyModalTimer);
    copyModalTimer = window.setTimeout(closeCopyModal, 5200);
  }

  function allowedServices() {
    return (state.availableServices.length ? state.availableServices : state.serviceCatalog).slice().sort();
  }

  function selectedService() {
    const services = allowedServices();
    if (!services.length) {
      state.portalSelectedService = "";
      return "";
    }
    if (!state.portalSelectedService || !services.includes(state.portalSelectedService)) {
      state.portalSelectedService = services[0];
    }
    return state.portalSelectedService;
  }

  function normalizedPath(raw) {
    raw = String(raw || "").trim();
    if (!raw) return "/";
    return raw.startsWith("/") ? raw : "/" + raw;
  }

  function tokenValue() {
    const value = String(state.portalToken || "").trim();
    if (value) return value;
    if (hasAssignedToken()) return keyShownOncePlaceholder();
    return "<paste-token>";
  }

  function assignedTokenType() {
    return state.portalAssignedTokenType === "ai" ? "ai" : "standard";
  }

  function isAIKey() {
    return assignedTokenType() === "ai";
  }

  function hasAssignedToken() {
    return !!state.portalHasAssignedToken;
  }

  function ensureAssignedTokenLoaded(force = false) {
    if (force) state.portalToken = "";
  }

  function hasPastedToken() {
    return !!String(state.portalToken || "").trim();
  }

  function keyShownOncePlaceholder() {
    return "apig0_key_shown_once_paste_yours_here";
  }

  function keyShownOnceMessage() {
    const prefix = String(state.portalAssignedTokenPrefix || "").trim();
    return prefix
      ? "Assigned key prefix: " + prefix + ". Raw keys are only shown once at creation time."
      : "A key is assigned to this account, but raw keys are only shown once at creation time.";
  }

  function splitCommand(input) {
    const text = String(input || "").trim();
    if (!text) return ["", ""];
    const index = text.indexOf(" ");
    return index === -1
      ? [text.toLowerCase(), ""]
      : [text.slice(0, index).toLowerCase(), text.slice(index + 1).trim()];
  }

  function parseHeaders(raw) {
    return String(raw || "")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const index = line.indexOf(":");
        if (index === -1) return null;
        const name = line.slice(0, index).trim();
        const value = line.slice(index + 1).trim();
        if (!name) return null;
        return {name, value};
      })
      .filter(Boolean);
  }

  function formatHeaders(headers) {
    return headers.map((header) => header.name + ": " + header.value).join("\n");
  }

  function hasHeader(headers, name) {
    const match = String(name || "").toLowerCase();
    return headers.some((header) => header.name.toLowerCase() === match);
  }

  function upsertHeader(name, value) {
    const target = String(name || "").trim();
    if (!target) return false;
    const headers = parseHeaders(state.portalHeadersText);
    const next = [];
    let replaced = false;
    for (const header of headers) {
      if (header.name.toLowerCase() === target.toLowerCase()) {
        if (!replaced) {
          next.push({name: target, value});
          replaced = true;
        }
      } else {
        next.push(header);
      }
    }
    if (!replaced) next.push({name: target, value});
    state.portalHeadersText = formatHeaders(next);
    return true;
  }

  function removeHeader(name) {
    const target = String(name || "").trim().toLowerCase();
    if (!target) return false;
    const headers = parseHeaders(state.portalHeadersText);
    const next = headers.filter((header) => header.name.toLowerCase() !== target);
    if (next.length === headers.length) return false;
    state.portalHeadersText = formatHeaders(next);
    return true;
  }

  function activeHeaders() {
    const headers = parseHeaders(state.portalHeadersText);
    const rawToken = String(state.portalToken || "").trim();
    if (rawToken && !hasHeader(headers, "Authorization")) {
      headers.unshift({name: "Authorization", value: "Bearer " + tokenValue()});
    }
    return headers;
  }

  function exportHeaders() {
    const headers = activeHeaders().slice();
    if (hasAssignedToken() && !hasPastedToken() && !hasHeader(headers, "Authorization")) {
      headers.unshift({name: "Authorization", value: "Bearer " + keyShownOncePlaceholder()});
    }
    return headers;
  }

  function serviceBase() {
    const service = selectedService();
    return service ? state.gatewayOrigin + "/" + service : "";
  }

  function requestUrl() {
    const base = serviceBase();
    if (!base) return "";
    return base + normalizedPath(state.portalPath);
  }

  function headerValue() {
    const auth = exportHeaders().find((header) => header.name.toLowerCase() === "authorization");
    return auth ? auth.name + ": " + auth.value : "";
  }

  function aiBaseURL() {
    return state.gatewayOrigin + "/openai/v1";
  }

  function aiModels() {
    return Array.isArray(state.portalAssignedAllowedModels) ? state.portalAssignedAllowedModels : [];
  }

  function aiProviders() {
    return Array.isArray(state.portalAssignedAllowedProviders) ? state.portalAssignedAllowedProviders : [];
  }

  function pendingDeliveries() {
    return Array.isArray(state.portalPendingDeliveries) ? state.portalPendingDeliveries : [];
  }

  function nextPendingDelivery() {
    return pendingDeliveries()[0] || null;
  }

  function aiDefaultModel() {
    return aiModels()[0] || "fast";
  }

  function aiCurlSnippet() {
    return [
      "curl " + aiBaseURL() + "/chat/completions \\",
      '  -H "Authorization: Bearer ' + shellDoubleQuote(tokenValue()) + '" \\',
      '  -H "Content-Type: application/json" \\',
      "  -d '{\"model\":\"" + shellSingleQuote(aiDefaultModel()) + "\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello\"}]}'"
    ].join("\n");
  }

  function aiPythonSnippet() {
    return [
      "# Example using an OpenAI-compatible client against the AI gateway",
      "from openai import OpenAI",
      "",
      "client = OpenAI(",
      '    api_key="' + tokenValue() + '",',
      '    base_url="' + aiBaseURL() + '"',
      ")",
      "",
      "response = client.chat.completions.create(",
      '    model="' + aiDefaultModel() + '",',
      '    messages=[{"role": "user", "content": "Say hello"}],',
      ")",
      "",
      "print(response.choices[0].message.content)"
    ].join("\n");
  }

  function aiJavaScriptSnippet() {
    return [
      "// Example using an OpenAI-compatible client against the AI gateway",
      'import OpenAI from "openai";',
      "",
      "const client = new OpenAI({",
      '  apiKey: "' + tokenValue() + '",',
      '  baseURL: "' + aiBaseURL() + '"',
      "});",
      "",
      "const response = await client.chat.completions.create({",
      '  model: "' + aiDefaultModel() + '",',
      '  messages: [{ role: "user", content: "Say hello" }]',
      "});",
      "",
      "console.log(response.choices[0].message.content);"
    ].join("\n");
  }

  function assignedBackendLabel() {
    return String(state.portalAssignedBackendLabel || "").trim() || String(state.portalAssignedOpenAIService || "").trim() || "auto";
  }

  function renderPendingDeliveryCard() {
    const card = dom.id("portal-pending-delivery-card");
    const copy = dom.id("portal-pending-delivery-copy");
    const button = dom.id("portal-claim-next-key");
    if (!card || !copy || !button) return;

    const deliveries = pendingDeliveries();
    const next = nextPendingDelivery();
    if (!next) {
      card.style.display = "none";
      return;
    }

    const count = deliveries.length;
    const backend = next.backend_label || next.service || "gateway access";
    const expiresAt = next.expires_at ? new Date(next.expires_at).toLocaleString() : "soon";
    copy.textContent =
      (count === 1 ? "1 key is waiting" : count + " keys are waiting") +
      " for this account. Next delivery: " + (next.token_prefix || "pending key") +
      " for " + backend + ". Claim it before " + expiresAt + ".";
    button.textContent = count === 1 ? "Claim Key" : "Claim Next Key";
    card.style.display = "block";
  }

  function updateFormFromState() {
    const method = dom.id("portal-method");
    const path = dom.id("portal-path");
    const token = dom.id("portal-token");
    const headers = dom.id("portal-headers");
    const body = dom.id("portal-body");
    if (method && method.value !== state.portalMethod) method.value = state.portalMethod;
    if (path && path.value !== state.portalPath) path.value = state.portalPath;
    if (token && token.value !== state.portalToken) token.value = state.portalToken;
    if (headers && headers.value !== state.portalHeadersText) headers.value = state.portalHeadersText;
    if (body && body.value !== state.portalBody) body.value = state.portalBody;
  }

  function setUsageEnabled(enabled) {
    const tokenInput = dom.id("portal-token");
    const configDrawer = dom.id("portal-config-drawer");
    const actions = [
      dom.id("portal-copy-command-main"),
      dom.id("portal-copy-endpoint-main")
    ];
    if (tokenInput) tokenInput.disabled = !enabled;
    for (const button of actions) {
      if (button) button.disabled = !enabled;
    }
    if (configDrawer) configDrawer.open = enabled && configDrawer.open;
    if (configDrawer) configDrawer.style.opacity = enabled ? "1" : "0.55";
  }

  function renderServiceCards() {
    const container = dom.id("portal-services");
    if (!container) return;
    const services = allowedServices();
    container.innerHTML = "";
    if (!services.length) {
      container.innerHTML = '<div style="color:var(--text-dim);font-size:13px">No services are assigned to this account yet.</div>';
      render();
      return;
    }
    const active = selectedService();
    for (const name of services) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "portal-svc" + (name === active ? " active" : "");
      button.dataset.action = "portal-select-service";
      button.dataset.service = name;
      button.innerHTML =
        '<div class="portal-svc-name">' + util.escHtml(name) + "</div>" +
        '<div class="portal-svc-url">' + util.escHtml(state.gatewayOrigin + "/" + name) + "</div>";
      container.appendChild(button);
    }
    render();
  }

  function renderHistory() {
    const wrap = dom.id("portal-history");
    if (!wrap) return;
    if (!state.portalHistory.length) {
      wrap.innerHTML = '<div class="service-muted">No recent requests yet.</div>';
      return;
    }
    wrap.innerHTML = state.portalHistory.map((item, index) =>
      '<button type="button" class="portal-history-item" data-action="portal-apply-history" data-history-index="' + index + '">' +
      '<div class="portal-history-top">' +
      '<span class="portal-history-main">' + util.escHtml(item.label) + "</span>" +
      '<span class="portal-history-time">' + util.escHtml(new Date(item.ts).toLocaleTimeString()) + "</span>" +
      "</div>" +
      '<div class="portal-history-sub">' + util.escHtml(item.preview) + "</div>" +
      "</button>"
    ).join("");
  }

  function saveHistory() {
    try {
      window.localStorage.removeItem(historyKey);
    } catch {}
  }

  function loadHistory() {
    try {
      window.localStorage.removeItem(historyKey);
    } catch {
    }
    state.portalHistory = [];
  }

  function safeHistoryHeaders(raw) {
    return formatHeaders(parseHeaders(raw).filter((header) => {
      const name = header.name.toLowerCase();
      return !["authorization", "proxy-authorization", "x-api-key", "cookie", "set-cookie"].includes(name);
    }));
  }

  function recordHistory() {
    const service = selectedService();
    if (!service) return;
    const headersText = safeHistoryHeaders(state.portalHeadersText);
    const entry = {
      ts: Date.now(),
      service,
      method: state.portalMethod,
      path: normalizedPath(state.portalPath),
      body: "",
      headersText,
      insecure: !!dom.id("portal-insecure")?.checked,
      label: state.portalMethod + " " + service + normalizedPath(state.portalPath),
      preview: parseHeaders(headersText).length + " saved header" + (parseHeaders(headersText).length === 1 ? "" : "s")
    };
    const key = [
      entry.label,
      entry.headersText,
      entry.body,
      entry.insecure ? "1" : "0"
    ].join("|");
    state.portalHistory = [entry].concat(
      state.portalHistory.filter((existing) => [
        existing.label,
        existing.headersText || "",
        existing.body || "",
        existing.insecure ? "1" : "0"
      ].join("|") !== key)
    ).slice(0, 20);
    saveHistory();
    renderHistory();
  }

  function applyHistory(index) {
    const item = state.portalHistory[index];
    if (!item) return;
    state.portalSelectedService = item.service || state.portalSelectedService;
    state.portalMethod = item.method || "GET";
    state.portalPath = item.path || "/";
    state.portalBody = item.body || "";
    state.portalToken = "";
    state.portalHeadersText = safeHistoryHeaders(item.headersText || "");
    const insecure = dom.id("portal-insecure");
    if (insecure) insecure.checked = item.insecure !== false;
    updateFormFromState();
    renderServiceCards();
    pushTerminal([
      {type: "ok", text: "Restored " + (item.label || "request") + "."}
    ]);
  }

  function clearHistory() {
    state.portalHistory = [];
    saveHistory();
    renderHistory();
  }

  function exportsBlock() {
    return [
      'export APIG0_BASE="' + state.gatewayOrigin + '"',
      'export APIG0_SERVICE="' + (selectedService() || "<service>") + '"',
      'export APIG0_TOKEN="' + tokenValue() + '"'
    ].join("\n");
  }

  function shellSingleQuote(value) {
    return String(value || "").replace(/'/g, "'\\''");
  }

  function shellDoubleQuote(value) {
    return String(value || "").replace(/(["\\$`])/g, "\\$1");
  }

  function shellArg(value) {
    return "'" + shellSingleQuote(value) + "'";
  }

  function buildCommand() {
    const insecure = dom.id("portal-insecure")?.checked;
    const method = state.portalMethod;
    const body = String(state.portalBody || "").trim();
    const headers = exportHeaders().slice();
    if (body && method !== "GET" && method !== "DELETE" && !hasHeader(headers, "Content-Type")) {
      headers.push({name: "Content-Type", value: "application/json"});
    }
    const parts = ["curl"];
    if (insecure) parts.push("-k");
    parts.push("-X", method);
    for (const header of headers) {
      parts.push("-H", shellArg(header.name + ": " + header.value));
    }
    if (body && method !== "GET" && method !== "DELETE") {
      parts.push("--data", shellArg(body));
    }
    parts.push(shellArg(requestUrl()));
    return parts.join(" ");
  }

  function activeCommand() {
    return selectedService() ? buildCommand() : "";
  }

  function terminalLine(type, text) {
    return {type, text};
  }

  function pushTerminal(lines) {
    terminalLines.push(...lines);
    while (terminalLines.length > 120) terminalLines.shift();
    renderTerminal();
  }

  function seedTerminal() {
    terminalLines.length = 0;
    pushTerminal([
      terminalLine("info", "Local terminal ready. It can now call the selected gateway service from the browser."),
      terminalLine("muted", "Try: get / | post /items {\"name\":\"demo\"} | send | copy command")
    ]);
  }

  function renderTerminal() {
    const screen = dom.id("portal-terminal-screen");
    if (!screen) return;
    const prefixMap = {
      prompt: "$",
      info: ">",
      ok: "=",
      err: "!",
      muted: "."
    };
    screen.innerHTML = terminalLines.map((line) =>
      '<div class="portal-terminal-line ' + util.escAttr(line.type) + '">' +
      '<span class="portal-terminal-prefix">' + util.escHtml(prefixMap[line.type] || ">") + "</span>" +
      '<span class="portal-terminal-text">' + util.escHtml(line.text) + "</span>" +
      "</div>"
    ).join("");
    screen.scrollTop = screen.scrollHeight;
  }

  function printHelp() {
    pushTerminal([
      terminalLine("info", "help"),
      terminalLine("muted", "service <name>"),
      terminalLine("muted", "method GET|POST|PUT|PATCH|DELETE"),
      terminalLine("muted", "path /v1/items"),
      terminalLine("muted", "get /path"),
      terminalLine("muted", "post /path {\"json\":true}"),
      terminalLine("muted", "send"),
      terminalLine("muted", "token <value>"),
      terminalLine("muted", "header X-Name: value"),
      terminalLine("muted", "unheader X-Name"),
      terminalLine("muted", "body {\"json\":true}"),
      terminalLine("muted", "tls on|off"),
      terminalLine("muted", "print | headers | exports | copy command | reset | clear")
    ]);
  }

  function renderContext() {
    ensureAssignedTokenLoaded();
    if (isAIKey()) {
      renderAIInterface();
      return;
    }
    const service = selectedService();
    const endpoint = requestUrl();
    const hasService = !!service;
    const headers = exportHeaders();
    const tokenAssigned = hasAssignedToken();
    const emptyState = dom.id("portal-service-empty");
    const readyState = dom.id("portal-service-ready");
    const configDrawer = dom.id("portal-config-drawer");
    const tokenEmpty = dom.id("portal-token-empty");
    const terminalEmpty = dom.id("portal-terminal-empty");
    const usageEnabled = hasService;
    const tokenLoaded = hasPastedToken();
    dom.text("portal-selected-service-copy", service ? "Selected service: " + service : "Select a service to begin");
    dom.text("portal-endpoint", endpoint || "Select a service to generate commands.");
    dom.text("portal-header-block", headers.map((header) => header.name + ": " + header.value).join("\n"));
    dom.text("portal-env-block", exportsBlock());
    dom.text("portal-key-type", "Standard");
    dom.text("portal-key-prefix", tokenAssigned ? (state.portalAssignedTokenPrefix || "-") : "-");
    dom.text("portal-token-status", !tokenAssigned ? "Missing" : (tokenLoaded ? "Loaded" : "Assigned"));
    dom.text("portal-method-status", state.portalMethod);
    if (emptyState) emptyState.style.display = hasService ? "none" : "block";
    if (readyState) readyState.style.display = hasService ? "flex" : "none";
    if (configDrawer) configDrawer.style.display = hasService ? "block" : "none";
    if (tokenEmpty) tokenEmpty.style.display = hasService && !tokenAssigned ? "block" : "none";
    if (terminalEmpty) terminalEmpty.style.display = hasService && !tokenAssigned ? "block" : "none";
    setUsageEnabled(usageEnabled);
    dom.text("portal-context-copy", service
      ? (tokenAssigned
        ? "Set the token you were given before running the command. If you leave it blank, the copied preview keeps a safe placeholder."
        : "A service is available, but this username has no active token assigned yet.")
      : "Wait for a service to be assigned, then click it to generate a copy-ready request.");
    dom.text("portal-response-meta", service
      ? (tokenAssigned ? "curl preview for " + service : "Waiting for assigned key")
      : "Copy-ready curl");
    dom.text("portal-response-headers", [
      "Endpoint: " + (endpoint || "-"),
      "Method: " + state.portalMethod,
      "Key: " + (!tokenAssigned ? "Missing" : (tokenLoaded ? "Loaded in browser session" : "Set the token you were given"))
    ].join("\n"));
  }

  function renderAIInterface() {
    const standard = dom.id("portal-standard-interface");
    const ai = dom.id("portal-ai-interface");
    const actions = dom.id("portal-surface-actions");
    if (standard) standard.style.display = "none";
    if (ai) ai.style.display = "block";
    if (actions) actions.style.display = "none";
    dom.text("portal-surface-title", "AI Access");
    dom.text("portal-surface-copy", "This key is meant for SDK and API clients. Use the gateway base URL below with OpenAI-compatible clients. Raw keys are shown only once when created.");
    dom.text("portal-selected-service-copy", "Assigned AI key");
    dom.text("portal-context-copy", "This account has an AI gateway key. Copy the base URL and snippets below to call models through `/openai/v1`.");
    dom.text("portal-ai-endpoint", aiBaseURL());
    dom.text("portal-ai-key-type", "AI");
    dom.text("portal-ai-key-prefix", state.portalAssignedTokenPrefix || "-");
    dom.text("portal-ai-service", assignedBackendLabel());
    dom.text("portal-ai-model-count", aiModels().length ? String(aiModels().length) : "All");
    dom.text("portal-ai-models", aiModels().length ? aiModels().join("\n") : "All models allowed for this key.");
    dom.text("portal-ai-providers", [keyShownOnceMessage()].concat(aiProviders().length ? ["", "Allowed providers:", ...aiProviders()] : ["", "All providers allowed for this key."]).join("\n"));
    dom.text("portal-ai-meta", "Universal AI gateway key for " + (state.currentUser || "user"));
    dom.text("portal-ai-curl", hasAssignedToken() ? aiCurlSnippet() : "No active AI key is assigned to this username yet.");
    dom.text("portal-ai-python", hasAssignedToken() ? aiPythonSnippet() : "No active AI key is assigned to this username yet.");
    dom.text("portal-ai-javascript", hasAssignedToken() ? aiJavaScriptSnippet() : "No active AI key is assigned to this username yet.");
    const tokenEmpty = dom.id("portal-ai-token-empty");
    if (tokenEmpty) tokenEmpty.style.display = hasAssignedToken() ? "none" : "block";
  }

  function renderPreview() {
    if (isAIKey()) return;
    const service = selectedService();
    if (!service) {
      dom.text("portal-response-body", "Choose a service to generate a command.");
      return;
    }
    if (!hasAssignedToken()) {
      dom.text("portal-response-body", "No active token is assigned to `" + (state.currentUser || "this user") + "`. An admin must create one before this terminal can be used.");
      return;
    }
    dom.text("portal-response-body", activeCommand());
  }

  function render() {
    updateFormFromState();
    const standard = dom.id("portal-standard-interface");
    const ai = dom.id("portal-ai-interface");
    const actions = dom.id("portal-surface-actions");
    if (standard) standard.style.display = isAIKey() ? "none" : "block";
    if (ai) ai.style.display = isAIKey() ? "block" : "none";
    if (!isAIKey()) {
      if (actions) actions.style.display = "flex";
      dom.text("portal-surface-title", "API Access");
      dom.text("portal-surface-copy", "");
    }
    renderPendingDeliveryCard();
    renderContext();
    renderPreview();
  }

  function syncStateFromInputs() {
    state.portalMethod = dom.id("portal-method")?.value || "GET";
    state.portalPath = normalizedPath(dom.id("portal-path")?.value || "/");
    state.portalToken = dom.id("portal-token")?.value || "";
    state.portalHeadersText = dom.id("portal-headers")?.value || "";
    state.portalBody = dom.id("portal-body")?.value || "";
  }

  async function copyText(value, successMessage) {
    if (!String(value || "").trim()) {
      setNotice("Select a service first.", "err");
      return false;
    }
    try {
      await navigator.clipboard.writeText(value);
      const preview = compactPreview(value);
      setNotice(preview ? successMessage + " " + preview : successMessage, "ok", 2200);
      return true;
    } catch {
      setNotice("Clipboard copy failed in this browser.", "err");
      return false;
    }
  }

  async function claimNextPendingKey() {
    const delivery = nextPendingDelivery();
    if (!delivery) {
      setNotice("No pending key deliveries were found.", "err");
      return;
    }
    try {
      const {ok, data} = await App.api.post("/api/user/pending-keys/" + encodeURIComponent(delivery.id) + "/claim", undefined, {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed to claim key.", "err");
        await App.auth.loadPendingKeyDeliveries();
        render();
        return;
      }
      state.portalToken = data.raw_token || "";
      updateFormFromState();
      await App.auth.loadPendingKeyDeliveries();
      render();
      const baseHint = isAIKey() ? "\n\nGateway API Base URL: " + aiBaseURL() : "";
      window.prompt("Copy this gateway key now. It will not be shown again.", (data.raw_token || "") + baseHint);
      setNotice("Key claimed. It is now loaded in this browser session only.", "ok", 6000);
    } catch {
      setNotice("Network error while claiming key.", "err");
    }
  }

  async function refreshPendingKeys() {
    await App.auth.loadPendingKeyDeliveries();
    render();
    setNotice("Pending key deliveries refreshed.", "ok");
  }

  function selectService(name) {
    state.portalSelectedService = name;
    renderServiceCards();
  }

  function resetBuilder() {
    state.portalMethod = "GET";
    state.portalPath = "/";
    ensureAssignedTokenLoaded(true);
    state.portalHeadersText = "";
    state.portalBody = "";
    const insecure = dom.id("portal-insecure");
    if (insecure) insecure.checked = true;
    updateFormFromState();
    render();
  }

  async function copyCommand() {
    if (isAIKey()) {
      return copyText(aiCurlSnippet(), "curl snippet copied.");
    }
    syncStateFromInputs();
    render();
    recordHistory();
    return copyText(activeCommand(), "Command copied.");
  }

  async function copyHeaders() {
    syncStateFromInputs();
    render();
    return copyText(exportHeaders().map((header) => header.name + ": " + header.value).join("\n"), "Headers copied.");
  }

  function requestHeaders(includeBody) {
    const headers = {};
    for (const header of activeHeaders()) {
      headers[header.name] = header.value;
    }
    if (includeBody && !Object.keys(headers).some((name) => name.toLowerCase() === "content-type")) {
      headers["Content-Type"] = "application/json";
    }
    return headers;
  }

  function formatResponseBody(text, contentType) {
    const raw = String(text || "");
    if (!raw.trim()) return ["(empty response)"];
    if (String(contentType || "").includes("application/json")) {
      try {
        return JSON.stringify(JSON.parse(raw), null, 2).split("\n");
      } catch {}
    }
    return raw.split("\n");
  }

  async function callGatewayRequest(method = state.portalMethod, path = state.portalPath, bodyOverride = state.portalBody) {
    const service = selectedService();
    if (!service) {
      pushTerminal([terminalLine("err", "Select a service first.")]);
      return;
    }
    if (!hasAssignedToken()) {
      pushTerminal([terminalLine("err", "No active token is assigned to this username.")]);
      return;
    }

    const normalized = normalizedPath(path || "/");
    state.portalMethod = String(method || "GET").toUpperCase();
    state.portalPath = normalized;
    if (bodyOverride !== undefined) {
      state.portalBody = String(bodyOverride || "");
    }
    updateFormFromState();
    render();

    const includeBody = !["GET", "DELETE"].includes(state.portalMethod) && String(state.portalBody || "").trim() !== "";
    const requestInit = {
      method: state.portalMethod,
      headers: requestHeaders(includeBody)
    };
    if (includeBody) {
      requestInit.body = state.portalBody;
    }

    const url = requestUrl();
    pushTerminal([terminalLine("info", "Requesting " + state.portalMethod + " " + url)]);
    try {
      const response = await fetch(url, requestInit);
      const text = await response.text();
      const lines = [
        terminalLine(response.ok ? "ok" : "err", "HTTP " + response.status + " " + state.portalMethod + " " + normalized)
      ].concat(formatResponseBody(text, response.headers.get("content-type")).map((line) => terminalLine("info", line)));
      pushTerminal(lines);
      recordHistory();
    } catch (err) {
      pushTerminal([terminalLine("err", "Request failed: " + (err && err.message ? err.message : "network error"))]);
    }
  }

  function prewriteTerminal(value, message) {
    const input = dom.id("portal-terminal-input");
    if (!input) return;
    input.value = value;
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
    if (message) {
      pushTerminal([terminalLine("info", message)]);
    }
  }

  function primeTerminal() {
    if (isAIKey()) {
      setNotice("AI keys use the snippet cards below instead of the browser terminal.", "ok");
      return;
    }
    syncStateFromInputs();
    const service = selectedService();
    if (!service) {
      setNotice("Select a service first.", "err");
      pushTerminal([terminalLine("err", "Select a service before prewriting terminal commands.")]);
      return;
    }
    if (!hasAssignedToken()) {
      setNotice("No active token is assigned to this user.", "err");
      pushTerminal([terminalLine("err", "No active token is assigned to this username.")]);
      return;
    }
    prewriteTerminal(
      state.portalMethod.toLowerCase() + " " + normalizedPath(state.portalPath),
      "Request command prepared for " + service + ". Press Apply Line to call it now."
    );
    setNotice("Request command prewritten in terminal.", "ok");
  }

  function rememberCommand(input) {
    if (!input) return;
    if (!commandHistory.length || commandHistory[commandHistory.length - 1] !== input) {
      commandHistory.push(input);
    }
    commandHistoryIndex = commandHistory.length;
  }

  async function handleCopyTarget(target) {
    const value = String(target || "command").toLowerCase();
    if (value === "command") {
      const ok = await copyCommand();
      pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Command copied to clipboard." : "Copy failed.")]);
      return;
    }
    if (value === "headers") {
      const ok = await copyHeaders();
      pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Headers copied to clipboard." : "Copy failed.")]);
      return;
    }
    if (value === "endpoint") {
      const ok = await copyText(requestUrl(), "Endpoint copied.");
      pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Endpoint copied to clipboard." : "Copy failed.")]);
      return;
    }
    if (value === "exports") {
      const ok = await copyText(exportsBlock(), "Exports copied.");
      pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Exports copied to clipboard." : "Copy failed.")]);
      return;
    }
    if (value === "auth") {
      const ok = await copyText(headerValue(), "Auth header copied.");
      pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Auth header copied to clipboard." : "Copy failed.")]);
      return;
    }
    pushTerminal([terminalLine("err", 'Unknown copy target "' + target + '".')]);
  }

  async function executeTerminalInput(rawInput) {
    if (isAIKey()) {
      setNotice("AI keys do not use the browser terminal.", "err");
      return;
    }
    syncStateFromInputs();
    if (!hasAssignedToken()) {
      setNotice("No active token is assigned to this user.", "err");
      pushTerminal([terminalLine("err", "Terminal usage is locked until this username has an active token.")]);
      return;
    }
    const input = String(rawInput || "").trim();
    if (!input) return;
    rememberCommand(input);
    pushTerminal([terminalLine("prompt", input)]);
    const [command, rest] = splitCommand(input);
    if (!command) return;

    if (command === "help") {
      printHelp();
      return;
    }

    if (["get", "post", "put", "patch", "delete"].includes(command)) {
      if (!rest) {
        pushTerminal([terminalLine("err", "Usage: " + command + " /path" + (command === "get" || command === "delete" ? "" : " {json-body}"))]);
        return;
      }
      const firstSpace = rest.indexOf(" ");
      const pathPart = firstSpace === -1 ? rest : rest.slice(0, firstSpace).trim();
      const bodyPart = firstSpace === -1 ? "" : rest.slice(firstSpace + 1).trim();
      await callGatewayRequest(command.toUpperCase(), pathPart, bodyPart);
      return;
    }

    if (command === "service") {
      const services = allowedServices();
      if (!rest) {
        pushTerminal([terminalLine("err", "Usage: service <name>")]);
        return;
      }
      if (!services.includes(rest)) {
        pushTerminal([terminalLine("err", 'Service "' + rest + '" is not available to this account.')]);
        return;
      }
      state.portalSelectedService = rest;
      renderServiceCards();
      pushTerminal([terminalLine("ok", "Service set to " + rest + ".")]);
      return;
    }

    if (command === "method") {
      const method = rest.toUpperCase();
      if (!["GET", "POST", "PUT", "PATCH", "DELETE"].includes(method)) {
        pushTerminal([terminalLine("err", "Allowed methods: GET, POST, PUT, PATCH, DELETE.")]);
        return;
      }
      state.portalMethod = method;
      render();
      pushTerminal([terminalLine("ok", "Method set to " + method + ".")]);
      return;
    }

    if (command === "path") {
      state.portalPath = normalizedPath(rest || "/");
      render();
      pushTerminal([terminalLine("ok", "Path set to " + state.portalPath + ".")]);
      return;
    }

    if (command === "token") {
      state.portalToken = rest;
      render();
      pushTerminal([terminalLine("ok", rest ? "Token updated." : "Token cleared.")]);
      return;
    }

    if (command === "header") {
      const index = rest.indexOf(":");
      if (index === -1) {
        pushTerminal([terminalLine("err", "Usage: header Name: value")]);
        return;
      }
      const name = rest.slice(0, index).trim();
      const value = rest.slice(index + 1).trim();
      if (!name) {
        pushTerminal([terminalLine("err", "Header name is required.")]);
        return;
      }
      upsertHeader(name, value);
      render();
      pushTerminal([terminalLine("ok", "Header " + name + " updated.")]);
      return;
    }

    if (command === "unheader") {
      if (!rest) {
        pushTerminal([terminalLine("err", "Usage: unheader Name")]);
        return;
      }
      const removed = removeHeader(rest);
      render();
      pushTerminal([terminalLine(removed ? "ok" : "err", removed ? "Header " + rest + " removed." : "Header " + rest + " was not set.")]);
      return;
    }

    if (command === "body") {
      state.portalBody = rest;
      render();
      pushTerminal([terminalLine("ok", rest ? "Request body updated." : "Request body cleared.")]);
      return;
    }

    if (command === "tls") {
      const insecure = dom.id("portal-insecure");
      const next = rest.toLowerCase();
      if (!insecure || !["on", "off"].includes(next)) {
        pushTerminal([terminalLine("err", "Usage: tls on|off")]);
        return;
      }
      insecure.checked = next === "on";
      render();
      pushTerminal([terminalLine("ok", "TLS insecure flag " + (insecure.checked ? "enabled." : "disabled."))]);
      return;
    }

    if (command === "print" || command === "show" || command === "preview") {
      const value = activeCommand();
      pushTerminal([terminalLine(value ? "info" : "err", value || "Select a service first.")]);
      return;
    }

    if (command === "headers") {
      pushTerminal([terminalLine("info", exportHeaders().map((header) => header.name + ": " + header.value).join(" | "))]);
      return;
    }

    if (command === "send" || command === "run" || command === "request") {
      await callGatewayRequest();
      return;
    }

    if (command === "exports") {
      pushTerminal(exportsBlock().split("\n").map((line) => terminalLine("info", line)));
      return;
    }

    if (command === "copy") {
      await handleCopyTarget(rest || "command");
      return;
    }

    if (command === "reset") {
      resetBuilder();
      pushTerminal([terminalLine("ok", "Request builder reset. Token preserved.")]);
      return;
    }

    if (command === "clear") {
      seedTerminal();
      return;
    }

    pushTerminal([terminalLine("err", 'Unknown command "' + command + '". Type help.')]);
  }

  async function runTerminalInput() {
    if (isAIKey()) {
      setNotice("Use the AI snippet buttons below for this key type.", "err");
      return;
    }
    const input = dom.id("portal-terminal-input");
    if (!input) return;
    const value = input.value;
    input.value = "";
    if (!String(value || "").trim()) {
      await callGatewayRequest();
      return;
    }
    await executeTerminalInput(value);
  }

  function clearTerminal() {
    if (isAIKey()) return;
    seedTerminal();
    setNotice("Terminal cleared.", "ok");
  }

  function bindTerminalInput() {
    const input = dom.id("portal-terminal-input");
    if (!input || input.dataset.bound === "true") return;
    input.dataset.bound = "true";
    input.addEventListener("keydown", async (event) => {
      if (event.key === "Enter") {
        event.preventDefault();
        await runTerminalInput();
        return;
      }
      if (event.key === "ArrowUp") {
        if (!commandHistory.length) return;
        event.preventDefault();
        commandHistoryIndex = Math.max(0, commandHistoryIndex - 1);
        input.value = commandHistory[commandHistoryIndex] || "";
        return;
      }
      if (event.key === "ArrowDown") {
        if (!commandHistory.length) return;
        event.preventDefault();
        commandHistoryIndex = Math.min(commandHistory.length, commandHistoryIndex + 1);
        input.value = commandHistoryIndex >= commandHistory.length ? "" : (commandHistory[commandHistoryIndex] || "");
      }
    });
  }

  function bindInputs() {
    [
      "portal-method",
      "portal-path",
      "portal-token",
      "portal-headers",
      "portal-body",
      "portal-insecure"
    ].forEach((id) => {
      const element = dom.id(id);
      if (!element || element.dataset.bound === "true") return;
      element.dataset.bound = "true";
      element.addEventListener("input", () => {
        syncStateFromInputs();
        render();
      });
      element.addEventListener("change", () => {
        syncStateFromInputs();
        render();
      });
    });
    bindTerminalInput();
  }

  function init() {
    loadHistory();
    renderHistory();
    seedTerminal();
    bindInputs();
    const copyModal = dom.id("portal-copy-modal-bg");
    if (copyModal && copyModal.dataset.bound !== "true") {
      copyModal.dataset.bound = "true";
      copyModal.addEventListener("click", (event) => {
        if (event.target === copyModal) closeCopyModal();
      });
    }
    ensureAssignedTokenLoaded(true);
    syncStateFromInputs();
    render();
  }

  App.actions["portal-select-service"] = (el) => selectService(el.dataset.service);
  App.actions["portal-copy-command"] = async () => {
    if (isAIKey()) {
      const value = aiCurlSnippet();
      const ok = await copyText(value, "curl snippet copied.");
      if (ok) openCopyModal("command", value);
      return;
    }
    syncStateFromInputs();
    render();
    recordHistory();
    const value = activeCommand();
    const ok = await copyText(value, "Command copied.");
    if (ok) openCopyModal("command", value);
  };
  App.actions["portal-prime-terminal"] = primeTerminal;
  App.actions["portal-claim-next-key"] = claimNextPendingKey;
  App.actions["portal-refresh-pending-keys"] = refreshPendingKeys;
  App.actions["portal-copy-headers"] = async () => {
    const ok = await copyHeaders();
    pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Headers copied to clipboard." : "Copy failed.")]);
  };
  App.actions["portal-copy-auth-header"] = async () => {
    const ok = await copyText(headerValue(), "Auth header copied.");
    pushTerminal([terminalLine(ok ? "ok" : "err", ok ? "Auth header copied to clipboard." : "Copy failed.")]);
  };
  App.actions["portal-copy-endpoint"] = async () => {
    syncStateFromInputs();
    render();
    const value = requestUrl();
    const ok = await copyText(value, "Endpoint copied.");
    if (ok) openCopyModal("endpoint", value);
  };
  App.actions["portal-close-copy-modal"] = closeCopyModal;
  App.actions["portal-copy-modal-copy"] = async () => {
    if (!copyModalValue) {
      closeCopyModal();
      return;
    }
    const ok = await copyText(copyModalValue, copyModalTitle + " copied.");
    if (ok) openCopyModal(copyModalTitle === "Endpoint Ready" ? "endpoint" : "command", copyModalValue);
  };
  App.actions["portal-clear-history"] = clearHistory;
  App.actions["portal-apply-history"] = (el) => applyHistory(parseInt(el.dataset.historyIndex, 10));
  App.actions["portal-reset-builder"] = () => {
    resetBuilder();
    setNotice("Request builder reset.", "ok");
  };
  App.actions["portal-run-command"] = runTerminalInput;
  App.actions["portal-clear-terminal"] = clearTerminal;
  App.actions["portal-copy-ai-base-url"] = async () => {
    await copyText(aiBaseURL(), "AI base URL copied.");
  };
  App.actions["portal-copy-ai-curl"] = async () => {
    await copyText(aiCurlSnippet(), "AI curl snippet copied.");
  };
  App.actions["portal-copy-ai-python"] = async () => {
    await copyText(aiPythonSnippet(), "Python snippet copied.");
  };
  App.actions["portal-copy-ai-javascript"] = async () => {
    await copyText(aiJavaScriptSnippet(), "JavaScript snippet copied.");
  };

  App.portal = {
    init,
    render,
    renderContext,
    renderServiceCards
  };
})(window.App);
