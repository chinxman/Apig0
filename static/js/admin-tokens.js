(function(App) {
  const {state, dom, api, util} = App;

  function hasRealTimestamp(value) {
    if (!value) return false;
    return !String(value).startsWith("0001-01-01");
  }

  function renderTokenServiceChecks() {
    const wrap = dom.id("token-service-checks");
    if (!wrap) return;
    wrap.innerHTML = "";
    for (const service of state.serviceCatalog) {
      const label = document.createElement("label");
      label.className = "service-check";
      label.innerHTML = '<input type="checkbox" value="' + util.escAttr(service) + '" checked> <span>' + util.escHtml(service) + "</span>";
      wrap.appendChild(label);
    }
    renderOpenAIServiceOptions();
  }

  function renderOpenAIServiceOptions() {
    const select = dom.id("token-openai-service");
    if (!select) return;
    const services = (state.openAIServiceCatalog && state.openAIServiceCatalog.length)
      ? state.openAIServiceCatalog
      : state.adminServices.filter((service) => service.openai_compatible).map((service) => service.name).sort();
    const current = select.value;
    select.innerHTML = '<option value="">disabled</option>';
    for (const service of services) {
      const option = document.createElement("option");
      option.value = service;
      option.textContent = service;
      select.appendChild(option);
    }
    select.value = services.includes(current) ? current : "";
  }

  function tokenKeyType() {
    return dom.id("token-key-type")?.value === "ai" ? "ai" : "standard";
  }

  function updateTokenTypeFields() {
    const aiScope = dom.id("token-ai-scope");
    if (!aiScope) return;
    aiScope.style.display = tokenKeyType() === "ai" ? "block" : "none";
  }

  function parseLines(value, lower = false) {
    return Array.from(new Set(String(value || "")
      .split(/\r?\n|,/)
      .map((item) => lower ? item.trim().toLowerCase() : item.trim())
      .filter(Boolean)));
  }

  function toggleTokenForm() {
    const form = dom.id("token-form");
    form.classList.toggle("visible");
    if (form.classList.contains("visible")) {
      renderTokenServiceChecks();
      updateTokenTypeFields();
      dom.id("token-name").focus();
    }
  }

  async function loadAdminTokens() {
    try {
      const {ok, data} = await api.get("/api/admin/tokens", {csrf: true});
      if (!ok) return;
      state.adminTokens = Array.isArray(data.tokens) ? data.tokens : [];
      state.openAIServiceCatalog = Array.isArray(data.openai_service_catalog) ? data.openai_service_catalog : state.openAIServiceCatalog;
      renderOpenAIServiceOptions();
      renderAdminTokens();
    } catch {}
  }

  function renderAdminTokens() {
    const tbody = dom.id("tokens-tbody");
    if (!tbody) return;
    tbody.innerHTML = "";
    if (!state.adminTokens.length) {
      tbody.innerHTML = '<tr><td colspan="7" style="color:var(--text-dim);padding:16px;text-align:center">No gateway tokens yet</td></tr>';
      return;
    }
    for (const token of state.adminTokens) {
      const revoked = hasRealTimestamp(token.revoked_at);
      const expired = hasRealTimestamp(token.expires_at) && new Date(token.expires_at).getTime() < Date.now();
      const scopeParts = [];
      scopeParts.push("type: " + (token.key_type || "standard"));
      if (Array.isArray(token.allowed_services) && token.allowed_services.length) {
        scopeParts.push(token.allowed_services.join(", "));
      } else {
        scopeParts.push("user access");
      }
      if (token.openai_service) scopeParts.push("AI via " + token.openai_service);
      if (Array.isArray(token.allowed_models) && token.allowed_models.length) scopeParts.push("models: " + token.allowed_models.join(", "));
      if (Array.isArray(token.allowed_providers) && token.allowed_providers.length) scopeParts.push("providers: " + token.allowed_providers.join(", "));
      if ((token.rate_limit_rpm || 0) > 0) scopeParts.push("limit: " + token.rate_limit_rpm + " rpm / " + (token.rate_limit_burst || 1) + " burst");
      const scope = scopeParts.join(" • ");
      const status = revoked ? "revoked" : (expired ? "expired" : "active");
      const tr = document.createElement("tr");
      tr.innerHTML =
        '<td><strong style="color:var(--purple)">' + util.escHtml(token.name || token.token_prefix) + '</strong><div class="service-muted" style="margin-top:4px">prefix ' + util.escHtml(token.token_prefix || "-") + "</div></td>" +
        "<td>" + util.escHtml(token.user || "-") + "</td>" +
        '<td style="max-width:240px;word-break:break-word">' + util.escHtml(scope) + "</td>" +
        "<td>" + util.displayDateTime(token.expires_at) + "</td>" +
        "<td>" + util.displayDateTime(token.last_used_at) + "</td>" +
        '<td><span class="role-pill ' + (status === "active" ? "role-user" : "") + '">' + util.escHtml(status) + "</span></td>" +
        '<td><div class="actions">' + (status === "active" ? '<button class="btn-danger" data-action="revoke-token" data-token-id="' + util.escAttr(token.id) + '">Revoke</button>' : "") + "</div></td>";
      tbody.appendChild(tr);
    }
  }

  async function createAdminToken() {
    const name = dom.id("token-name").value.trim();
    const user = dom.id("token-user").value.trim();
    const expires = dom.id("token-expires").value;
    const keyType = tokenKeyType();
    const allowedServices = util.getCheckedValues(dom.qsa('#token-service-checks input[type="checkbox"]'));
    const openAIService = keyType === "ai" ? dom.id("token-openai-service").value : "";
    const allowedModels = keyType === "ai" ? parseLines(dom.id("token-allowed-models").value) : [];
    const allowedProviders = keyType === "ai" ? parseLines(dom.id("token-allowed-providers").value, true) : [];
    const rateLimitRPM = keyType === "ai" ? (parseInt(dom.id("token-rate-limit-rpm").value, 10) || 0) : 0;
    const rateLimitBurst = keyType === "ai" ? (parseInt(dom.id("token-rate-limit-burst").value, 10) || 0) : 0;
    if (!user) {
      App.users.setNotice("Token user is required", "err");
      return;
    }
    try {
      const {ok, data} = await api.post("/api/admin/tokens", {
        name,
        user,
        key_type: keyType,
        allowed_services: allowedServices.length === state.serviceCatalog.length ? [] : allowedServices,
        openai_service: openAIService,
        allowed_models: allowedModels,
        allowed_providers: allowedProviders,
        rate_limit_rpm: rateLimitRPM,
        rate_limit_burst: rateLimitBurst,
        expires_at: expires ? new Date(expires).toISOString() : ""
      }, {csrf: true});
      if (!ok) {
        App.users.setNotice((data && data.error) || "Failed to create token", "err");
        return;
      }
      dom.id("token-name").value = "";
      dom.id("token-user").value = "";
      dom.id("token-expires").value = "";
      dom.id("token-key-type").value = "standard";
      dom.id("token-openai-service").value = "";
      dom.id("token-allowed-models").value = "";
      dom.id("token-allowed-providers").value = "";
      dom.id("token-rate-limit-rpm").value = "0";
      dom.id("token-rate-limit-burst").value = "0";
      dom.qsa('#token-service-checks input[type="checkbox"]').forEach((cb) => { cb.checked = true; });
      updateTokenTypeFields();
      toggleTokenForm();
      await loadAdminTokens();
      if (data.delivery_ready) {
        const expiresAt = data.delivery && data.delivery.expires_at
          ? new Date(data.delivery.expires_at).toLocaleString()
          : "soon";
        App.users.setNotice('Gateway token created. "' + user + '" can claim it once from the portal until ' + expiresAt + ".", "ok", 6000);
        return;
      }
      const copyText = data.raw_token || "";
      const hint = data.token && data.token.openai_service
        ? copyText + "\n\nAI gateway base URL: " + state.gatewayOrigin + "/openai/v1"
        : copyText;
      window.prompt("Secure delivery was unavailable. Copy the new token now.", hint);
      App.users.setNotice((data.delivery_error || "Gateway token created") + ". Manual delivery fallback was used.", "err", 6000);
    } catch {
      App.users.setNotice("Network error", "err");
    }
  }

  async function revokeAdminToken(id) {
    if (!window.confirm("Revoke this gateway token?")) return;
    try {
      const {ok, data} = await api.post("/api/admin/tokens/" + encodeURIComponent(id) + "/revoke", undefined, {csrf: true});
      if (!ok) {
        App.users.setNotice((data && data.error) || "Failed to revoke token", "err");
        return;
      }
      await loadAdminTokens();
      App.users.setNotice("Gateway token revoked", "ok");
    } catch {
      App.users.setNotice("Network error", "err");
    }
  }

  App.actions["toggle-token-form"] = toggleTokenForm;
  App.actions["create-token"] = createAdminToken;
  App.actions["revoke-token"] = (el) => revokeAdminToken(el.dataset.tokenId);
  App.actions["update-token-type-fields"] = updateTokenTypeFields;

  App.tokens = {
    renderTokenServiceChecks,
    renderOpenAIServiceOptions,
    updateTokenTypeFields,
    toggleTokenForm,
    loadAdminTokens,
    renderAdminTokens,
    createAdminToken,
    revokeAdminToken
  };
})(window.App);
