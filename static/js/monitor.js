(function(App) {
  const {state, dom, api, util} = App;

  function setConnected(ok) {
    const dot = dom.id("conn-dot");
    const txt = dom.id("conn-text");
    if (!dot || !txt) return;
    if (ok) {
      dot.classList.add("live");
      txt.textContent = "Live";
      txt.style.color = "var(--green)";
    } else {
      dot.classList.remove("live");
      txt.textContent = "Disconnected";
      txt.style.color = "var(--red)";
    }
  }

  function renderServiceCards() {
    const container = dom.id("services-grid");
    container.innerHTML = "";
    for (const name of Object.keys(state.svcData).sort()) {
      const svc = state.svcData[name];
      const card = document.createElement("div");
      card.className = "svc-card";
      card.id = "svc-" + name;
      let dotClass = "idle";
      if (svc.requests > 0) {
        dotClass = (svc.last_status >= 200 && svc.last_status < 400) ? "ok" : "err";
      }
      const errRate = svc.requests > 0 ? ((svc.errors / svc.requests) * 100).toFixed(1) : "0.0";
      const avgLat = svc.avg_latency_ms ? svc.avg_latency_ms.toFixed(1) : "--";
      card.innerHTML =
        '<div class="svc-header"><span class="svc-name">' + util.escHtml(name) + '</span><div class="svc-dot ' + dotClass + '" id="svc-dot-' + util.escAttr(name) + '"></div></div>' +
        '<div class="svc-backend">' + util.escHtml(svc.backend || "") + "</div>" +
        '<div class="svc-stats">' +
        '<div><div class="svc-stat-val" id="svc-req-' + util.escAttr(name) + '">' + util.fmtNum(svc.requests) + '</div><div class="svc-stat-label">Requests</div></div>' +
        '<div><div class="svc-stat-val" id="svc-err-' + util.escAttr(name) + '" style="color:' + (svc.errors > 0 ? "var(--red)" : "var(--text)") + '">' + util.fmtNum(svc.errors) + '</div><div class="svc-stat-label">Errors</div></div>' +
        '<div><div class="svc-stat-val ' + util.latencyClass(svc.avg_latency_ms) + '" id="svc-lat-' + util.escAttr(name) + '">' + avgLat + '</div><div class="svc-stat-label">Avg ms</div></div>' +
        '<div><div class="svc-stat-val" id="svc-rate-' + util.escAttr(name) + '" style="color:' + (parseFloat(errRate) > 5 ? "var(--red)" : "var(--green)") + '">' + errRate + '%</div><div class="svc-stat-label">Err rate</div></div>' +
        "</div>";
      container.appendChild(card);
    }
  }

  function updateCounters() {
    dom.text("total-req", util.fmtNum(state.totalReq));
    dom.text("total-err", util.fmtNum(state.totalErr));
  }

  function updateServiceFromEvent(evt) {
    const svc = evt.service;
    if (!svc) return;
    if (!state.svcData[svc]) {
      state.svcData[svc] = {name: svc, backend: "", requests: 0, errors: 0, avg_latency_ms: 0, last_status: 0, last_seen: 0};
      renderServiceCards();
      return;
    }

    const data = state.svcData[svc];
    const totalLat = data.avg_latency_ms * data.requests;
    data.requests += 1;
    if (evt.status >= 400) data.errors += 1;
    data.avg_latency_ms = (totalLat + evt.latency_ms) / data.requests;
    data.last_status = evt.status;
    data.last_seen = evt.ts;

    const reqEl = dom.id("svc-req-" + svc);
    if (!reqEl) return;
    reqEl.textContent = util.fmtNum(data.requests);
    const errEl = dom.id("svc-err-" + svc);
    errEl.textContent = util.fmtNum(data.errors);
    errEl.style.color = data.errors > 0 ? "var(--red)" : "var(--text)";
    const latEl = dom.id("svc-lat-" + svc);
    latEl.textContent = data.avg_latency_ms.toFixed(1);
    latEl.className = "svc-stat-val " + util.latencyClass(data.avg_latency_ms);
    const rateEl = dom.id("svc-rate-" + svc);
    const errRate = ((data.errors / data.requests) * 100).toFixed(1);
    rateEl.textContent = errRate + "%";
    rateEl.style.color = parseFloat(errRate) > 5 ? "var(--red)" : "var(--green)";
    dom.id("svc-dot-" + svc).className = "svc-dot " + ((evt.status >= 200 && evt.status < 400) ? "ok" : "err");
    const card = dom.id("svc-" + svc);
    if (card) {
      card.classList.add("flash");
      setTimeout(() => card.classList.remove("flash"), 400);
    }
  }

  function addLogRow(evt, animate) {
    const tbody = dom.id("log-table");
    const empty = dom.id("empty-log");
    if (empty) empty.style.display = "none";
    const tr = document.createElement("tr");
    if (animate) tr.className = "new-row";
    const time = new Date(evt.ts).toLocaleTimeString("en-GB", {hour12: false});
    tr.innerHTML =
      '<td style="color:var(--text-dim)">' + util.escHtml(time) + "</td>" +
      '<td><span class="method method-' + util.escAttr(evt.method.toLowerCase()) + '">' + util.escHtml(evt.method) + "</span></td>" +
      "<td>" + util.escHtml(evt.path) + "</td>" +
      '<td><span class="status ' + util.statusClass(evt.status) + '">' + util.escHtml(evt.status) + "</span></td>" +
      '<td class="' + util.latencyClass(evt.latency_ms) + '">' + evt.latency_ms.toFixed(1) + "ms</td>" +
      '<td style="color:var(--text-dim)">' + util.escHtml(evt.client_ip) + "</td>" +
      '<td style="color:var(--purple)">' + util.escHtml(evt.user || "-") + "</td>";
    if (tbody.firstChild) {
      tbody.insertBefore(tr, tbody.firstChild);
    } else {
      tbody.appendChild(tr);
    }
    while (tbody.children.length > state.maxLogRows) {
      tbody.removeChild(tbody.lastChild);
    }
    if (state.autoScroll && animate) {
      dom.id("log-body").scrollTop = 0;
    }
  }

  function clearLog() {
    dom.html("log-table", "");
    dom.show("empty-log", "block");
  }

  function togglePause() {
    state.paused = !state.paused;
    const btn = dom.id("btn-pause");
    btn.textContent = state.paused ? "Resume" : "Pause";
    btn.classList.toggle("active", state.paused);
  }

  function renderTestButtons() {
    const container = dom.id("test-console-buttons");
    if (!container) return;
    container.innerHTML = "";

    const healthBtn = document.createElement("button");
    healthBtn.className = "test-btn";
    healthBtn.dataset.action = "test-endpoint";
    healthBtn.dataset.path = "/healthz";
    healthBtn.textContent = "healthz";
    container.appendChild(healthBtn);

    for (const name of state.serviceCatalog.slice().sort()) {
      const button = document.createElement("button");
      button.className = "test-btn";
      button.dataset.action = "test-endpoint";
      button.dataset.path = "/" + name;
      button.dataset.service = name;
      button.textContent = name;
      container.appendChild(button);
    }
  }

  function clearActiveTestBtn() {
    dom.qsa(".test-btn.active").forEach((button) => button.classList.remove("active"));
    closeSvcPanel();
  }

  function toggleSvcPanel(name, trigger) {
    if (state.activeSvcPanel === name) {
      closeSvcPanel();
      return;
    }
    dom.qsa(".test-btn.active").forEach((button) => button.classList.remove("active"));
    trigger.classList.add("active");
    state.activeSvcPanel = name;
    dom.text("svc-access-name", name);
    dom.show("svc-access-panel", "block");
    loadSvcAccessUsers(name);
  }

  function closeSvcPanel() {
    state.activeSvcPanel = null;
    dom.qsa(".test-btn.active").forEach((button) => button.classList.remove("active"));
    dom.hide("svc-access-panel");
  }

  async function loadSvcAccessUsers(serviceName) {
    const container = dom.id("svc-access-users");
    container.innerHTML = '<span class="svc-access-empty">Loading...</span>';
    try {
      const {ok, data} = await api.get("/api/admin/users", {csrf: true});
      if (!ok) {
        container.innerHTML = '<span class="svc-access-empty" style="color:var(--red)">Failed to load users</span>';
        return;
      }
      const users = (data.users || []).filter((user) => user.role !== "admin");
      if (!users.length) {
        container.innerHTML = '<span class="svc-access-empty">No regular users yet.</span>';
        return;
      }
      container.innerHTML = "";
      for (const user of users) {
        const allowed = Array.isArray(user.allowed_services) ? user.allowed_services : [];
        const configured = !!user.service_access_configured;
        const hasAccess = !configured || allowed.includes(serviceName);
        const chip = document.createElement("label");
        chip.className = "svc-user-chip" + (hasAccess ? " has-access" : "");
        chip.innerHTML =
          '<input type="checkbox" data-svc-user="' + util.escAttr(user.username) + '"' +
          ' data-configured="' + configured + '"' +
          " data-allowed='" + util.escAttr(JSON.stringify(allowed)) + "'" +
          (hasAccess ? " checked" : "") + ">" +
          "<span>" + util.escHtml(user.username) + "</span>";
        chip.querySelector("input").addEventListener("change", function() {
          chip.classList.toggle("has-access", this.checked);
        });
        container.appendChild(chip);
      }
    } catch (error) {
      container.innerHTML = '<span class="svc-access-empty" style="color:var(--red)">Error: ' + util.escHtml(error.message) + "</span>";
    }
  }

  async function saveSvcAccess() {
    if (!state.activeSvcPanel) return;
    const checks = dom.qsa("#svc-access-users input[data-svc-user]");
    const saveBtn = dom.qs('#svc-access-panel [data-action="save-service-access"]');
    saveBtn.disabled = true;
    saveBtn.textContent = "Saving...";

    try {
      await Promise.all(checks.map((chk) => {
        const username = chk.dataset.svcUser;
        const hasAccess = chk.checked;
        const configured = chk.dataset.configured === "true";
        const existing = configured ? JSON.parse(chk.dataset.allowed || "[]") : state.serviceCatalog.slice();
        const updated = hasAccess
          ? [...new Set([...existing, state.activeSvcPanel])]
          : existing.filter((service) => service !== state.activeSvcPanel);
        return api.put("/api/admin/users/" + encodeURIComponent(username) + "/access", {allowed_services: updated}, {csrf: true});
      }));
      saveBtn.textContent = "Saved ✓";
      setTimeout(() => {
        saveBtn.textContent = "Save Access";
        saveBtn.disabled = false;
      }, 1500);
      loadSvcAccessUsers(state.activeSvcPanel);
    } catch {
      saveBtn.textContent = "Save Access";
      saveBtn.disabled = false;
    }
  }

  async function testEndpoint(path) {
    const result = dom.id("test-result");
    result.className = "test-result show";
    result.textContent = "Testing " + path + "...";
    result.style.color = "var(--text-dim)";
    result.style.background = "var(--bg-deep)";
    result.style.border = "1px solid var(--border)";
    try {
      const resp = await fetch(state.gatewayOrigin + path);
      let body;
      const contentType = resp.headers.get("content-type") || "";
      if (contentType.includes("json")) {
        body = JSON.stringify(await resp.json(), null, 2);
      } else {
        body = await resp.text();
        if (body.length > 500) {
          body = body.substring(0, 500) + "...";
        }
      }
      result.className = "test-result show " + (resp.ok ? "ok" : "err");
      result.textContent = resp.status + " " + (resp.ok ? "OK" : resp.statusText) + "\n" + body;
    } catch (error) {
      result.className = "test-result show err";
      result.textContent = "Network error: " + error.message;
    }
  }

  async function loadAuditTrail() {
    try {
      const {ok, data} = await api.get("/api/admin/audit", {csrf: true});
      if (!ok) return;
      state.auditTrail = Array.isArray(data.events) ? data.events : [];
      renderAuditTrail();
    } catch {}
  }

  function renderAuditTrail() {
    const tbody = dom.id("audit-tbody");
    if (!tbody) return;
    tbody.innerHTML = "";
    if (!state.auditTrail.length) {
      tbody.innerHTML = '<tr><td colspan="7" style="color:var(--text-dim);padding:16px;text-align:center">No audit events yet</td></tr>';
      return;
    }
    for (const evt of state.auditTrail.slice().reverse()) {
      const when = evt.ts ? new Date(evt.ts).toLocaleString() : "-";
      const tr = document.createElement("tr");
      tr.innerHTML =
        '<td style="color:var(--text-dim)">' + util.escHtml(when) + "</td>" +
        "<td>" + util.escHtml(evt.user || "-") + "</td>" +
        "<td>" + util.escHtml(evt.auth_source || "-") + "</td>" +
        "<td>" + util.escHtml(evt.service || "-") + "</td>" +
        '<td style="max-width:220px;word-break:break-word">' + util.escHtml((evt.method || "-") + " " + (evt.path || "")) + "</td>" +
        '<td><span class="role-pill ' + (evt.decision === "allow" ? "role-user" : "") + '">' + util.escHtml(evt.decision || "-") + "</span></td>" +
        '<td style="max-width:280px;word-break:break-word">' + util.escHtml(evt.reason || "-") + "</td>";
      tbody.appendChild(tr);
    }
  }

  function connectSSE() {
    if (state.evtSource) {
      state.evtSource.close();
    }
    state.evtSource = new EventSource(state.gatewayOrigin + "/api/admin/events");
    state.evtSource.addEventListener("snapshot", (e) => {
      try {
        const snap = JSON.parse(e.data);
        state.uptimeBase = snap.uptime_sec;
        state.uptimeOffset = 0;
        state.totalReq = snap.total;
        state.totalErr = snap.errors;
        state.svcData = snap.services || {};
        renderServiceCards();
        updateCounters();
        if (snap.recent && snap.recent.length > 0) {
          for (const evt of snap.recent) addLogRow(evt, false);
        }
        setConnected(true);
      } catch (err) {
        console.error("Snapshot parse error:", err);
      }
    });
    state.evtSource.addEventListener("request", (e) => {
      try {
        const evt = JSON.parse(e.data);
        state.totalReq += 1;
        if (evt.status >= 400) state.totalErr += 1;
        updateServiceFromEvent(evt);
        updateCounters();
        if (!state.paused) addLogRow(evt, true);
      } catch (err) {
        console.error("Event parse error:", err);
      }
    });
    state.evtSource.onopen = () => setConnected(true);
    state.evtSource.onerror = () => {
      setConnected(false);
      state.evtSource.close();
      if (state.reconnectTimer) clearTimeout(state.reconnectTimer);
      state.reconnectTimer = setTimeout(connectSSE, 3000);
    };
  }

  function init() {
    const logBody = dom.id("log-body");
    if (logBody) {
      logBody.addEventListener("scroll", function() {
        state.autoScroll = this.scrollTop < 10;
      });
    }
    setInterval(() => {
      state.uptimeOffset += 1;
      if (dom.id("uptime")) dom.text("uptime", util.fmtUptime(state.uptimeBase + state.uptimeOffset));
    }, 1000);
  }

  App.actions["toggle-pause"] = togglePause;
  App.actions["clear-log"] = clearLog;
  App.actions["close-service-access-panel"] = closeSvcPanel;
  App.actions["save-service-access"] = saveSvcAccess;
  App.actions["refresh-audit"] = loadAuditTrail;
  App.actions["test-endpoint"] = (el) => {
    const service = el.dataset.service || "";
    if (!service) {
      clearActiveTestBtn();
    }
    testEndpoint(el.dataset.path || "/");
    if (service && state.currentRole === "admin") {
      toggleSvcPanel(service, el);
    }
  };

  App.monitor = {
    init,
    connectSSE,
    setConnected,
    renderServiceCards,
    updateServiceFromEvent,
    updateCounters,
    addLogRow,
    clearLog,
    togglePause,
    renderTestButtons,
    clearActiveTestBtn,
    toggleSvcPanel,
    closeSvcPanel,
    loadSvcAccessUsers,
    saveSvcAccess,
    testEndpoint,
    loadAuditTrail,
    renderAuditTrail
  };
})(window.App);
