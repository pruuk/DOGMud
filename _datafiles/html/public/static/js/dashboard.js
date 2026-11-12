/* dashboard.js: Dashboard controller and the layout ladder (owner 2026-11-12).
 * The game window is on the left, exactly as wide as 80 columns
 * (setGameWidth, from termfit.js). The side panels take the room right of
 * it, in #dash-panels, and layoutpick.js picks one of four modes from that
 * room:
 *   wide   : Layout D, every side panel open (spec ruling 9)
 *   column : Map, Health and Conditions open; the other four on the tab bar
 *   slim   : Health and Conditions open; the other five on the tab bar
 *   phone  : every side panel on the tab bar under the game
 * One tab bar and slide-up drawer serve column, slim and phone; the CSS
 * hides the buttons of the panels a mode keeps open. Pop-out windows are a
 * wide-only feature.
 */
"use strict";

(function () {

  var SIDE_PANELS = ["map", "quests", "vitals", "art", "chat", "status", "trig"];

  // The game column is termfit's 80-column width plus the Game panel's
  // border (1px each side, dashboard.css .dash-panel) and 4px of slack: a
  // page zoomed below 100% snaps the border to a device pixel and the fit
  // addon rounds a pixel away, which gave 79 columns without it (plan 5,
  // Task 7 step 2). termfit caps the terminal at 80, so the slack is harmless.
  var FEED_BORDER = 2, SLACK = 4;

  // The modes whose side panels open from the tab bar, and each mode's
  // enter and exit.
  var TABBED = { column: true, slim: true, phone: true };
  var ENTER = { wide: "_enterWide", column: "_enterColumn", slim: "_enterSlim", phone: "_enterPhone" };
  var EXIT = { column: "_exitColumn", slim: "_exitSlim", phone: "_exitPhone" };

  // Saved layout: folded and popped-out panels and the pop-out windows'
  // sizes and places. v1 also held a column arrangement and widths, which
  // the ladder replaced; restoreLayout carries the rest over once.
  var LAYOUT_KEY = "dogmud.dashboard.layout.v2";
  var OLD_LAYOUT_KEY = "dogmud.dashboard.layout.v1";

  // Game tab names and line icons, used by the one tab bar.
  // Names are client-only labels (2026-11-01); the panel ids stay as they
  // are. Icons are 24x24 stroke paths so they look the same on every device,
  // unlike emoji.
  var PANEL_TABS = {
    map:    { label: "Map",        icon: ["M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2z", "M9 4v14M15 6v14"] },
    quests: { label: "Quests",     icon: ["M7 3h10a2 2 0 0 1 2 2v14l-3-2-3 2-3-2-3 2V5a2 2 0 0 1 2-2z", "M9 8h6M9 12h4"] },
    vitals: { label: "Health",     icon: ["M12 20s-7-4.4-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 10c0 5.6-7 10-7 10z", "M7 12h3l1.5-2 2 4 1.5-2H17"] },
    art:    { label: "Inventory",  icon: ["M6 8h12l-1 12H7z", "M9 8V6a3 3 0 0 1 6 0v2", "M9 13h6"] },
    chat:   { label: "Chat",       icon: ["M4 5h16v10H9l-5 4z", "M8 9h8M8 12h5"] },
    status: { label: "Conditions",   icon: ["M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6z", "M9 12l2 2 4-4"] },
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
    mode: null,         // current mode: "wide" | "column" | "slim" | "phone"
    gameW: null,        // the measured game column's width in px, or null before termfit measures it (the layout then uses LayoutPick.GAME_W_DEFAULT)
    ready: false,       // init has run: setGameWidth may be called

    // DOM refs created once
    _phoneTabs: null,   // makeTabBar() handle
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

      // Build tab-bar and drawer (OUTSIDE #dashboard, appended to body
      // parent so position:fixed is not clipped by the grid container)
      this._buildPhone();

      // Apply the first mode, from the game width termfit last reported.
      // The page keeps it on window because termfit reports before this
      // script sets itself up (plan 5, Task 8). A width measured before the
      // web font loaded is not used: the stylesheet's default column holds
      // until the page passes the real one.
      this.setGameWidth(window.dogmudFontReady ? window.dogmudGameWidth || null : null);
      this.ready = true;

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

      // Folding, pop-out and the saved layout
      this.initCollapse   && this.initCollapse();
      this.initPopout     && this.initPopout();
      this.restoreLayout  && this.restoreLayout();
    },

    // The game window's width at 80 columns (termfit.js widthFor80), or
    // null before it is measured. The game column is that plus the Game
    // panel's border and the slack; the CSS reads it as --game-w on the
    // root (not #dashboard: the tab bar and drawer live outside it). Until
    // termfit measures, the column is LayoutPick.GAME_W_DEFAULT, so a
    // desktop never gets the phone layout only for lack of a width. The
    // wide, column and slim grids use var(--game-w) with no fallback, so
    // it is always set.
    setGameWidth: function (px) {
      this.gameW = px ? px + FEED_BORDER + SLACK : null;
      document.documentElement.style.setProperty("--game-w",
        (this.gameW || LayoutPick.GAME_W_DEFAULT) + "px");
      this._apply();
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
        // 3. Re-home panels in their page order by entering the current
        //    mode again, whichever of the four it is.
        this._poppedWhileNarrow = null;
        this._boxGeom = null;
        document.querySelectorAll(".dash-panel").forEach(function (p) { p.style.display = ""; });
        if (this.mode && ENTER[this.mode]) this[ENTER[this.mode]]();
        // 4. Forget the persisted layout so a later reload also starts clean.
        try { localStorage.removeItem(LAYOUT_KEY); } catch (e) {}
        // 5. Confirm — no disconnect this time.
        this._toast("Layout reset to default.");
      } catch (e) {
        // Anything unexpected: fall back so the button always does *something*.
        try { localStorage.removeItem(LAYOUT_KEY); } catch (e2) {}
        location.reload();
      }
    },

    _relayoutTabs: function () {
      if (TABBED[this.mode] && this._phoneTabs) this._phoneTabs.relayout();
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
    // A phone always gets the phone layout: a touch screen whose short side
    // is under 600px. The touch test keeps a zoomed-in desktop on a small
    // screen (1280x720 at 125% has a short side under 600) on the ladder.
    _computeMode: function () {
      var w = document.getElementById("dashboard").clientWidth;
      var phone = ViewportFit.isPhone(screen.width, screen.height) &&
                  matchMedia("(pointer: coarse)").matches;
      return LayoutPick.pick(w, this.gameW || LayoutPick.GAME_W_DEFAULT, phone);
    },

    // The panel area's width, for the column mode's tab bar and drawer.
    // It changes on every resize within a mode too.
    _measurePanels: function () {
      var panels = document.getElementById("dash-panels");
      if (!panels) return;
      document.documentElement.style.setProperty("--panels-w",
        Math.round(panels.getBoundingClientRect().width) + "px");
    },

    _apply: function () {
      this._measurePanels();
      var newMode = this._computeMode();
      if (newMode === this.mode) return; // guard: no-op when mode unchanged

      var prev = this.mode;
      this.mode = newMode;

      // Tear down previous mode before setting up new one. Wide has nothing
      // to tear down; its pop-outs are docked just below.
      if (prev && EXIT[prev]) this[EXIT[prev]]();

      // Pop-out windows are a wide-screen feature: dock them when the screen
      // narrows (a floating Game window covered a phone) and pop them out
      // again on the way back to wide.
      if (newMode !== "wide") this._dockPoppedForNarrow();

      document.getElementById("dashboard").dataset.mode = newMode;
      // For the CSS that hides tab bar buttons and places the bar and drawer.
      var rootClasses = document.documentElement.classList;
      rootClasses.toggle("layout-column", newMode === "column");
      rootClasses.toggle("layout-slim", newMode === "slim");

      this[ENTER[newMode]]();
      // The panel area has just changed shape.
      this._measurePanels();

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
      // Every side panel back in its home column, in the page's order
      // (map/quests/vitals/art left, chat/status/trig right). The columns
      // are display: contents in #dash-panels, so the CSS grid areas place
      // each panel in Layout D.
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

      // Hide tabbar, drawer
      if (this._tabbar) this._tabbar.style.display  = "none";
      if (this._drawer) {
        this._drawer.classList.remove("open");
        this._drawer.style.display = "none";
      }
    },

    // ---------------------------------------------------------------
    // Column, slim and phone modes: one tab bar and drawer. The CSS
    // decides which panels stay open in #dash-panels, which tab bar
    // buttons show, and where the bar and the drawer sit.
    // ---------------------------------------------------------------
    _enterColumn: function () { this._enterTabbed(); },
    _exitColumn:  function () { this._exitTabbed(); },
    _enterSlim:   function () { this._enterTabbed(); },
    _exitSlim:    function () { this._exitTabbed(); },
    _enterPhone:  function () { this._enterTabbed(); },
    _exitPhone:   function () { this._exitTabbed(); },

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

    _enterTabbed: function () {
      // Show tab-bar
      if (this._tabbar) this._tabbar.style.display = "";

      // Ensure all side panels are in their home cols (the CSS hides those
      // the mode puts on the tab bar)
      var self = this;
      SIDE_PANELS.forEach(function (name) {
        var panel = document.getElementById("panel-" + name);
        if (!panel) return;
        if (panel.dataset.popped === "1") return; // skip popped-out panels
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

    _exitTabbed: function () {
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
      this._scrollChatToNewest(panel);

      this._updateTabbarActive(name);
    },

    // Show the newest Chat messages. Hiding a panel's body (a fold, a move
    // into the phone card) loses its scroll place and it comes back at the
    // oldest message. Panels without a chat window are left alone.
    _scrollChatToNewest: function (panel) {
      var chatMessages = panel ? panel.querySelector(".chat-window.active") : null;
      if (chatMessages) chatMessages.scrollTop = chatMessages.scrollHeight;
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
    // Layout persistence (localStorage key: LAYOUT_KEY, v2)
    // ---------------------------------------------------------------

    saveLayout: function () {
      if (this._suspendSave) return;
      try {
        // Collapsed panels (class survives mode changes — always safe)
        var collapsed = [];
        document.querySelectorAll(".dash-panel.collapsed").forEach(function (p) {
          if (p.dataset.panel) collapsed.push(p.dataset.panel);
        });

        // Popped panels (always safe)
        // Include panels docked only for a narrow screen, so they pop out again.
        var popped = Object.keys(this._popped || {}).concat(this._poppedWhileNarrow || []);

        // No arrangement or column widths: panels always sit in the page's
        // order, and the ladder places them. A popped panel docks back
        // before its anchor (redock), which is its place in that order
        // (#768 review item 14).
        var payload = { collapsed: collapsed, popped: popped };
        if (this._boxGeom && Object.keys(this._boxGeom).length) payload.boxes = this._boxGeom;

        localStorage.setItem(LAYOUT_KEY, JSON.stringify(payload));
      } catch (e) {
        // localStorage may be unavailable (private browsing, quota, etc.) — ignore
      }
    },

    // The saved layout, or null. An old v1 entry with no v2 is carried over
    // once: its folded panels, popped-out panels and window places move to
    // v2, its column arrangement and widths are dropped, and v1 goes.
    _readLayout: function () {
      try {
        if (localStorage.getItem(LAYOUT_KEY) === null) {
          var old = localStorage.getItem(OLD_LAYOUT_KEY);
          if (old !== null) {
            // v1 goes only once v2 holds what it carried (or v1 held
            // nothing to carry), so a failed write keeps it for next time.
            try {
              var o = JSON.parse(old);
              if (o && typeof o === "object") {
                var carried = {};
                if (Array.isArray(o.collapsed)) carried.collapsed = o.collapsed;
                if (Array.isArray(o.popped)) carried.popped = o.popped;
                if (o.boxes && typeof o.boxes === "object") carried.boxes = o.boxes;
                localStorage.setItem(LAYOUT_KEY, JSON.stringify(carried));
              }
              localStorage.removeItem(OLD_LAYOUT_KEY);
            } catch (e) {
              // A v1 that is not JSON can never be carried: drop it.
              if (e instanceof SyntaxError) {
                try { localStorage.removeItem(OLD_LAYOUT_KEY); } catch (e2) {}
              }
            }
          }
        }
        var raw = localStorage.getItem(LAYOUT_KEY);
        if (!raw) return null;
        var data = JSON.parse(raw);
        return data && typeof data === "object" ? data : null;
      } catch (e) {
        return null;
      }
    },

    // Pop-out window size and place per panel, kept while the window is
    // moved or resized and when it closes or docks, and used again on the
    // next pop-out so a window comes back where the player left it.
    _boxGeom: null,

    _rememberBox: function (name, wb) {
      if (!wb) return;
      var g = { x: wb.x, y: wb.y, width: wb.width, height: wb.height };
      if (!isFinite(g.x) || !isFinite(g.y) || !(g.width > 0) || !(g.height > 0)) return;
      this._boxGeom = this._boxGeom || {};
      this._boxGeom[name] = g;
    },

    // Moving a window fires many times a second: save once it settles.
    _saveSoon: function () {
      var self = this;
      if (this._saveTimer) clearTimeout(this._saveTimer);
      this._saveTimer = setTimeout(function () {
        self._saveTimer = null;
        self.saveLayout && self.saveLayout();
      }, 300);
    },

    restoreLayout: function () {
      var self = this;
      var data = this._readLayout();
      if (!data) return;

      // 1. Pop-out window sizes and places, before any panel pops out below.
      try {
        if (data.boxes && typeof data.boxes === "object") {
          Object.keys(data.boxes).forEach(function (name) {
            self._rememberBox(name, data.boxes[name]);
          });
        }
      } catch (e) {}

      // 2. Collapsed panels
      try {
        if (Array.isArray(data.collapsed)) {
          data.collapsed.forEach(function (name) {
            var panel = document.getElementById("panel-" + name);
            if (panel) panel.classList.add("collapsed");
          });
        }
      } catch (e) {}

      // 3. Popped panels, LAST (depends on panels being in place)
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

      var opts = {
        title: title,
        mount: panel,
        class: "dash-winbox",
        width: 360,
        height: 300,
        onmove: function () { self._rememberBox(name, this); self._saveSoon(); },
        onresize: function () { self._rememberBox(name, this); self._saveSoon(); },
        onclose: function () {
          // Keep the size and place for the next pop-out, whether the player
          // closed it or a narrow screen docked it.
          self._rememberBox(name, this);
          self.redock(name);
          // return undefined → WinBox proceeds with close normally
        }
      };
      // Come back at the size and place it had last time, kept on screen.
      var geom = this._boxGeom && this._boxGeom[name];
      if (geom) {
        var vw = window.innerWidth || geom.x + geom.width;
        var vh = window.innerHeight || geom.y + geom.height;
        opts.width = Math.min(geom.width, vw);
        opts.height = Math.min(geom.height, vh);
        opts.x = Math.max(0, Math.min(geom.x, vw - opts.width));
        opts.y = Math.max(0, Math.min(geom.y, vh - opts.height));
      }
      var wb = new WinBox(opts);

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
          // The anchor may itself be popped out: dock before its anchor.
          var hops = 0;
          while (anchor && anchor.parentNode !== col && anchor.dataset &&
                 anchor.dataset.popped === "1" && hops++ < 20) {
            anchor = this._dockAnchor[anchor.dataset.panel] || null;
          }
          if (anchor && anchor.parentNode === col) col.insertBefore(panel, anchor);
          else col.appendChild(panel);
        }
      }
      if (this._dockAnchor) delete this._dockAnchor[name];
      delete this._popped[name];
      this.saveLayout && this.saveLayout();
    },

    // ---------------------------------------------------------------
    // Fold toggle (the ▾ button), in every mode
    // ---------------------------------------------------------------
    initCollapse: function () {
      var self = this;
      document.querySelectorAll(".dash-panel").forEach(function (panel) {
        var head = panel.querySelector(".dash-panel-head");
        if (!head) return;
        var collapseBtn = head.querySelector(".ph-collapse");
        if (collapseBtn) {
          collapseBtn.addEventListener("click", function (ev) {
            ev.stopPropagation();
            panel.classList.toggle("collapsed");
            // Unfolded: back to the newest Chat message, not the oldest.
            if (!panel.classList.contains("collapsed")) self._scrollChatToNewest(panel);
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
