(function(App) {
  const {state, dom, api, util} = App;

  function setNotice(msg, type) {
    util.setNotice("users-notice", msg, type);
  }

  function renderServiceSecretStorage() {
    const el = dom.id("service-secret-storage-copy");
    if (!el) return;
    if (!state.serviceSecretStorage) {
      el.textContent = "";
      return;
    }
    const parts = ["Service key storage: " + (state.serviceSecretStorage.mode || "memory")];
    if (state.serviceSecretStorage.file_path) parts.push(state.serviceSecretStorage.file_path);
    if (state.serviceSecretStorage.locked) parts.push("locked");
    el.textContent = parts.join(" • ");
  }

  async function loadAdminServices() {
    try {
      const {ok, data} = await api.get("/api/admin/services", {csrf: true});
      if (!ok) return;
      state.adminServices = Array.isArray(data.services) ? data.services : [];
      state.serviceSecretStorage = data.service_secret_storage || null;
      state.serviceSecretMeta = data.service_secret_meta || {};
      if (Array.isArray(data.service_catalog)) {
        state.serviceCatalog = data.service_catalog;
      }
      state.openAIServiceCatalog = state.adminServices.filter((service) => service.openai_compatible).map((service) => service.name).sort();
      renderServiceSecretStorage();
      renderServicesAdmin(state.adminServices);
      App.users.renderCreateServiceChecks();
      App.tokens.renderTokenServiceChecks();
    } catch {}
  }

  function renderServicesAdmin(services) {
    const tbody = dom.id("services-tbody");
    if (!tbody) return;
    tbody.innerHTML = "";
    if (!services.length) {
      tbody.innerHTML = '<tr><td colspan="7" style="color:var(--text-dim);padding:16px;text-align:center">No API endpoints configured yet</td></tr>';
      return;
    }
    for (const svc of services.slice().sort((a, b) => a.name.localeCompare(b.name))) {
      const meta = state.serviceSecretMeta[svc.name] || {};
      const secretCell = svc.has_secret
        ? '<span style="color:var(--green)">saved</span><div class="service-muted" style="margin-top:4px">test: ' + util.escHtml(meta.last_test_status ? String(meta.last_test_status) : "never") + "</div>"
        : '<span class="service-muted">none</span>';
      const tr = document.createElement("tr");
      tr.innerHTML =
        "<td><strong style=\"color:var(--purple)\">" + util.escHtml(svc.name) + "</strong></td>" +
        '<td style="color:var(--text-dim);max-width:360px;word-break:break-all">' + util.escHtml(svc.base_url || "-") + "</td>" +
        "<td>" + util.escHtml(svc.auth_type || "none") + (svc.openai_compatible ? '<div class="service-muted" style="margin-top:4px">openai • ' + util.escHtml(svc.provider || "custom") + "</div>" : "") + "</td>" +
        "<td>" + util.escHtml(String(svc.retry_count || 0)) + " • " + util.escHtml(String(svc.timeout_ms || 10000)) + "ms</td>" +
        "<td>" + secretCell + "</td>" +
        "<td>" + (svc.enabled
          ? '<span class="role-pill role-user" style="background:rgba(158,206,106,0.12);color:var(--green)">enabled</span>'
          : '<span class="role-pill" style="background:rgba(247,118,142,0.12);color:var(--red)">disabled</span>') + "</td>" +
        '<td><div class="actions">' +
        '<button class="btn-primary" data-action="test-service-auth" data-service="' + util.escAttr(svc.name) + '" style="padding:4px 10px">Test</button>' +
        '<button class="btn-ghost" data-action="edit-service" data-service="' + util.escAttr(svc.name) + '">Edit</button>' +
        '<button class="btn-danger" data-action="delete-service" data-service="' + util.escAttr(svc.name) + '">Delete</button>' +
        "</div></td>";
      tbody.appendChild(tr);
    }
  }

  function updateServiceAuthFields() {
    const authType = dom.id("service-auth-type").value;
    dom.show("service-header-row", (authType === "x-api-key" || authType === "custom-header") ? "block" : "none");
    dom.show("service-basic-row", authType === "basic" ? "block" : "none");
    dom.show("service-secret-row", authType === "none" ? "none" : "block");
    dom.show("service-secret-expiry-row", authType === "none" ? "none" : "block");
    dom.show("service-secret-notes-row", authType === "none" ? "none" : "block");

    const note = dom.id("service-form-note");
    if (authType === "x-api-key") {
      dom.id("service-header-name").placeholder = "X-API-Key";
      note.textContent = "x-api-key and custom-header modes can store a saved key for this endpoint.";
    } else if (authType === "custom-header") {
      dom.id("service-header-name").placeholder = "Authorization";
      note.textContent = "Custom header mode forwards the saved value in the header you choose.";
    } else if (authType === "basic") {
      note.textContent = "Basic auth uses the saved secret together with the Basic Username field.";
    } else if (authType === "bearer") {
      note.textContent = "Bearer mode sends the saved secret as Authorization: Bearer <token>.";
    } else {
      note.textContent = 'Auth type "none" skips saved keys and forwards requests directly.';
    }
  }

  function toggleServiceSecretClear() {
    const checked = dom.id("service-clear-secret").checked;
    const input = dom.id("service-secret");
    input.disabled = checked;
    if (checked) input.value = "";
  }

  function clearServiceForm() {
    state.editingServiceName = null;
    dom.text("service-form-title", "New API Endpoint");
    dom.id("service-name").value = "";
    dom.id("service-name").disabled = false;
    dom.id("service-base-url").value = "";
    dom.id("service-auth-type").value = "none";
    dom.id("service-header-name").value = "";
    dom.id("service-basic-username").value = "";
    dom.id("service-provider").value = "";
    dom.id("service-openai-compatible").checked = false;
    dom.id("service-secret").value = "";
    dom.id("service-secret").disabled = false;
    dom.id("service-secret-expires").value = "";
    dom.id("service-secret-notes").value = "";
    dom.id("service-clear-secret").checked = false;
    dom.show("service-clear-secret-wrap", "none");
    dom.id("service-timeout-ms").value = "10000";
    dom.id("service-retry-count").value = "0";
    dom.id("service-enabled").checked = true;
    updateServiceAuthFields();
  }

  function toggleServiceForm() {
    const form = dom.id("service-form");
    if (form.classList.contains("visible") && !state.editingServiceName) {
      form.classList.remove("visible");
      clearServiceForm();
      return;
    }
    clearServiceForm();
    form.classList.add("visible");
    dom.id("service-name").focus();
  }

  function cancelServiceForm() {
    clearServiceForm();
    dom.id("service-form").classList.remove("visible");
  }

  function editServiceAdmin(name) {
    const svc = state.adminServices.find((service) => service.name === name);
    if (!svc) return;
    state.editingServiceName = svc.name;
    dom.text("service-form-title", "Edit API Endpoint");
    dom.id("service-name").value = svc.name;
    dom.id("service-name").disabled = true;
    dom.id("service-base-url").value = svc.base_url || "";
    dom.id("service-auth-type").value = svc.auth_type || "none";
    dom.id("service-header-name").value = svc.header_name || "";
    dom.id("service-basic-username").value = svc.basic_username || "";
    dom.id("service-provider").value = svc.provider || "";
    dom.id("service-openai-compatible").checked = !!svc.openai_compatible;
    dom.id("service-timeout-ms").value = svc.timeout_ms || 10000;
    dom.id("service-retry-count").value = svc.retry_count || 0;
    dom.id("service-secret").value = "";
    dom.id("service-secret").disabled = false;
    const meta = state.serviceSecretMeta[svc.name] || {};
    dom.id("service-secret-expires").value = util.isoInputValue(meta.expires_at);
    dom.id("service-secret-notes").value = meta.notes || "";
    dom.id("service-clear-secret").checked = false;
    dom.show("service-clear-secret-wrap", svc.has_secret ? "flex" : "none");
    dom.id("service-enabled").checked = !!svc.enabled;
    updateServiceAuthFields();
    dom.id("service-form").classList.add("visible");
    dom.id("service-base-url").focus();
  }

  async function saveServiceAdmin() {
    const name = dom.id("service-name").value.trim();
    const payload = {
      name,
      base_url: dom.id("service-base-url").value.trim(),
      auth_type: dom.id("service-auth-type").value,
      header_name: dom.id("service-header-name").value.trim(),
      basic_username: dom.id("service-basic-username").value.trim(),
      provider: dom.id("service-provider").value,
      openai_compatible: dom.id("service-openai-compatible").checked,
      timeout_ms: parseInt(dom.id("service-timeout-ms").value, 10) || 10000,
      retry_count: parseInt(dom.id("service-retry-count").value, 10) || 0,
      secret: dom.id("service-secret").value,
      secret_notes: dom.id("service-secret-notes").value.trim(),
      secret_expires_at: dom.id("service-secret-expires").value ? new Date(dom.id("service-secret-expires").value).toISOString() : "",
      clear_secret: dom.id("service-clear-secret").checked,
      enabled: dom.id("service-enabled").checked
    };

    if (!name || !payload.base_url) {
      setNotice("API name and base URL are required", "err");
      return;
    }

    const isEdit = !!state.editingServiceName;
    const target = isEdit ? state.editingServiceName : name;
    try {
      const result = isEdit
        ? await api.put("/api/admin/services/" + encodeURIComponent(target), payload, {csrf: true})
        : await api.post("/api/admin/services", payload, {csrf: true});
      if (!result.ok) {
        setNotice((result.data && result.data.error) || "Failed to save API endpoint", "err");
        return;
      }
      cancelServiceForm();
      App.monitor.closeSvcPanel();
      setNotice('API endpoint "' + target + '" saved', "ok");
      await loadAdminServices();
      await App.users.loadUsers();
      await App.auth.loadSessionInfo();
    } catch {
      setNotice("Network error", "err");
    }
  }

  async function testServiceAuthAdmin(name) {
    try {
      const {ok, data} = await api.post("/api/admin/services/" + encodeURIComponent(name) + "/test-auth", {path: ""}, {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Service auth test failed", "err");
        return;
      }
      setNotice('Auth test for "' + name + '" returned ' + data.status, data.ok ? "ok" : "err");
      await loadAdminServices();
    } catch {
      setNotice("Network error", "err");
    }
  }

  async function deleteServiceAdmin(name) {
    if (!window.confirm('Delete API endpoint "' + name + '"? This also removes its saved key and clears it from user access lists.')) {
      return;
    }
    try {
      const {ok, data} = await api.delete("/api/admin/services/" + encodeURIComponent(name), {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed to delete API endpoint", "err");
        return;
      }
      if (state.editingServiceName === name) {
        cancelServiceForm();
      }
      App.monitor.closeSvcPanel();
      setNotice('API endpoint "' + name + '" deleted', "ok");
      await loadAdminServices();
      await App.users.loadUsers();
      await App.auth.loadSessionInfo();
    } catch {
      setNotice("Network error", "err");
    }
  }

  App.actions["toggle-service-form"] = toggleServiceForm;
  App.actions["cancel-service-form"] = cancelServiceForm;
  App.actions["save-service"] = saveServiceAdmin;
  App.actions["edit-service"] = (el) => editServiceAdmin(el.dataset.service);
  App.actions["delete-service"] = (el) => deleteServiceAdmin(el.dataset.service);
  App.actions["test-service-auth"] = (el) => testServiceAuthAdmin(el.dataset.service);
  App.actions["update-service-auth-fields"] = updateServiceAuthFields;
  App.actions["toggle-service-secret-clear"] = toggleServiceSecretClear;

  App.services = {
    loadAdminServices,
    renderServicesAdmin,
    renderServiceSecretStorage,
    updateServiceAuthFields,
    toggleServiceSecretClear,
    clearServiceForm,
    toggleServiceForm,
    cancelServiceForm,
    editServiceAdmin,
    saveServiceAdmin,
    testServiceAuthAdmin,
    deleteServiceAdmin
  };
})(window.App);
