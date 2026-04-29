(function(App) {
  const {state, dom, api, util} = App;

  function setNotice(msg, type) {
    util.setNotice("users-notice", msg, type);
  }

  function updateCreateServiceHint() {
    const hint = dom.id("create-service-hint");
    if (!hint) return;
    const role = dom.id("new-role")?.value;
    if (role === "admin") {
      hint.textContent = "Admin accounts always have full access.";
      return;
    }
    const selected = util.getCheckedValues(dom.qsa('#create-service-checks input[type="checkbox"]'));
    hint.textContent = selected.length
      ? "Selected APIs will be available to this user."
      : "No APIs selected. This user will be blocked from all proxied APIs until access is assigned.";
  }

  function renderCreateServiceChecks() {
    const wrap = dom.id("create-service-checks");
    if (!wrap) return;
    wrap.innerHTML = "";
    for (const service of state.serviceCatalog) {
      const label = document.createElement("label");
      label.className = "service-check";
      label.innerHTML = '<input type="checkbox" value="' + util.escAttr(service) + '" checked> <span>' + util.escHtml(service) + "</span>";
      wrap.appendChild(label);
    }
    updateCreateServiceHint();
  }

  function renderServiceControls(username, allowed, isAdmin, configured) {
    if (isAdmin) return '<span class="service-muted">Full access</span>';
    const selected = new Set(configured ? allowed : state.serviceCatalog);
    return '<div class="service-checks">' + state.serviceCatalog.map((service) =>
      '<label class="service-check"><input type="checkbox" data-service-check="1" data-user="' + util.escAttr(username) + '" value="' + util.escAttr(service) + '"' +
      (selected.has(service) ? " checked" : "") + "> <span>" + util.escHtml(service) + "</span></label>"
    ).join("") + "</div>";
  }

  function renderUsers(users) {
    const tbody = dom.id("users-tbody");
    tbody.innerHTML = "";
    if (!users.length) {
      tbody.innerHTML = '<tr><td colspan="5" style="color:var(--text-dim);padding:16px;text-align:center">No users found</td></tr>';
      return;
    }
    for (const user of users) {
      const tr = document.createElement("tr");
      const created = user.created_at ? new Date(user.created_at).toLocaleDateString() : "-";
      const allowed = Array.isArray(user.allowed_services) ? user.allowed_services : [];
      const isAdmin = user.role === "admin";
      const protectedAdmin = !!user.protected_admin;
      const configured = !!user.service_access_configured;
      tr.innerHTML =
        "<td>" + util.escHtml(user.username) + "</td>" +
        '<td><span class="role-pill role-' + util.escAttr(user.role) + '">' + util.escHtml(user.role) + "</span></td>" +
        '<td class="access-cell">' + renderServiceControls(user.username, allowed, isAdmin, configured) + "</td>" +
        '<td style="color:var(--text-dim)">' + util.escHtml(created) + "</td>" +
        '<td><div class="actions">' +
        (isAdmin ? "" : '<button class="btn-primary" data-action="save-user-access" data-user="' + util.escAttr(user.username) + '" style="padding:4px 10px">Save Access</button>') +
        (isAdmin ? "" : '<button class="btn-ghost" data-action="edit-user-policies" data-user="' + util.escAttr(user.username) + '">Policies</button>') +
        '<button class="btn-ghost" data-action="reset-user-totp" data-user="' + util.escAttr(user.username) + '">Reset TOTP</button>' +
        (protectedAdmin
          ? '<span class="service-muted">Reset only</span>'
          : '<button class="btn-danger" data-action="delete-user" data-user="' + util.escAttr(user.username) + '">Delete</button>') +
        "</div></td>";
      tbody.appendChild(tr);
    }
  }

  async function loadUsers() {
    try {
      const {ok, data} = await api.get("/api/admin/users");
      if (!ok) return;
      state.serviceCatalog = Array.isArray(data.service_catalog) ? data.service_catalog : state.serviceCatalog;
      renderCreateServiceChecks();
      renderUsers(data.users || []);
    } catch {}
  }

  function toggleCreateForm() {
    const form = dom.id("create-form");
    form.classList.toggle("visible");
    if (form.classList.contains("visible")) {
      dom.id("new-username").focus();
      updateCreateServiceHint();
    }
  }

  async function createUser() {
    const username = dom.id("new-username").value.trim();
    const password = dom.id("new-password").value;
    const role = dom.id("new-role").value;
    const allowedServices = util.getCheckedValues(dom.qsa('#create-service-checks input[type="checkbox"]'));
    if (!username || !password) {
      setNotice("Username and password required", "err");
      return;
    }
    try {
      const {ok, data} = await api.post("/api/admin/users", {
        username,
        password,
        role,
        allowed_services: role === "admin" ? state.serviceCatalog : allowedServices
      }, {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed", "err");
        return;
      }
      dom.id("new-username").value = "";
      dom.id("new-password").value = "";
      dom.qsa('#create-service-checks input[type="checkbox"]').forEach((cb) => { cb.checked = true; });
      updateCreateServiceHint();
      dom.id("create-form").classList.remove("visible");
      setNotice('User "' + username + '" created', "ok");
      loadUsers();
      if (data.otpauth) App.auth.showQR(username, data.otpauth);
    } catch {
      setNotice("Network error", "err");
    }
  }

  async function saveUserAccess(username) {
    const boxes = dom.qsa('input[data-service-check="1"]').filter((box) => box.dataset.user === username);
    const allowedServices = util.getCheckedValues(boxes);
    try {
      const {ok, data} = await api.put("/api/admin/users/" + encodeURIComponent(username) + "/access", {allowed_services: allowedServices}, {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed", "err");
        return;
      }
      setNotice('API access updated for "' + username + '"', "ok");
      if (username === state.currentUser) {
        state.availableServices = allowedServices;
        App.auth.renderPortalServices();
      }
    } catch {
      setNotice("Network error", "err");
    }
  }

  async function editUserPolicies(username) {
    try {
      const {ok, data} = await api.get("/api/admin/users/" + encodeURIComponent(username) + "/policies", {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed to load policies", "err");
        return;
      }
      const draft = window.prompt(
        'Edit route policies as JSON. Example:\n[{"service":"orders","path_prefix":"/read","methods":["GET"]}]',
        JSON.stringify(data.policies || [], null, 2)
      );
      if (draft === null) return;
      let parsed;
      try {
        parsed = draft.trim() ? JSON.parse(draft) : [];
      } catch {
        setNotice("Policy JSON is invalid", "err");
        return;
      }
      const save = await api.put("/api/admin/users/" + encodeURIComponent(username) + "/policies", {policies: parsed}, {csrf: true});
      if (!save.ok) {
        setNotice((save.data && save.data.error) || "Failed to save policies", "err");
        return;
      }
      setNotice('Route policies updated for "' + username + '"', "ok");
    } catch {
      setNotice("Network error", "err");
    }
  }

  async function deleteUser(username) {
    if (!window.confirm('Delete user "' + username + '"? This cannot be undone.')) return;
    try {
      const {ok, data} = await api.delete("/api/admin/users/" + encodeURIComponent(username), {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed", "err");
        return;
      }
      setNotice('User "' + username + '" deleted', "ok");
      loadUsers();
    } catch {
      setNotice("Network error", "err");
    }
  }

  async function resetTOTP(username) {
    if (!window.confirm('Reset TOTP for "' + username + '"? They will need to re-scan a new QR code.')) return;
    try {
      const {ok, data} = await api.post("/api/admin/users/" + encodeURIComponent(username) + "/reset", undefined, {csrf: true});
      if (!ok) {
        setNotice((data && data.error) || "Failed", "err");
        return;
      }
      setNotice('TOTP rotated for "' + username + '"', "ok");
      if (data.otpauth) App.auth.showQR(username, data.otpauth);
    } catch {
      setNotice("Network error", "err");
    }
  }

  App.actions["toggle-create-user-form"] = toggleCreateForm;
  App.actions["create-user"] = createUser;
  App.actions["save-user-access"] = (el) => saveUserAccess(el.dataset.user);
  App.actions["edit-user-policies"] = (el) => editUserPolicies(el.dataset.user);
  App.actions["delete-user"] = (el) => deleteUser(el.dataset.user);
  App.actions["reset-user-totp"] = (el) => resetTOTP(el.dataset.user);

  App.users = {
    loadUsers,
    renderUsers,
    renderCreateServiceChecks,
    renderServiceControls,
    toggleCreateForm,
    createUser,
    saveUserAccess,
    editUserPolicies,
    deleteUser,
    resetTOTP,
    updateCreateServiceHint,
    setNotice
  };
})(window.App);
