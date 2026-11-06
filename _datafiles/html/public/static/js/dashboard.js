/* dashboard.js — Dashboard controller + responsive column-stack reflow
 * Three breakpoints:
 *   wide  >= 1100px : default three-column layout (no changes)
 *   rail  700–1100px: left+right cols hidden; side panels in a tabbed rail
 *   phone < 700px   : center fills; fixed bottom tab-bar; slide-up drawer
 */
"use strict";

(function () {

  var SIDE_PANELS = ["map", "quests", "vitals", "art", "chat", "status", "trig"];

  // Game tab names and line icons, shared by the rail and the phone bar.
  // Names are client-only labels (2026-11-01); the panel ids stay as they
  // are. Icons are 24x24 stroke paths so they look the same on every device,
  // unlike emoji.
  var PANEL_TABS = {
    map:    { label: "Map",        icon: ["M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2z", "M9 4v14M15 6v14"] },
    quests: { label: "Quests",     icon: ["M7 3h10a2 2 0 0 1 2 2v14l-3-2-3 2-3-2-3 2V5a2 2 0 0 1 2-2z", "M9 8h6M9 12h4"] },
    vitals: { label: "Health",     icon: ["M12 20s-7-4.4-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 10c0 5.6-7 10-7 10z", "M7 12h3l1.5-2 2 4 1.5-2H17"] },
    art:    { label: "Inventory",  icon: ["M6 8h12l-1 12H7z", "M9 8V6a3 3 0 0 1 6 0v2", "M9 13h6"] },
    chat:   { label: "Chat",       icon: ["M4 5h16v10H9l-5 4z", "M8 9h8M8 12h5"] },
    status: { label: "Status",     icon: ["M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6z", "M9 12l2 2 4-4"] },
    trig:   { label: "Automation", icon: ["M12 6a7 7 0 1 0 0 14a7 7 0 1 0 0-14z", "M12 9v4l3 2M9 3h6"] },
  };

  var SVG_NS = "http://www.w3.org/2000/svg";

  // One round-icon tab bar with a single name pill (design 2026-11-01).
  // The pill rests on the open tab and slides to a hovered or keyboard-focused
  // circle as a preview, then slides back. Touch has no hover, so on touch the
  // pill moves only when a tap opens a tab. onPick(name) runs on click.
  function makeTabBar(extraClass, onPick) {
    var bar = document.createElement("div");
    bar.className = "gtabs " + extraClass;
    bar.setAttribute("role", "group");
    bar.setAttribute("aria-label", "Game panels");

    var pill = document.createElement("span");
    pill.className = "gtab-pill";
    pill.setAttribute("aria-hidden", "true");
    bar.appendChild(pill);

    SIDE_PANELS.forEach(function (name) {
      var tab = PANEL_TABS[name];
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "gtab";
      btn.dataset.panel = name;
      btn.setAttribute("aria-label", tab.label);
      btn.setAttribute("aria-pressed", "false");
      var svg = document.createElementNS(SVG_NS, "svg");
      svg.setAttribute("viewBox", "0 0 24 24");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
      tab.icon.forEach(function (d) {
        var path = document.createElementNS(SVG_NS, "path");
        path.setAttribute("d", d);
        svg.appendChild(path);
      });
      btn.appendChild(svg);
      btn.addEventListener("click", function () { onPick(name); });
      bar.appendChild(btn);
    });

    var openName = null;

    function place(btn, preview) {
      if (!btn) { pill.classList.remove("shown"); return; }
      // Phone bar with no panel open: there is no strip above the bar for the
      // name, so a hover preview would sit over the command box. Skip it.
      if (preview && !openName && extraClass === "gtabs-phone") { pill.classList.remove("shown"); return; }
      pill.textContent = PANEL_TABS[btn.dataset.panel].label;
      pill.classList.toggle("preview", !!preview);
      pill.classList.add("shown");
      var centre = btn.offsetLeft + btn.offsetWidth / 2;
      var w = pill.offsetWidth;
      // Clamp so an end name never leaves the bar (or the screen).
      var x = Math.max(0, Math.min(centre - w / 2, bar.clientWidth - w));
      pill.style.transform = "translateX(" + Math.round(x) + "px)";
    }
    function openBtn() {
      return openName ? bar.querySelector('.gtab[data-panel="' + openName + '"]') : null;
    }
    function rest() { place(openBtn(), false); }

    bar.addEventListener("pointerover", function (e) {
      if (e.pointerType === "touch") return;
      var t = e.target.closest(".gtab");
      if (t) place(t, t.dataset.panel !== openName);
    });
    bar.addEventListener("pointerleave", function () {
      // A keyboard user's focused circle keeps its preview; otherwise go home.
      var f = document.activeElement;
      if (f && bar.contains(f) && f.matches(":focus-visible")) place(f, f.dataset.panel !== openName);
      else rest();
    });
    bar.addEventListener("focusin", function (e) {
      var t = e.target.closest(".gtab");
      if (t && t.matches(":focus-visible")) place(t, t.dataset.panel !== openName);
    });
    bar.addEventListener("focusout", function (e) {
      if (!bar.contains(e.relatedTarget)) rest();
    });

    return {
      el: bar,
      // Mark a tab open (or none with null) and rest the pill on it.
      setOpen: function (name) {
        openName = name;
        bar.querySelectorAll(".gtab").forEach(function (btn) {
          var on = btn.dataset.panel === name;
          btn.classList.toggle("open", on);
          // Screen readers hear which tab is open (#731).
          btn.setAttribute("aria-pressed", on ? "true" : "false");
        });
        rest();
      },
      // Re-measure after the bar is shown or resized (offsets are 0 while hidden).
      relayout: rest,
    };
  }

  var Dashboard = {
    mode: null,         // current mode: "wide" | "rail" | "phone"
    activeSide: "map",  // last-active side panel name

    // DOM refs created once
    _rail: null,
    _railTabs: null,    // makeTabBar() handle
    _phoneTabs: null,   // makeTabBar() handle
    _railBody: null,
    _tabbar: null,
    _drawer: null,
    _drawerBody: null,
    _drawerClose: null, // the phone card's ✕ button

    // ---------------------------------------------------------------
    // Public entry point
    // ---------------------------------------------------------------
    init: function () {
      // Record each side panel's home column so we can restore later.
      // Order within each col is preserved by the original DOM order.
      // Every panel, not only the side ones: redock() needs the Game
      // panel's home too, or a docked Game window lands under the command
      // box (BAR finding, 2026-11-02).
      document.querySelectorAll("#dashboard .dash-panel").forEach(function (panel) {
        var col = panel.closest(".dash-col");
        if (col) panel.dataset.homeCol = col.id;
      });

      // Build rail (inside #dashboard, in the grid area "rail")
      this._buildRail();

      // Build tab-bar and drawer (OUTSIDE #dashboard, appended to body
      // parent so position:fixed is not clipped by the grid container)
      this._buildPhone();

      // Apply initial mode
      this._apply();

      // Window resize listener
      var self = this;
      window.addEventListener("resize", function () {
        self._apply();
        self._relayoutTabs();
      });
      // The name pill is measured in pixels: measure again once the web font lands.
      if (document.fonts && document.fonts.ready) {
        document.fonts.ready.then(function () { self._relayoutTabs(); });
      }

      // Optional hooks for later tasks (splitters, rearrange, popout, layout restore)
      this.initSplitters  && this.initSplitters();
      this.initRearrange  && this.initRearrange();
      this.initPopout     && this.initPopout();
      this.restoreLayout  && this.restoreLayout();
    },

    // ---------------------------------------------------------------
    // Reset layout
    // ---------------------------------------------------------------
    // Reset the dashboard to its default arrangement IN PLACE — no page reload,
    // so the websocket (and the player's session) stays connected. Falls back to
    // the old hard reload only if something unexpected throws.
    resetLayout: function () {
      var self = this;
      try {
        // 1. Dock any popped-out panels (closing the WinBox re-docks via onclose).
        if (this._popped) {
          Object.keys(this._popped).forEach(function (name) {
            var wb = self._popped[name];
            if (wb && typeof wb.close === "function") wb.close();
            else if (self.redock) self.redock(name);
          });
        }
        // 2. Un-collapse every panel.
        document.querySelectorAll(".dash-panel.collapsed").forEach(function (p) {
          p.classList.remove("collapsed");
        });
        // 3. Re-home panels in default order (clear the saved arrangement first
        //    so _enterWide uses the default SIDE_PANELS order, not the custom one).
        this._savedArrange = null;
        this._poppedWhileNarrow = null;
        document.querySelectorAll(".dash-panel").forEach(function (p) { p.style.display = ""; });
        if (this.mode === "rail" && this._enterRail) this._enterRail();
        else if (this.mode === "phone" && this._enterPhone) this._enterPhone();
        else if (this._enterWide) this._enterWide();
        // 4. Forget the persisted layout so a later reload also starts clean.
        try { localStorage.removeItem("dogmud.dashboard.layout.v1"); } catch (e) {}
        // 5. Confirm — no disconnect this time.
        this._toast("Layout reset to default.");
      } catch (e) {
        // Anything unexpected: fall back so the button always does *something*.
        try { localStorage.removeItem("dogmud.dashboard.layout.v1"); } catch (e2) {}
        location.reload();
      }
    },

    _relayoutTabs: function () {
      if (this.mode === "rail" && this._railTabs) this._railTabs.relayout();
      if (this.mode === "phone" && this._phoneTabs) this._phoneTabs.relayout();
    },

    // Brief, self-dismissing notification (bottom-center, leather-styled).
    _toast: function (msg) {
      var t = document.createElement("div");
      t.className = "dash-toast";
      t.textContent = msg;
      document.body.appendChild(t);
      requestAnimationFrame(function () { t.classList.add("show"); });
      setTimeout(function () {
        t.classList.remove("show");
        setTimeout(function () { if (t.parentNode) t.parentNode.removeChild(t); }, 400);
      }, 2600);
    },

    // ---------------------------------------------------------------
    // Mode application
    // ---------------------------------------------------------------
    _computeMode: function () {
      var w = document.getElementById("dashboard").clientWidth;
      if (w >= 1100) return "wide";
      if (w >= 700)  return "rail";
      return "phone";
    },

    _apply: function () {
      var newMode = this._computeMode();
      if (newMode === this.mode) return; // guard: no-op when mode unchanged

      var prev = this.mode;
      this.mode = newMode;

      // Tear down previous mode before setting up new one
      if (prev === "rail")  this._exitRail();
      if (prev === "phone") this._exitPhone();

      // Pop-out windows are a wide-screen feature: dock them when the screen
      // narrows (a floating Game window covered a phone) and pop them out
      // again on the way back to wide.
      if (newMode !== "wide") this._dockPoppedForNarrow();

      document.getElementById("dashboard").dataset.mode = newMode;

      if (newMode === "wide")  this._enterWide();
      if (newMode === "rail")  this._enterRail();
      if (newMode === "phone") this._enterPhone();

      if (newMode === "wide") this._repopAfterNarrow();
    },

    // Panels docked only because the screen is narrow; popped out again in wide.
    _poppedWhileNarrow: null,

    _dockPoppedForNarrow: function () {
      var self = this;
      var names = Object.keys(this._popped || {});
      if (!names.length) return;
      this._poppedWhileNarrow = (this._poppedWhileNarrow || []).concat(names);
      // The saved layout keeps these as popped: they come back in wide.
      this._suspendSave = true;
      try {
        names.forEach(function (name) {
          var wb = self._popped[name];
          if (wb && typeof wb.close === "function") wb.close();
          else self.redock(name);
        });
      } finally {
        // Always resume saving, even if a window throws while closing.
        this._suspendSave = false;
      }
    },

    _repopAfterNarrow: function () {
      var self = this;
      var names = this._poppedWhileNarrow || [];
      this._poppedWhileNarrow = null;
      names.forEach(function (name) {
        if (!(self._popped && self._popped[name])) self.popout(name);
      });
    },

    // ---------------------------------------------------------------
    // Wide mode
    // ---------------------------------------------------------------
    _enterWide: function () {
      // Re-home side panels honoring the saved arrangement order when available
      // (prevents a wide→rail→wide round-trip from resetting a custom layout).
      // Falls back to the default SIDE_PANELS order when no saved arrange exists.
      var arrange = this._savedArrange;
      if (arrange && typeof arrange === "object") {
        // Replay saved column order: for each column append panels in saved sequence.
        // Any panel not listed in saved arrange falls through to the default below.
        var placed = {};
        ["dash-col-left", "dash-col-center", "dash-col-right"].forEach(function (colId) {
          var col = document.getElementById(colId);
          if (!col) return;
          var names = arrange[colId];
          if (!Array.isArray(names)) return;
          names.forEach(function (name) {
            var panel = document.getElementById("panel-" + name);
            if (!panel) return;
            if (panel.dataset.popped === "1") return;
            col.appendChild(panel);
            panel.dataset.homeCol = colId;
            panel.style.display = "";
            placed[name] = true;
          });
        });
        // Catch any panels not covered by the saved arrange (new panels added after save)
        SIDE_PANELS.forEach(function (name) {
          if (placed[name]) return;
          var panel = document.getElementById("panel-" + name);
          if (!panel) return;
          if (panel.dataset.popped === "1") return;
          var homeId = panel.dataset.homeCol;
          if (!homeId) return;
          var col = document.getElementById(homeId);
          if (col) col.appendChild(panel);
          panel.style.display = "";
        });
      } else {
        // No saved arrangement — use default SIDE_PANELS order
        // (map/vitals/art → left, chat/status/trig → right)
        SIDE_PANELS.forEach(function (name) {
          var panel = document.getElementById("panel-" + name);
          if (!panel) return;
          if (panel.dataset.popped === "1") return; // skip popped-out panels
          var homeId = panel.dataset.homeCol;
          if (!homeId) return;
          var col = document.getElementById(homeId);
          if (col) col.appendChild(panel);
          panel.style.display = "";
        });
      }

      // Hide rail, tabbar, drawer
      if (this._rail)   this._rail.style.display   = "none";
      if (this._tabbar) this._tabbar.style.display  = "none";
      if (this._drawer) {
        this._drawer.classList.remove("open");
        this._drawer.style.display = "none";
      }
    },

    // ---------------------------------------------------------------
    // Rail mode
    // ---------------------------------------------------------------
    _buildRail: function () {
      var rail = document.createElement("div");
      rail.id = "dash-rail";
      rail.style.display = "none";

      var self = this;
      var tabs = makeTabBar("gtabs-rail", function (name) { self._setRailActive(name); });
      rail.appendChild(tabs.el);

      var body = document.createElement("div");
      body.className = "rail-body";
      rail.appendChild(body);

      document.getElementById("dashboard").appendChild(rail);

      this._rail     = rail;
      this._railTabs = tabs;
      this._railBody = body;
    },

    _enterRail: function () {
      var self = this;

      // Show rail, hide phone chrome
      this._rail.style.display = "";
      if (this._tabbar) this._tabbar.style.display = "none";
      if (this._drawer) {
        this._drawer.classList.remove("open");
        this._drawer.style.display = "none";
      }

      // Move all side panels into rail body
      SIDE_PANELS.forEach(function (name) {
        var panel = document.getElementById("panel-" + name);
        if (!panel) return;
        if (panel.dataset.popped === "1") return; // skip popped-out panels
        // Return from drawer if it was there
        if (self._drawerBody && self._drawerBody.contains(panel)) {
          self._drawerBody.removeChild(panel);
        }
        self._railBody.appendChild(panel);
        panel.style.display = "none"; // will show only the active tab's panel
      });

      // Restore last-active or default to "map"
      this._setRailActive(this.activeSide);
    },

    _setRailActive: function (name) {
      this.activeSide = name;

      this._railTabs.setOpen(name);

      // Show only the active panel
      SIDE_PANELS.forEach(function (pName) {
        var panel = document.getElementById("panel-" + pName);
        if (!panel) return;
        if (panel.dataset.popped === "1") return; // skip popped-out panels
        panel.style.display = (pName === name) ? "" : "none";
      });
    },

    _exitRail: function () {
      // Move panels back to home cols (wide will re-place them)
      var self = this;
      SIDE_PANELS.forEach(function (name) {
        var panel = document.getElementById("panel-" + name);
        if (!panel) return;
        if (panel.dataset.popped === "1") return; // skip popped-out panels
        if (self._railBody && self._railBody.contains(panel)) {
          self._railBody.removeChild(panel);
          var homeId = panel.dataset.homeCol;
          if (homeId) {
            var col = document.getElementById(homeId);
            if (col) col.appendChild(panel);
          }
        }
        panel.style.display = "";
      });

      this._rail.style.display = "none";
    },

    // ---------------------------------------------------------------
    // Phone mode
    // ---------------------------------------------------------------
    _buildPhone: function () {
      var self = this;
      var parent = document.getElementById("dashboard").parentNode;

      // Tab-bar
      var tabs = makeTabBar("gtabs-phone", function (name) {
        // Toggle: if drawer is open for same panel, close it
        if (self._drawer.classList.contains("open") &&
            self._drawerActive === name) {
          self._closeDrawer();
        } else {
          self._openDrawer(name);
        }
      });
      var tabbar = document.createElement("div");
      tabbar.id = "dash-tabbar";
      tabbar.style.display = "none";
      tabbar.appendChild(tabs.el);

      parent.appendChild(tabbar);
      this._tabbar = tabbar;
      this._phoneTabs = tabs;

      // Drawer
      var drawer = document.createElement("div");
      drawer.id = "dash-drawer";

      // The ✕ that closes the open tab: a square button, the same as the
      // phone's Settings square. It sits in the open tab's own title bar, to
      // the right of its ▾ button (see _openDrawer), so the card needs no
      // extra strip just for it and the game keeps that room.
      var closeBtn = document.createElement("button");
      closeBtn.type = "button";
      closeBtn.className = "brass drawer-close";
      closeBtn.textContent = "✕︎";
      closeBtn.title = "Close";
      closeBtn.setAttribute("aria-label", "Close");
      closeBtn.setAttribute("draggable", "false");
      closeBtn.addEventListener("click", function (ev) {
        ev.stopPropagation();
        self._closeDrawer();
      });

      var drawerBody = document.createElement("div");
      drawerBody.className = "drawer-body";
      drawer.appendChild(drawerBody);

      parent.appendChild(drawer);
      this._drawer     = drawer;
      this._drawerClose = closeBtn;
      this._drawerBody = drawerBody;
      this._drawerActive = null;
    },

    _enterPhone: function () {
      // Show tab-bar; hide rail
      if (this._tabbar) this._tabbar.style.display = "";
      if (this._rail)   this._rail.style.display   = "none";

      // Ensure all side panels are in their home cols (hidden via CSS)
      var self = this;
      SIDE_PANELS.forEach(function (name) {
        var panel = document.getElementById("panel-" + name);
        if (!panel) return;
        if (panel.dataset.popped === "1") return; // skip popped-out panels
        // If it's in rail body, move it home
        if (self._railBody && self._railBody.contains(panel)) {
          self._railBody.removeChild(panel);
        }
        // If it's in drawer, move it home
        if (self._drawerBody && self._drawerBody.contains(panel)) {
          self._drawerBody.removeChild(panel);
        }
        var homeId = panel.dataset.homeCol;
        if (homeId) {
          var col = document.getElementById(homeId);
          if (col && !col.contains(panel)) col.appendChild(panel);
        }
        panel.style.display = "";
      });

      // Close drawer if open
      if (this._drawer) {
        this._drawer.classList.remove("open");
        this._drawerActive = null;
      }

      this._updateTabbarActive(null);
    },

    _exitPhone: function () {
      // Return any drawer panel to its home col
      this._closeDrawer();

      if (this._tabbar) this._tabbar.style.display = "none";
      if (this._drawer) this._drawer.style.display  = "none";
    },

    _openDrawer: function (name) {
      // Return current drawer panel to its home col first
      if (this._drawerActive && this._drawerActive !== name) {
        var prev = document.getElementById("panel-" + this._drawerActive);
        if (prev && this._drawerBody.contains(prev)) {
          this._drawerBody.removeChild(prev);
          var prevHomeId = prev.dataset.homeCol;
          if (prevHomeId) {
            var prevCol = document.getElementById(prevHomeId);
            if (prevCol) prevCol.appendChild(prev);
          }
        }
      }

      var panel = document.getElementById("panel-" + name);
      if (!panel) return;

      // Move panel into drawer body
      this._drawerBody.appendChild(panel);
      panel.style.display = "";

      // Put the ✕ in this tab's title bar, after its ▾ button, and name it
      // for screen readers ("Close Chat", "Close Map" and so on).
      var btns = panel.querySelector(".ph-btns");
      if (btns && this._drawerClose) {
        btns.appendChild(this._drawerClose);
        var label = "Close " + (PANEL_TABS[name] ? PANEL_TABS[name].label : name);
        this._drawerClose.title = label;
        this._drawerClose.setAttribute("aria-label", label);
      }

      this._drawerActive = name;
      this._drawer.style.display = "";
      // Force reflow so CSS transition fires
      this._drawer.getBoundingClientRect();
      this._drawer.classList.add("open");

      // Moving a tab into the card scrolls it back to the top. For Chat that
      // would show the oldest messages, so once the card is open, jump back
      // to the newest one.
      var chatMessages = panel.querySelector(".chat-window.active");
      if (chatMessages) chatMessages.scrollTop = chatMessages.scrollHeight;

      this._updateTabbarActive(name);
    },

    _closeDrawer: function () {
      if (!this._drawerActive) return;

      var panel = document.getElementById("panel-" + this._drawerActive);
      if (panel && this._drawerBody && this._drawerBody.contains(panel)) {
        this._drawerBody.removeChild(panel);
        var homeId = panel.dataset.homeCol;
        if (homeId) {
          var col = document.getElementById(homeId);
          if (col) col.appendChild(panel);
        }
        panel.style.display = "";
      }

      this._drawerActive = null;
      if (this._drawer) {
        this._drawer.classList.remove("open");
      }
      this._updateTabbarActive(null);
    },

    _updateTabbarActive: function (activeName) {
      // Open a strip for the name pill only while a panel is open.
      if (this._tabbar) this._tabbar.classList.toggle("named", !!activeName);
      if (this._phoneTabs) this._phoneTabs.setOpen(activeName);
    },

    // ---------------------------------------------------------------
    // Layout persistence (localStorage key: dogmud.dashboard.layout.v1)
    // ---------------------------------------------------------------

    // In-memory copy of the last-saved arrange block so _enterWide can
    // honor it after a wide→rail→wide round-trip without re-reading LS.
    _savedArrange: null,

    saveLayout: function () {
      if (this._suspendSave) return;
      try {
        var el = document.getElementById("dashboard");

        // Column widths (always safe to capture from the #dashboard style)
        var cols = {
          l: el ? el.style.getPropertyValue("--col-l-w") : "",
          r: el ? el.style.getPropertyValue("--col-r-w") : ""
        };

        // Collapsed panels (class survives mode changes — always safe)
        var collapsed = [];
        document.querySelectorAll(".dash-panel.collapsed").forEach(function (p) {
          if (p.dataset.panel) collapsed.push(p.dataset.panel);
        });

        // Popped panels (always safe)
        // Include panels docked only for a narrow screen, so they pop out again.
        var popped = Object.keys(this._popped || {}).concat(this._poppedWhileNarrow || []);

        // Arrange: only valid in wide mode. In rail/phone the panels are
        // relocated away from their columns, so DOM order is not the real
        // arrangement. Preserve whatever was previously saved for arrange
        // and only overwrite it when we're actually in wide mode.
        var arrange;
        if (this.mode === "wide") {
          arrange = {};
          ["dash-col-left", "dash-col-center", "dash-col-right"].forEach(function (colId) {
            var col = document.getElementById(colId);
            if (!col) return;
            arrange[colId] = [];
            col.querySelectorAll(".dash-panel").forEach(function (p) {
              if (p.dataset.panel) arrange[colId].push(p.dataset.panel);
            });
          });
          this._savedArrange = arrange;
        } else {
          // Read whatever was already persisted so we can write it back unchanged
          try {
            var existing = JSON.parse(localStorage.getItem("dogmud.dashboard.layout.v1") || "{}");
            arrange = existing.arrange || this._savedArrange || null;
          } catch (e) {
            arrange = this._savedArrange || null;
          }
        }

        var payload = { cols: cols, collapsed: collapsed, popped: popped };
        if (arrange) payload.arrange = arrange;

        localStorage.setItem("dogmud.dashboard.layout.v1", JSON.stringify(payload));
      } catch (e) {
        // localStorage may be unavailable (private browsing, quota, etc.) — ignore
      }
    },

    restoreLayout: function () {
      var self = this;
      var data;
      try {
        var raw = localStorage.getItem("dogmud.dashboard.layout.v1");
        if (!raw) return;
        data = JSON.parse(raw);
        if (!data || typeof data !== "object") return;
      } catch (e) {
        return;
      }

      // 1. Column widths
      try {
        var el = document.getElementById("dashboard");
        if (el && data.cols) {
          if (data.cols.l) el.style.setProperty("--col-l-w", data.cols.l);
          if (data.cols.r) el.style.setProperty("--col-r-w", data.cols.r);
        }
      } catch (e) {}

      // 2. Arrange — only apply if we're currently in wide mode
      try {
        if (this.mode === "wide" && data.arrange && typeof data.arrange === "object") {
          this._savedArrange = data.arrange;
          var arrange = data.arrange;
          ["dash-col-left", "dash-col-center", "dash-col-right"].forEach(function (colId) {
            var col = document.getElementById(colId);
            if (!col) return;
            var names = arrange[colId];
            if (!Array.isArray(names)) return;
            names.forEach(function (name) {
              var panel = document.getElementById("panel-" + name);
              if (!panel) return;
              col.appendChild(panel);
              panel.dataset.homeCol = colId;
            });
          });
        } else if (data.arrange) {
          // Cache it so _enterWide can use it later even if we're in rail/phone now
          this._savedArrange = data.arrange;
        }
      } catch (e) {}

      // 3. Collapsed panels
      try {
        if (Array.isArray(data.collapsed)) {
          data.collapsed.forEach(function (name) {
            var panel = document.getElementById("panel-" + name);
            if (panel) panel.classList.add("collapsed");
          });
        }
      } catch (e) {}

      // 4. Popped panels — do this LAST (depends on panels being in place)
      try {
        if (Array.isArray(data.popped)) {
          data.popped.forEach(function (name) {
            // Not wide: keep it docked until the screen is wide again.
            if (self.mode !== "wide") {
              self._poppedWhileNarrow = (self._poppedWhileNarrow || []).concat(name);
            } else {
              self.popout && self.popout(name);
            }
          });
        }
      } catch (e) {}
    },

    // ---------------------------------------------------------------
    // Pop-out panels into floating WinBox windows
    // ---------------------------------------------------------------
    initPopout: function () {
      var self = this;
      this._popped = this._popped || {};
      this._dockAnchor = this._dockAnchor || {};

      // Wire every pop-out button (side panels AND the game feed)
      document.querySelectorAll("#dashboard .ph-popout").forEach(function (btn) {
        var panel = btn.closest(".dash-panel");
        if (!panel) return;
        btn.addEventListener("click", function (ev) {
          ev.stopPropagation();
          self.popout(panel.dataset.panel);
        });
      });
    },

    popout: function (name) {
      var self = this;
      // Toggle: the ⧉ now lives in the floating window, so a second press re-docks.
      if (this._popped[name]) { this._popped[name].close(); return; }
      if (this.mode !== "wide") return; // wide-screen feature; the ⧉ is hidden elsewhere

      var panel = document.getElementById("panel-" + name);
      if (!panel) return;

      // Remember the slot so we can re-dock in the SAME position (not the bottom).
      this._dockAnchor[name] = panel.nextElementSibling || null;

      // Mark before creating the WinBox so reflow guards see it immediately
      panel.dataset.popped = "1";

      var titleEl = panel.querySelector(".ph-title");
      var title = titleEl ? titleEl.textContent.trim() : name;

      var wb = new WinBox({
        title: title,
        mount: panel,
        class: "dash-winbox",
        width: 360,
        height: 300,
        onclose: function () {
          self.redock(name);
          // return undefined → WinBox proceeds with close normally
        }
      });

      this._popped[name] = wb;
      this.saveLayout && this.saveLayout();
    },

    redock: function (name) {
      var panel = document.getElementById("panel-" + name);
      if (panel) {
        panel.removeAttribute("style");      // drop WinBox inline sizing → CSS flex governs again
        panel.removeAttribute("data-popped");
        var homeId = panel.dataset.homeCol;
        var col = homeId ? document.getElementById(homeId) : null;
        if (col) {
          var anchor = this._dockAnchor ? this._dockAnchor[name] : null;
          if (anchor && anchor.parentNode === col) col.insertBefore(panel, anchor);
          else col.appendChild(panel);
        }
      }
      if (this._dockAnchor) delete this._dockAnchor[name];
      delete this._popped[name];
      this.saveLayout && this.saveLayout();
    },

    // ---------------------------------------------------------------
    // Column-width splitters (wide mode only)
    // ---------------------------------------------------------------
    initSplitters: function () {
      var self = this;
      var el = document.getElementById("dashboard");

      function clamp(v, min, max) {
        return Math.max(min, Math.min(max, v));
      }

      function makeSplitter(id, edge) {
        var s = document.createElement("div");
        s.id = id;
        s.className = "dash-splitter col";
        s.dataset.edge = edge;
        el.appendChild(s);

        var rect = null;

        s.addEventListener("pointerdown", function (ev) {
          if (self.mode !== "wide") return;
          rect = el.getBoundingClientRect();
          s.classList.add("dragging");
          s.setPointerCapture(ev.pointerId);
          ev.preventDefault();
        });

        s.addEventListener("pointermove", function (ev) {
          if (!s.classList.contains("dragging")) return;
          if (edge === "l") {
            var w = clamp(ev.clientX - rect.left, 180, rect.width * 0.45);
            el.style.setProperty("--col-l-w", w + "px");
          } else {
            var w = clamp(rect.right - ev.clientX, 180, rect.width * 0.45);
            el.style.setProperty("--col-r-w", w + "px");
          }
        });

        s.addEventListener("pointerup", function (ev) {
          if (!s.classList.contains("dragging")) return;
          s.classList.remove("dragging");
          s.releasePointerCapture(ev.pointerId);
          self.saveLayout && self.saveLayout();
        });
      }

      makeSplitter("dash-splitter-l", "l");
      makeSplitter("dash-splitter-r", "r");
    },

    // ---------------------------------------------------------------
    // Drag-to-swap panel rearrange + collapse toggle (wide mode only)
    // ---------------------------------------------------------------
    initRearrange: function () {
      var self = this;
      var dragSourcePanel = null; // data-panel value of the panel being dragged

      // Helper: swap two <section> elements in the DOM, regardless of
      // whether they live in the same or different parent columns.
      function swapNodes(a, b) {
        var marker = document.createComment("swap");
        a.parentNode.insertBefore(marker, a);
        b.parentNode.insertBefore(a, b);
        marker.parentNode.insertBefore(b, marker);
        marker.parentNode.removeChild(marker);
      }

      // Update data-homeCol for a panel to reflect its current column.
      function updateHomeCol(panel) {
        var col = panel.closest(".dash-col");
        if (col) panel.dataset.homeCol = col.id;
      }

      var panels = document.querySelectorAll(".dash-panel");

      panels.forEach(function (panel) {
        var head = panel.querySelector(".dash-panel-head");
        if (!head) return;

        // --- Drag handle: mark header as draggable ---
        head.setAttribute("draggable", "true");

        // Prevent the small button spans from initiating a swap drag
        var btnSpans = head.querySelectorAll(".ph-collapse, .ph-popout");
        btnSpans.forEach(function (span) {
          span.setAttribute("draggable", "false");
        });

        // dragstart — only if wide mode
        head.addEventListener("dragstart", function (ev) {
          if (self.mode !== "wide") {
            ev.preventDefault();
            return;
          }
          dragSourcePanel = panel.dataset.panel;
          ev.dataTransfer.effectAllowed = "move";
          ev.dataTransfer.setData("text/plain", dragSourcePanel);
        });

        head.addEventListener("dragend", function () {
          dragSourcePanel = null;
          // clear any lingering drop highlight (e.g. drag released off-window)
          var hl = document.querySelectorAll("#dashboard .dash-panel.drag-over");
          for (var i = 0; i < hl.length; i++) hl[i].classList.remove("drag-over");
        });

        // --- Drop target: the entire panel section ---
        panel.addEventListener("dragover", function (ev) {
          if (!dragSourcePanel || self.mode !== "wide") return;
          if (panel.dataset.panel === dragSourcePanel) return;
          ev.preventDefault();
          panel.classList.add("drag-over");
        });

        panel.addEventListener("dragleave", function () {
          panel.classList.remove("drag-over");
        });

        panel.addEventListener("drop", function (ev) {
          ev.preventDefault();
          panel.classList.remove("drag-over");

          if (!dragSourcePanel || self.mode !== "wide") return;
          var targetPanelName = panel.dataset.panel;
          if (targetPanelName === dragSourcePanel) return;

          var srcPanel = document.getElementById("panel-" + dragSourcePanel);
          var dstPanel = panel;
          if (!srcPanel || !dstPanel) return;

          swapNodes(srcPanel, dstPanel);

          // Update data-homeCol for both panels (critical for responsive restore)
          updateHomeCol(srcPanel);
          updateHomeCol(dstPanel);

          dragSourcePanel = null;
          self.saveLayout && self.saveLayout();
        });

        // --- Collapse toggle ---
        var collapseBtn = head.querySelector(".ph-collapse");
        if (collapseBtn) {
          collapseBtn.addEventListener("click", function (ev) {
            ev.stopPropagation();
            panel.classList.toggle("collapsed");
            self.saveLayout && self.saveLayout();
          });
        }
      });
    },
  };

  window.Dashboard = Dashboard;

  document.addEventListener("DOMContentLoaded", function () {
    Dashboard.init();
  });

})();
