(function(App) {
  const {state, dom} = App;

  function switchPage(name) {
    if (name !== "portal" && state.currentRole !== "admin") return;
    dom.qsa(".nav-tab").forEach((tab) => tab.classList.remove("active"));
    dom.qsa(".page").forEach((page) => page.classList.remove("active"));
    dom.id("nav-" + name).classList.add("active");
    dom.id("page-" + name).classList.add("active");

    if (name === "users") {
      App.setup.refreshStatus();
      App.services.loadAdminServices();
      App.users.loadUsers();
      App.tokens.loadAdminTokens();
    }
    if (name === "ratelimits") {
      App.rateLimits.loadRateLimits();
    }
    if (name === "monitor") {
      App.monitor.closeSvcPanel();
      App.auth.loadSessionInfo();
      App.monitor.loadAuditTrail();
    }
  }

  App.actions["switch-page"] = (el) => switchPage(el.dataset.page);

  App.navigation = {switchPage};
})(window.App);
