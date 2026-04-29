(function(App) {
  const {dom, actions} = App;

  function bindDelegatedActions() {
    document.addEventListener("click", (event) => {
      const target = event.target.closest("[data-action]");
      if (!target) return;
      const handler = actions[target.dataset.action];
      if (!handler) return;
      event.preventDefault();
      handler(target, event);
    });

    document.addEventListener("change", (event) => {
      const target = event.target.closest("[data-change-action]");
      if (!target) return;
      const handler = actions[target.dataset.changeAction];
      if (!handler) return;
      handler(target, event);
    });
  }

  function bindKeyShortcuts() {
    dom.id("login-password").addEventListener("keydown", (e) => {
      if (e.key === "Enter") actions["submit-password"]();
    });
    dom.id("login-totp").addEventListener("keydown", (e) => {
      if (e.key === "Enter") actions["submit-totp"]();
    });
    dom.id("login-username").addEventListener("keydown", (e) => {
      if (e.key === "Enter") dom.id("login-password").focus();
    });
    dom.id("bootstrap-username").addEventListener("keydown", (e) => {
      if (e.key === "Enter") dom.id("bootstrap-password").focus();
    });
    dom.id("bootstrap-password").addEventListener("keydown", (e) => {
      if (e.key === "Enter") actions["submit-bootstrap"]();
    });
  }

  function bindStaticChangeListeners() {
    dom.id("new-role").addEventListener("change", App.users.updateCreateServiceHint);
    document.addEventListener("change", (event) => {
      if (event.target && event.target.matches('#create-service-checks input[type="checkbox"]')) {
        App.users.updateCreateServiceHint();
      }
    });
  }

  function init() {
    dom.text("gateway-url", App.state.gatewayOrigin);
    bindDelegatedActions();
    bindKeyShortcuts();
    bindStaticChangeListeners();
    try {
      App.portal.init();
    } catch (err) {
      console.error("Portal init error:", err);
    }
    App.monitor.init();
    App.auth.init();
  }

  init();
})(window.App);
