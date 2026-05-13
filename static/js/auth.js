(function(App) {
  const {state, dom, api, util} = App;

  function setLoginErr(id, msg) {
    const el = dom.id(id);
    el.textContent = msg;
    el.classList.toggle("visible", msg !== "");
  }

  function setLoginBusy(id, busy) {
    dom.id(id).disabled = busy;
  }

  function hideAllOverlays() {
    dom.id("setup-overlay").classList.remove("visible");
    dom.id("login-overlay").classList.remove("visible");
    dom.id("bootstrap-overlay").classList.remove("visible");
    dom.hide("app-shell");
  }

  function showLogin() {
    state.pendingChallenge = null;
    hideAllOverlays();
    dom.id("login-username").value = "";
    dom.id("login-password").value = "";
    dom.id("login-totp").value = "";
    setLoginErr("err-password", "");
    setLoginErr("err-totp", "");
    setLoginBusy("btn-password", false);
    setLoginBusy("btn-totp", false);
    dom.id("step-totp").classList.remove("active");
    dom.id("step-password").classList.add("active");
    dom.id("login-overlay").classList.add("visible");
    dom.id("login-username").focus();
  }

  function showBootstrap() {
    hideAllOverlays();
    dom.id("bootstrap-overlay").classList.add("visible");
    dom.id("bootstrap-username").focus();
  }

  async function submitBootstrap() {
    const username = dom.id("bootstrap-username").value.trim();
    const password = dom.id("bootstrap-password").value;
    if (!username || !password) {
      setLoginErr("err-bootstrap", "Username and password required");
      return;
    }

    setLoginBusy("btn-bootstrap", true);
    setLoginErr("err-bootstrap", "");
    try {
      const {ok, data} = await api.post("/api/setup/bootstrap-admin", {username, password});
      if (!ok) {
        setLoginErr("err-bootstrap", (data && data.error) || "Bootstrap failed");
        return;
      }
      state.setupStatus = data.status || state.setupStatus;
      App.setup.renderStatus();
      hideAllOverlays();
      onLogin(data.user, data.role);
      if (data.otpauth) {
        showQR(username, data.otpauth);
      }
    } catch {
      setLoginErr("err-bootstrap", "Network error");
    } finally {
      setLoginBusy("btn-bootstrap", false);
    }
  }

  async function loadSessionInfo() {
    try {
      const {ok, data} = await api.get("/api/user/info");
      if (!ok) return;
      state.serviceCatalog = Array.isArray(data.service_catalog) ? data.service_catalog : [];
      state.availableServices = Array.isArray(data.available_services) ? data.available_services : state.serviceCatalog.slice();
      state.portalAssignedTokenPrefix = data.assigned_token_prefix || "";
      state.portalHasAssignedToken = !!data.has_assigned_token;
      state.portalAssignedTokenType = data.assigned_token_type || "standard";
      state.portalAssignedOpenAIService = data.assigned_openai_service || "";
      state.portalAssignedBackendLabel = data.assigned_backend_label || "";
      state.portalAssignedAllowedModels = Array.isArray(data.assigned_allowed_models) ? data.assigned_allowed_models : [];
      state.portalAssignedAllowedProviders = Array.isArray(data.assigned_allowed_providers) ? data.assigned_allowed_providers : [];
      state.portalToken = "";
      await loadPendingKeyDeliveries();
      App.users.renderCreateServiceChecks();
      App.tokens.renderTokenServiceChecks();
      App.portal.renderServiceCards();
      App.portal.render();
      App.monitor.renderTestButtons();
    } catch {}
  }

  async function loadPendingKeyDeliveries() {
    try {
      const {ok, data} = await api.get("/api/user/pending-keys");
      if (!ok) return;
      state.portalPendingDeliveries = Array.isArray(data.deliveries) ? data.deliveries : [];
    } catch {}
  }

  function onLogin(user, role) {
    state.currentUser = user;
    state.currentRole = role || "user";
    hideAllOverlays();
    dom.show("app-shell");
    dom.text("hdr-user", user);
    dom.text("sess-status", "Active");
    const roleBadge = dom.id("hdr-role");
    roleBadge.textContent = state.currentRole;
    roleBadge.className = "role-badge" + (state.currentRole === "admin" ? " admin" : "");

    dom.qsa(".nav-tab.admin-only").forEach((tab) => {
      tab.classList.toggle("visible", state.currentRole === "admin");
    });
    if (state.currentRole === "admin") {
      dom.show("conn-status", "flex");
    } else {
      dom.hide("conn-status");
    }

    loadSessionInfo();
    if (state.currentRole === "admin") {
      App.monitor.connectSSE();
    }
    App.navigation.switchPage("portal");
  }

  function backToPassword() {
    state.pendingChallenge = null;
    dom.id("step-totp").classList.remove("active");
    dom.id("step-password").classList.add("active");
    setLoginErr("err-totp", "");
    dom.id("login-totp").value = "";
    dom.id("login-password").focus();
  }

  async function submitPassword() {
    const username = dom.id("login-username").value.trim();
    const password = dom.id("login-password").value;
    if (!username || !password) {
      setLoginErr("err-password", "Username and password required");
      return;
    }

    setLoginBusy("btn-password", true);
    setLoginErr("err-password", "");
    try {
      const {ok, data} = await api.post("/auth/login", {username, password});
      if (!ok) {
        setLoginErr("err-password", (data && data.error) || "Login failed");
        return;
      }
      state.pendingChallenge = data.challenge;
      dom.id("step-password").classList.remove("active");
      dom.id("step-totp").classList.add("active");
      dom.id("login-totp").focus();
    } catch {
      setLoginErr("err-password", "Network error");
    } finally {
      setLoginBusy("btn-password", false);
    }
  }

  async function submitTOTP() {
    const code = dom.id("login-totp").value.trim();
    if (!code) {
      setLoginErr("err-totp", "Enter your 6-digit code");
      return;
    }

    setLoginBusy("btn-totp", true);
    setLoginErr("err-totp", "");
    try {
      const {ok, data} = await api.post("/auth/verify", {challenge: state.pendingChallenge, code});
      if (!ok) {
        setLoginErr("err-totp", (data && data.error) || "Verification failed");
        return;
      }
      onLogin(data.user, data.role);
    } catch {
      setLoginErr("err-totp", "Network error");
    } finally {
      setLoginBusy("btn-totp", false);
    }
  }

  async function logout() {
    await api.post("/auth/logout", undefined, {csrf: true});
    if (state.evtSource) {
      state.evtSource.close();
      state.evtSource = null;
    }
    state.currentUser = null;
    state.currentRole = null;
    state.portalAssignedTokenPrefix = "";
    state.portalHasAssignedToken = false;
    state.portalAssignedTokenType = "standard";
    state.portalAssignedOpenAIService = "";
    state.portalAssignedBackendLabel = "";
    state.portalAssignedAllowedModels = [];
    state.portalAssignedAllowedProviders = [];
    state.portalPendingDeliveries = [];
    state.portalToken = "";
    App.monitor.setConnected(false);
    showLogin();
  }

  function showQR(username, otpauth) {
    dom.text("qr-username-msg", username + " — scan with an authenticator app.");
    dom.text("qr-uri", otpauth);
    const canvas = dom.id("qr-canvas");
    canvas.innerHTML = "";
    if (typeof QRCode !== "undefined") {
      new QRCode(canvas, {
        text: otpauth,
        width: 200,
        height: 200,
        colorDark: "#c0caf5",
        colorLight: "#16161e"
      });
    }
    dom.id("qr-modal-bg").classList.add("visible");
  }

  function closeQR() {
    dom.id("qr-modal-bg").classList.remove("visible");
  }

  async function init() {
    try {
      const {ok, data} = await api.get("/api/setup/status");
      if (!ok) {
        showLogin();
        return;
      }
      state.setupStatus = data;
      App.setup.renderStatus();
    } catch {
      showLogin();
      return;
    }

    if (state.setupStatus.setup_required) {
      App.setup.showSetup();
      return;
    }

    if (state.setupStatus.bootstrap_required && !state.setupStatus.has_admin) {
      showBootstrap();
      return;
    }

    try {
      const {ok, data} = await api.get("/api/user/info");
      if (!ok) {
        showLogin();
        return;
      }
      onLogin(data.user, data.role);
    } catch {
      showLogin();
    }
  }

  App.actions["submit-password"] = submitPassword;
  App.actions["submit-totp"] = submitTOTP;
  App.actions["back-to-password"] = backToPassword;
  App.actions["submit-bootstrap"] = submitBootstrap;
  App.actions["logout"] = logout;
  App.actions["close-qr"] = closeQR;

  App.auth = {
    init,
    hideAllOverlays,
    showLogin,
    showBootstrap,
    submitBootstrap,
    loadSessionInfo,
    loadPendingKeyDeliveries,
    onLogin,
    showQR,
    closeQR,
    setLoginErr,
    setLoginBusy,
    backToPassword
  };
})(window.App);
