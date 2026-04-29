(function(App) {
  const {dom, api, util} = App;

  function setNotice(msg, type) {
    util.setNotice("rl-notice", msg, type);
  }

  async function loadRateLimits() {
    try {
      const {ok, data} = await api.get("/api/admin/settings/ratelimits");
      if (!ok) return;
      dom.id("rl-default-rpm").value = data.default?.requests_per_minute ?? 60;
      dom.id("rl-default-burst").value = data.default?.burst ?? 10;
      const list = dom.id("rl-users-list");
      list.innerHTML = "";
      for (const [user, rule] of Object.entries(data.users || {})) {
        addRLUserRow(user, rule.requests_per_minute, rule.burst);
      }
    } catch {}
  }

  function addRLUserRow(username = "", rpm = 60, burst = 10) {
    const list = dom.id("rl-users-list");
    const div = document.createElement("div");
    div.className = "rl-user-row";
    div.innerHTML =
      '<input class="form-input" placeholder="username" value="' + util.escAttr(username) + '" style="width:110px;min-width:0">' +
      '<input class="rl-input" type="number" min="0" value="' + rpm + '" placeholder="requests/min" title="Sustained requests allowed per minute. Set 0 to disable limiting for this user.">' +
      '<span class="rl-unit">requests/min</span>' +
      '<input class="rl-input" type="number" min="0" value="' + burst + '" placeholder="instant requests" title="Immediate requests allowed before throttling.">' +
      '<span class="rl-unit">instant requests</span>' +
      '<button class="btn-danger" data-action="remove-rate-limit-row" style="padding:3px 8px">x</button>';
    list.appendChild(div);
  }

  function parseLimitValue(value, fallback) {
    const parsed = parseInt(value, 10);
    if (Number.isNaN(parsed) || parsed < 0) return fallback;
    return parsed;
  }

  async function saveRateLimits() {
    const settings = {
      default: {
        requests_per_minute: parseLimitValue(dom.id("rl-default-rpm").value, 60),
        burst: parseLimitValue(dom.id("rl-default-burst").value, 10)
      },
      users: {}
    };
    for (const row of dom.id("rl-users-list").children) {
      const inputs = row.querySelectorAll("input");
      const username = inputs[0].value.trim();
      if (!username) continue;
      settings.users[username] = {
        requests_per_minute: parseLimitValue(inputs[1].value, 60),
        burst: parseLimitValue(inputs[2].value, 10)
      };
    }
    try {
      const {ok} = await api.post("/api/admin/settings/ratelimits", settings, {csrf: true});
      if (!ok) {
        setNotice("Failed to save", "err");
        return;
      }
      setNotice("Rate limits saved", "ok");
    } catch {
      setNotice("Network error", "err");
    }
  }

  App.actions["add-rate-limit-user-row"] = () => addRLUserRow();
  App.actions["save-rate-limits"] = saveRateLimits;
  App.actions["remove-rate-limit-row"] = (el) => el.closest(".rl-user-row")?.remove();

  App.rateLimits = {
    loadRateLimits,
    addRLUserRow,
    saveRateLimits
  };
})(window.App);
