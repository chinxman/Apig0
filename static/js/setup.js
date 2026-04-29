(function(App) {
  const {state, dom, api} = App;
  const vaultRowTypes = ["file", "hashicorp", "aws", "gcp", "azure", "1password", "cyberark", "http", "exec"];

  function showSetup() {
    App.auth.hideAllOverlays();
    dom.id("setup-overlay").classList.add("visible");
    dom.id("setup-username").focus();
  }

  function renderStatus() {
    if (!state.setupStatus) return;
    const secretsText = state.setupStatus.secrets_backend + (state.setupStatus.secrets_path ? " • " + state.setupStatus.secrets_path : "");
    const usersText = state.setupStatus.users_backend + (state.setupStatus.users_path ? " • " + state.setupStatus.users_path : "");
    const isPersistent = state.setupStatus.persistent_configured;
    const modeText = isPersistent ? "Persistent" : "Temporary (in-memory)";

    const modeEl = dom.id("users-storage-mode");
    if (modeEl) {
      modeEl.textContent = modeText;
      modeEl.className = "status-value" + (isPersistent ? "" : " warn");
    }

    const secretsEl = dom.id("users-secrets-status");
    if (secretsEl) {
      secretsEl.textContent = secretsText;
      secretsEl.className = "status-value" + (state.setupStatus.secrets_mode === "persistent" ? "" : " warn");
    }

    if (dom.id("users-store-status")) dom.text("users-store-status", usersText);
    if (dom.id("users-service-count")) dom.text("users-service-count", (state.setupStatus.service_count || 0) + " configured");
    if (dom.id("users-reset-copy")) dom.text("users-reset-copy", state.setupStatus.reset_behavior || "");
    if (dom.id("users-recovery-copy")) dom.text("users-recovery-copy", state.setupStatus.recovery_hint || "");
    if (dom.id("upgrade-storage-btn-wrap")) dom.show("upgrade-storage-btn-wrap", isPersistent ? "none" : "block");
    if (state.setupStatus.port && dom.id("setup-port")) dom.id("setup-port").value = state.setupStatus.port;
  }

  async function refreshStatus() {
    try {
      const {ok, data} = await api.get("/api/setup/status");
      if (ok) {
        state.setupStatus = data;
        renderStatus();
      }
    } catch {}
  }

  function selectSetupMode(mode) {
    state.setupMode = mode;
    dom.id("mode-btn-temporary").classList.toggle("active", mode === "temporary");
    dom.id("mode-btn-persistent").classList.toggle("active", mode === "persistent");
    dom.show("setup-persistent-section", mode === "persistent" ? "block" : "none");
    if (mode === "persistent" && state.setupServiceSecretMode === "memory") {
      selectServiceSecrets("file");
    }
  }

  function selectVault(type) {
    state.setupVaultType = type;
    dom.qsa('.vault-chip[id^="vchip-"]').forEach((button) => button.classList.remove("active"));
    const chip = dom.id("vchip-" + type);
    if (chip) chip.classList.add("active");
    vaultRowTypes.forEach((rowType) => dom.show("setup-vault-" + rowType + "-row", type === rowType ? "block" : "none"));
  }

  function selectServiceSecrets(mode) {
    state.setupServiceSecretMode = mode;
    dom.qsa('.vault-chip[id^="sschip-"]').forEach((button) => button.classList.remove("active"));
    const chip = dom.id("sschip-" + mode);
    if (chip) chip.classList.add("active");
    dom.show("setup-master-row", mode === "encrypted_file" ? "block" : "none");
  }

  function buildProviderEnv(prefix, vaultType) {
    const providerEnv = {};
    const set = (key, id) => {
      const value = dom.id(prefix + id)?.value?.trim();
      if (value) providerEnv[key] = value;
    };

    if (vaultType === "aws") {
      set("AWS_REGION", "aws-region");
      set("AWS_PROFILE", "aws-profile");
      set("AWS_ACCESS_KEY_ID", "aws-access-key-id");
      set("AWS_SECRET_ACCESS_KEY", "aws-secret-access-key");
      set("AWS_SESSION_TOKEN", "aws-session-token");
    } else if (vaultType === "gcp") {
      set("GCP_PROJECT", "gcp-project");
      set("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", "gcp-creds-file");
    } else if (vaultType === "azure") {
      set("AZURE_VAULT_NAME", "azure-vault-name");
      set("AZURE_TENANT_ID", "azure-tenant-id");
      set("AZURE_CLIENT_ID", "azure-client-id");
      set("AZURE_CLIENT_SECRET", "azure-client-secret");
    } else if (vaultType === "1password") {
      set("OP_VAULT", "op-vault");
      set("OP_SERVICE_ACCOUNT_TOKEN", "op-token");
    }

    return providerEnv;
  }

  function validateVault(type, prefix, isUpgrade) {
    const errId = isUpgrade ? "err-upgrade" : "err-setup";
    if (type === "aws" && !dom.id(prefix + "aws-region")?.value?.trim()) {
      App.auth.setLoginErr(errId, "AWS region is required");
      return false;
    }
    if (type === "gcp" && !dom.id(prefix + "gcp-project")?.value?.trim()) {
      App.auth.setLoginErr(errId, "GCP project is required");
      return false;
    }
    if (type === "azure" && !dom.id(prefix + "azure-vault-name")?.value?.trim()) {
      App.auth.setLoginErr(errId, "Azure vault name is required");
      return false;
    }
    return true;
  }

  async function completeSetup() {
    const username = dom.id("setup-username").value.trim();
    const password = dom.id("setup-password").value;
    if (!username || !password) {
      App.auth.setLoginErr("err-setup", "Admin username and password required");
      return;
    }

    const isTemp = state.setupMode === "temporary";
    const vaultType = isTemp ? "env" : state.setupVaultType;
    const secretMode = isTemp ? "memory" : (state.setupServiceSecretMode === "encrypted_file" ? "encrypted_file" : "file");
    const masterPassword = dom.id("setup-master-password").value;

    if (!isTemp && secretMode === "encrypted_file" && !masterPassword) {
      App.auth.setLoginErr("err-setup", "Master password required for encrypted file secrets");
      return;
    }
    if (!isTemp && !validateVault(vaultType, "setup-", false)) {
      return;
    }

    const payload = {
      mode: state.setupMode,
      port: dom.id("setup-port").value.trim() || "8989",
      admin_username: username,
      admin_password: password,
      user_vault: {
        type: vaultType,
        address: dom.id("setup-vault-address")?.value || "",
        engine: dom.id("setup-vault-engine")?.value || "",
        file_path: dom.id("setup-vault-file-path")?.value || "",
        provider_env: buildProviderEnv("setup-", vaultType)
      },
      service_secrets: {mode: secretMode},
      service_master_password: masterPassword
    };

    App.auth.setLoginBusy("btn-setup", true);
    App.auth.setLoginErr("err-setup", "");
    try {
      const {ok, data} = await api.post("/api/setup/complete", payload);
      if (!ok) {
        if (data && data.status) {
          state.setupStatus = data.status;
          renderStatus();
        }
        App.auth.setLoginErr("err-setup", (data && data.error) || "Bootstrap failed");
        return;
      }
      state.setupStatus = data.status || state.setupStatus;
      renderStatus();
      App.auth.hideAllOverlays();
      App.auth.onLogin(data.user, data.role);
      if (data.otpauth) {
        App.auth.showQR(username, data.otpauth);
      }
    } catch {
      App.auth.setLoginErr("err-setup", "Network error");
    } finally {
      App.auth.setLoginBusy("btn-setup", false);
    }
  }

  function showUpgradeModal() {
    state.upgradeVaultType = "file";
    state.upgradeServiceSecretMode = "file";
    dom.qsa('[id^="up-vchip-"]').forEach((button) => button.classList.remove("active"));
    dom.qsa('[id^="up-sschip-"]').forEach((button) => button.classList.remove("active"));
    dom.id("up-vchip-file").classList.add("active");
    dom.id("up-sschip-file").classList.add("active");
    vaultRowTypes.forEach((rowType) => dom.show("up-vault-" + rowType + "-row", "none"));
    dom.show("up-master-row", "none");
    App.auth.setLoginErr("err-upgrade", "");
    dom.id("upgrade-modal-bg").classList.add("visible");
  }

  function closeUpgradeModal() {
    dom.id("upgrade-modal-bg").classList.remove("visible");
  }

  function selectUpgradeVault(type) {
    state.upgradeVaultType = type;
    dom.qsa('[id^="up-vchip-"]').forEach((button) => button.classList.remove("active"));
    const chip = dom.id("up-vchip-" + type);
    if (chip) chip.classList.add("active");
    vaultRowTypes.forEach((rowType) => dom.show("up-vault-" + rowType + "-row", type === rowType ? "block" : "none"));
  }

  function selectUpgradeServiceSecrets(mode) {
    state.upgradeServiceSecretMode = mode;
    dom.qsa('[id^="up-sschip-"]').forEach((button) => button.classList.remove("active"));
    const chip = dom.id("up-sschip-" + mode);
    if (chip) chip.classList.add("active");
    dom.show("up-master-row", mode === "encrypted_file" ? "block" : "none");
  }

  async function submitUpgrade() {
    const masterPassword = dom.id("up-master-password").value;
    if (state.upgradeServiceSecretMode === "encrypted_file" && !masterPassword) {
      App.auth.setLoginErr("err-upgrade", "Master password required for encrypted file mode");
      return;
    }
    if (!validateVault(state.upgradeVaultType, "up-", true)) {
      return;
    }

    App.auth.setLoginBusy("btn-upgrade", true);
    App.auth.setLoginErr("err-upgrade", "");
    try {
      const {ok, data} = await api.post("/api/admin/settings/storage", {
        user_vault: {
          type: state.upgradeVaultType,
          address: dom.id("up-vault-address")?.value || "",
          engine: dom.id("up-vault-engine")?.value || "",
          provider_env: buildProviderEnv("up-", state.upgradeVaultType)
        },
        service_secrets: {mode: state.upgradeServiceSecretMode},
        master_password: masterPassword
      }, {csrf: true});
      if (!ok) {
        App.auth.setLoginErr("err-upgrade", (data && data.error) || "Upgrade failed");
        return;
      }
      state.setupStatus = data.status || state.setupStatus;
      renderStatus();
      closeUpgradeModal();
      App.users.setNotice("Storage upgraded to persistent mode", "ok");
    } catch {
      App.auth.setLoginErr("err-upgrade", "Network error");
    } finally {
      App.auth.setLoginBusy("btn-upgrade", false);
    }
  }

  async function adminResetSetup() {
    const phrase = window.prompt("This will wipe setup, users, services, secrets, and active sessions.\n\nType RESET to continue.");
    if (phrase !== "RESET") {
      if (phrase !== null) App.users.setNotice("Reset cancelled", "err");
      return;
    }

    App.auth.setLoginBusy("btn-admin-reset", true);
    try {
      const {ok, data} = await api.post("/api/admin/setup/reset", undefined, {csrf: true});
      if (!ok) {
        App.users.setNotice((data && data.error) || "Reset failed", "err");
        return;
      }
      state.setupStatus = data.status || null;
      if (state.evtSource) {
        state.evtSource.close();
        state.evtSource = null;
      }
      if (state.reconnectTimer) {
        clearTimeout(state.reconnectTimer);
        state.reconnectTimer = null;
      }
      state.currentUser = null;
      state.currentRole = null;
      state.pendingChallenge = null;
      dom.hide("app-shell");
      dom.id("login-password").value = "";
      dom.id("login-totp").value = "";
      dom.id("bootstrap-password").value = "";
      renderStatus();
      showSetup();
    } catch {
      App.users.setNotice("Network error", "err");
    } finally {
      App.auth.setLoginBusy("btn-admin-reset", false);
    }
  }

  App.actions["select-setup-mode"] = (el) => selectSetupMode(el.dataset.mode);
  App.actions["select-vault"] = (el) => selectVault(el.dataset.vault);
  App.actions["select-service-secrets"] = (el) => selectServiceSecrets(el.dataset.secretMode);
  App.actions["complete-setup"] = completeSetup;
  App.actions["show-upgrade-modal"] = showUpgradeModal;
  App.actions["close-upgrade-modal"] = closeUpgradeModal;
  App.actions["select-upgrade-vault"] = (el) => selectUpgradeVault(el.dataset.vault);
  App.actions["select-upgrade-service-secrets"] = (el) => selectUpgradeServiceSecrets(el.dataset.secretMode);
  App.actions["submit-upgrade"] = submitUpgrade;
  App.actions["admin-reset-setup"] = adminResetSetup;

  App.setup = {
    showSetup,
    renderStatus,
    refreshStatus,
    selectSetupMode,
    selectVault,
    selectServiceSecrets,
    completeSetup,
    showUpgradeModal,
    closeUpgradeModal,
    selectUpgradeVault,
    selectUpgradeServiceSecrets,
    submitUpgrade,
    adminResetSetup
  };
})(window.App);
