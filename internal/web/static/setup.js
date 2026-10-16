"use strict";

// First-run setup wizard (ADR 0022). Loaded before app.js; app.js calls
// HallidoSetup.start() after the texts are loaded and starts the normal app
// once the promise resolves. The wizard only appears while the agent reports
// setup_required (fresh container without an access key):
//   1. welcome -> the server creates the access key (POST /api/v1/setup/token)
//   2. show the key exactly once (copy, confirm "saved")
//   3. speaker search with the new key
//   4. done -> optionally open the scene editor
// The key is stored like a manually entered token (localStorage "sra_token").
(function () {
  const TOKEN_KEY = "sra_token";
  const STEPS = ["welcome", "token", "speakers", "done"];

  const $ = (id) => document.getElementById(id);
  const dlg = $("setupDialog");
  let token = "";
  let finish = null;

  async function fetchStatus() {
    try {
      const res = await fetch("/api/v1/setup", { cache: "no-store" });
      if (!res.ok) return null;
      return await res.json();
    } catch (_) {
      return null;
    }
  }

  function showError(text) {
    const node = $("setupError");
    node.textContent = text || "";
    node.hidden = !text;
  }

  function showStep(name) {
    for (const sec of dlg.querySelectorAll("[data-setup-step]")) {
      sec.hidden = sec.dataset.setupStep !== name;
    }
    const idx = STEPS.indexOf(name);
    $("setupStepNo").textContent = idx >= 0 ? t("setup.step", { n: idx + 1, total: STEPS.length }) : "";
    showError("");
    const focus = dlg.querySelector('[data-setup-step="' + name + '"] h2');
    if (focus) {
      focus.tabIndex = -1;
      focus.focus();
    }
  }

  function showBlocked(reason) {
    const keys = { expired: "expired", not_local: "notLocal", done: "alreadySetUp" };
    const key = keys[reason] || "unavailable";
    $("setupBlockedTitle").textContent = t("setup.blocked." + key + ".title");
    $("setupBlockedText").textContent = t("setup.blocked." + key + ".text");
    showStep("blocked");
  }

  async function createToken() {
    const btn = $("setupCreateBtn");
    btn.disabled = true;
    showError("");
    let tz = "";
    try {
      tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
    } catch (_) { /* old browser: server keeps its default */ }
    try {
      const res = await fetch("/api/v1/setup/token", {
        method: "POST",
        cache: "no-store",
        headers: { "Content-Type": "application/json", "X-Hallido-Setup": "1" },
        body: JSON.stringify({ timezone: tz }),
      });
      let body = null;
      try { body = await res.json(); } catch (_) { /* empty body */ }
      if (res.status === 201 && body && body.token) {
        token = body.token;
        localStorage.setItem(TOKEN_KEY, token);
        renderToken(token);
        showStep("token");
        return;
      }
      const code = body && body.error;
      if (code === "setup_closed") return showBlocked("expired");
      if (code === "forbidden") return showBlocked("not_local");
      if (code === "already_set_up") return showBlocked("done");
      showError((body && body.message) || t("setup.error.generic"));
    } catch (_) {
      showError(t("setup.error.network"));
    } finally {
      btn.disabled = false;
    }
  }

  /** Shows the key with line-break opportunities only between the groups. */
  function renderToken(value) {
    const out = $("setupToken");
    out.replaceChildren();
    value.split("-").forEach((part, i) => {
      if (i > 0) out.append("-", document.createElement("wbr"));
      out.append(part);
    });
  }

  /** Copy: Clipboard API needs HTTPS/localhost; plain http in the LAN uses execCommand. */
  async function copyToken() {
    let ok = false;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(token);
        ok = true;
      }
    } catch (_) { /* fallback below */ }
    if (!ok) {
      const area = document.createElement("textarea");
      area.value = token;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      dlg.appendChild(area);
      area.select();
      try { ok = document.execCommand("copy"); } catch (_) { ok = false; }
      area.remove();
    }
    const label = $("setupCopyBtn").querySelector(".btn-label");
    label.textContent = t(ok ? "setup.token.copied" : "setup.token.copyManual");
    if (!ok) selectToken();
    setTimeout(() => { label.textContent = t("setup.token.copy"); }, 2500);
  }

  function selectToken() {
    const range = document.createRange();
    range.selectNodeContents($("setupToken"));
    const sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
  }

  async function searchSpeakers(fresh) {
    const btn = $("setupSearchBtn");
    const list = $("setupSpeakerList");
    const hint = $("setupSpeakerHint");
    btn.disabled = true;
    btn.setAttribute("aria-busy", "true");
    hint.classList.remove("error");
    hint.textContent = t("setup.speakers.searching");
    try {
      const res = await fetch(fresh ? "/api/v1/discovery/refresh" : "/api/v1/speakers", {
        method: fresh ? "POST" : "GET",
        headers: { Authorization: "Bearer " + token },
      });
      const body = await res.json();
      if (!res.ok) throw new Error((body && body.message) || "HTTP " + res.status);
      const speakers = (body.speakers || []).slice().sort((a, b) => a.name.localeCompare(b.name, I18N.locale));
      list.replaceChildren();
      for (const sp of speakers) {
        const li = document.createElement("li");
        li.className = "setup-speaker" + (sp.reachable === false ? " offline" : "");
        li.textContent = sp.name;
        list.appendChild(li);
      }
      hint.textContent = speakers.length
        ? tn("setup.speakers.found", speakers.length)
        : t("setup.speakers.none");
    } catch (err) {
      hint.classList.add("error");
      hint.textContent = t("setup.speakers.failed", { msg: err.message });
    } finally {
      btn.disabled = false;
      btn.setAttribute("aria-busy", "false");
    }
  }

  function done(next) {
    dlg.close();
    if (finish) finish({ next });
  }

  function bind() {
    // The wizard cannot be dismissed with Escape: without a key the app is unusable.
    dlg.addEventListener("cancel", (e) => e.preventDefault());
    $("setupCreateBtn").addEventListener("click", createToken);
    $("setupCopyBtn").addEventListener("click", copyToken);
    $("setupToken").addEventListener("click", selectToken);
    $("setupSavedCheck").addEventListener("change", (e) => {
      $("setupTokenNext").disabled = !e.target.checked;
    });
    $("setupTokenNext").addEventListener("click", () => {
      showStep("speakers");
      searchSpeakers(false);
    });
    $("setupSearchBtn").addEventListener("click", () => searchSpeakers(true));
    $("setupSpeakersNext").addEventListener("click", () => showStep("done"));
    $("setupSceneBtn").addEventListener("click", () => done("scene"));
    $("setupFinishBtn").addEventListener("click", () => done("later"));
    $("setupRetryBtn").addEventListener("click", async () => {
      const st = await fetchStatus();
      if (!st || !st.setup_required) return done("later");
      if (st.open) showStep("welcome");
      else showBlocked(st.reason);
    });
  }

  /**
   * Resolves with null if no setup is needed, otherwise once the wizard is
   * finished with { next: "scene" | "later" }.
   */
  async function start() {
    if (!dlg) return null;
    const st = await fetchStatus();
    if (!st || !st.setup_required) return null;
    bind();
    dlg.showModal();
    if (st.open) showStep("welcome");
    else showBlocked(st.reason);
    return new Promise((resolve) => { finish = resolve; });
  }

  window.HallidoSetup = { start };
})();
