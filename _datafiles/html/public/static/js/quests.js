"use strict";
/*
 * quests.js — the quest editor (admin web-building 5c), fifth mode of the
 * /build page. Consumes Build.Quests (list) + Build.Quest (detail) GMCP. The
 * editor is off until quest arc phase 6 (P7): the server refuses
 * Build.Quest.Get/Update/Create/Delete, so the save and delete calls here are
 * dormant until then. Mirrors the removed dialogue editor's collapsible
 * (whole-row ▸/▾ toggle) and chip/line helpers; every emitted key is the
 * struct's yaml/json name (they are mirrored server-side).
 */
(function () {
  function ce(tag, attrs, kids) {
    var e = document.createElement(tag);
    if (attrs) for (var k in attrs) {
      if (k === "text") e.textContent = attrs[k];
      else if (k === "html") e.innerHTML = attrs[k];
      else e.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (c) { if (c) e.appendChild(c); });
    return e;
  }
  function gmcp(pkg, obj) { if (window.Builder && window.Builder.sendGMCP) window.Builder.sendGMCP(pkg, obj); }
  function toast(m, e) { if (window.Builder && window.Builder.toast) window.Builder.toast(m, e); }
  function field(labelText, input, hint) {
    var kids = [ce("label", { text: labelText }), input];
    if (hint) kids.push(ce("div", { style: "font-size:10px;color:var(--gold-dim);margin-top:2px;", text: hint }));
    return ce("div", {}, kids);
  }
  function sectionTitle(t) { return ce("h3", { text: t }); }
  function markDirty() { Panel.dirty = true; }
  function toInt(v) { var n = Math.round(parseFloat(v)); return isNaN(n) ? 0 : n; }
  // QM (questmerge.js): every gather overlays its controls on a deep copy of
  // the object the editor loaded, so a field with no control survives a save.
  var QM = window.QuestMerge;
  // orUndef is a control's empty value as undefined, which merge deletes.
  function orUndef(v) { return v ? v : undefined; }

  var Panel = {
    rows: [],
    search: "",
    selectedId: 0,
    listBody: null,
    enums: {},
    dirty: false,
    saving: false,
    deleting: false,
    zoneRows: [],       // Build.Zones rows (room picker assist)
    mobRows: [],        // Build.Mobs rows (mob pickers)
    roomsByZone: {},    // zone -> Build.Rooms rows
    _pendingRoomZone: "",
    _roomSelects: []    // live {sel, zoneSel} pairs to refill on Build.Rooms
  };

  // ---- generic input builders ------------------------------------------

  function textInput(val) {
    var i = ce("input", { type: "text" }); i.value = val == null ? "" : val;
    i.addEventListener("input", markDirty); return i;
  }
  function textArea(val) {
    var t = ce("textarea", {}); t.value = val || "";
    t.addEventListener("input", markDirty); return t;
  }
  function numInput(val, dl) {
    var i = ce("input", { type: "text", style: "max-width:110px;" });
    if (dl) i.setAttribute("list", dl);
    i.value = val ? String(val) : "";
    i.addEventListener("input", markDirty); return i;
  }
  function boolInput(val) {
    var c = ce("input", { type: "checkbox" }); c.checked = !!val;
    c.addEventListener("change", markDirty); return c;
  }
  function strPick(val, dl) {
    var i = ce("input", { type: "text" });
    if (dl) i.setAttribute("list", dl);
    i.value = val || "";
    i.addEventListener("input", markDirty); return i;
  }

  // chips: a removable-tag list with a text entry (token lists, flag values).
  function chips(vals, dl) {
    var box = ce("div", { "class": "chips" });
    function row(v) {
      var r = ce("span", { "class": "chip" });
      r.appendChild(ce("span", { text: v }));
      var rm = ce("button", { "class": "mini rm", text: "×" });
      rm.addEventListener("click", function () { box.removeChild(r); markDirty(); });
      r.appendChild(rm);
      r._val = v;
      box.insertBefore(r, add);
      return r;
    }
    var entry = ce("input", { type: "text", placeholder: "add value, press Enter", style: "min-width:140px;" });
    if (dl) entry.setAttribute("list", dl);
    entry.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter" && entry.value.trim()) { row(entry.value.trim()); entry.value = ""; markDirty(); ev.preventDefault(); }
    });
    var add = ce("span", {}, [entry]);
    box.appendChild(add);
    (vals || []).forEach(row);
    return {
      el: box,
      get: function () {
        var out = [];
        Array.prototype.forEach.call(box.children, function (c) { if (c._val != null) out.push(c._val); });
        if (entry.value.trim()) out.push(entry.value.trim());
        return out;
      }
    };
  }

  // pairRows: structured editing for "name:number[,name:number]" composite
  // strings (skillinfo, stat_info, item_info).
  function pairRows(joined, dl, leftPlaceholder, rightPlaceholder) {
    var box = ce("div", {});
    function row(l, r) {
      var left = ce("input", { type: "text", placeholder: leftPlaceholder, style: "max-width:160px;" });
      if (dl) left.setAttribute("list", dl);
      left.value = l || "";
      var right = ce("input", { type: "text", placeholder: rightPlaceholder, style: "max-width:70px;" });
      right.value = r || "";
      left.addEventListener("input", markDirty); right.addEventListener("input", markDirty);
      var rm = ce("button", { "class": "mini rm", text: "×" });
      var wrap = ce("div", {}, [left, right, rm]);
      rm.addEventListener("click", function () { box.removeChild(wrap); markDirty(); });
      wrap._pair = function () {
        if (!left.value.trim()) return "";
        return right.value.trim() ? left.value.trim() + ":" + right.value.trim() : left.value.trim();
      };
      box.insertBefore(wrap, addBtn);
    }
    var addBtn = ce("button", { "class": "mini", text: "+ add" });
    addBtn.addEventListener("click", function () { row("", ""); markDirty(); });
    box.appendChild(addBtn);
    (joined || "").split(",").forEach(function (part) {
      part = part.trim();
      if (!part) return;
      var bits = part.split(":");
      row(bits[0], bits.length > 1 ? bits[1] : "");
    });
    return {
      el: box,
      get: function () {
        var parts = [];
        Array.prototype.forEach.call(box.children, function (c) {
          if (c._pair) { var p = c._pair(); if (p) parts.push(p); }
        });
        return parts.join(",");
      }
    };
  }

  // collapsible row shell — the whole-row ▸/▾ toggle the removed dialogue editor used.
  function collapsible(box, summaryFn, buildBody, ordered) {
    var body = ce("div", { style: "display:none;padding:4px 0 8px 10px;" });
    var sum = ce("span", { text: "▸ " + summaryFn() });
    var head = ce("div", { "class": "irow", style: "cursor:pointer;" });
    head.appendChild(sum);
    if (ordered) {
      var up = ce("button", { "class": "mini", text: "↑", title: "move up" });
      var dn = ce("button", { "class": "mini", text: "↓", title: "move down" });
      up.addEventListener("click", function (ev) { ev.stopPropagation(); var w = head.parentNode; var prev = w.previousElementSibling; if (prev && prev._isRow) { w.parentNode.insertBefore(w, prev); markDirty(); } });
      dn.addEventListener("click", function (ev) { ev.stopPropagation(); var w = head.parentNode; var next = w.nextElementSibling; if (next && next._isRow) { w.parentNode.insertBefore(next, w); markDirty(); } });
      head.appendChild(up); head.appendChild(dn);
    }
    var rm = ce("button", { "class": "mini rm", text: "✕", title: "remove" });
    head.appendChild(rm);
    var wrap = ce("div", {}, [head, body]);
    wrap._isRow = true;
    rm.addEventListener("click", function (ev) { ev.stopPropagation(); wrap.parentNode.removeChild(wrap); markDirty(); });
    head.addEventListener("click", function () {
      var open = body.style.display !== "none";
      body.style.display = open ? "none" : "";
      sum.textContent = (open ? "▸ " : "▾ ") + summaryFn();
    });
    var built = buildBody(body);
    wrap._gather = built.gather;
    wrap._refreshSummary = function () {
      sum.textContent = (body.style.display !== "none" ? "▾ " : "▸ ") + summaryFn();
    };
    box.appendChild(wrap);
    return wrap;
  }

  // room picker assist: the numeric input is authoritative; the zone+room
  // dropdowns just fill it (no client-side room→zone reverse lookup exists).
  function roomAssist(numEl) {
    var zoneSel = ce("select", {});
    zoneSel.appendChild(ce("option", { value: "", text: "(zone)" }));
    Panel.zoneRows.forEach(function (z) {
      zoneSel.appendChild(ce("option", { value: z.zone, text: z.zone }));
    });
    var roomSel = ce("select", {});
    roomSel.appendChild(ce("option", { value: "", text: "(room)" }));
    function fill(zone) {
      roomSel.innerHTML = "";
      roomSel.appendChild(ce("option", { value: "", text: "(room)" }));
      (Panel.roomsByZone[zone] || []).forEach(function (r) {
        roomSel.appendChild(ce("option", { value: String(r.id), text: "#" + r.id + " " + (r.title || "") }));
      });
    }
    zoneSel.addEventListener("change", function () {
      if (!zoneSel.value) return;
      if (Panel.roomsByZone[zoneSel.value]) { fill(zoneSel.value); return; }
      Panel._pendingRoomZone = zoneSel.value;
      gmcp("Build.Room.List", { zone: zoneSel.value });
    });
    roomSel.addEventListener("change", function () {
      if (roomSel.value) { numEl.value = roomSel.value; markDirty(); }
    });
    Panel._roomSelects.push({ sel: roomSel, zoneSel: zoneSel, fill: fill });
    return ce("span", {}, [zoneSel, roomSel]);
  }

  // ---- list ------------------------------------------------------------

  Panel.render = function (rows) {
    this.rows = rows || [];
    var host = document.getElementById("questlist");
    if (!host) return;
    host.innerHTML = "";

    // The quest editor is off (quest arc phase 2a, P7): a save could break the
    // quest loop rules and stop the next boot. The server refuses Build.Quest.Get,
    // Update, Create and Delete; say so here instead of offering their buttons.
    host.appendChild(ce("div", {
      "class": "quest-off",
      style: "font-size:12px;color:var(--gold-dim);margin:6px 0;",
      text: "Quest editing is off for now. The quest builder arrives in quest arc phase 6. " +
        "Until then, edit quest files in the repo. The list below is read-only."
    }));

    var search = ce("input", { type: "text", placeholder: "search id or name" });
    search.value = this.search;
    search.addEventListener("input", function () { Panel.search = search.value; Panel.drawRows(); });
    host.appendChild(ce("div", { "class": "filters" }, [search]));

    this.listBody = ce("div", {});
    host.appendChild(this.listBody);
    this.drawRows();
  };

  Panel.drawRows = function () {
    if (!this.listBody) return;
    this.listBody.innerHTML = "";
    var q = this.search.trim().toLowerCase();
    var shown = 0;
    this.rows.forEach(function (r) {
      if (q && String(r.id).indexOf(q) === -1 && (r.name || "").toLowerCase().indexOf(q) === -1) return;
      shown++;
      var row = ce("div", { "class": "irow" + (r.id === Panel.selectedId ? " sel" : "") });
      row.appendChild(ce("span", { "class": "iid", text: "#" + r.id + " " }));
      row.appendChild(document.createTextNode(r.name || "(unnamed)"));
      row.appendChild(ce("span", { "class": "mzone", text: "  " + r.stepCount + " steps · " + r.triggerCount + " triggers" }));
      if (r.secret) row.appendChild(ce("span", { "class": "mbadge", text: "secret" }));
      if (r.repeatable) row.appendChild(ce("span", { "class": "mbadge", text: "repeatable" }));
      // No click-to-open: Build.Quest.Get is refused while the editor is off.
      Panel.listBody.appendChild(row);
    });
    if (!shown) this.listBody.appendChild(ce("div", { "style": "color:var(--gold-dim);font-style:italic;padding:8px;", text: "no quests match" }));
  };

  // ---- action sub-forms -------------------------------------------------

  // Field kinds: token/item/mob/room/condition = picker on the matching datalist
  // (numeric except token); spell/recipe/faction/skill/stat/flagkey = string
  // picker; text/num/bool literal. "custom" types get dedicated builders.
  var ACTION_FORMS = {
    grant:         [{ k: "grant", kind: "token", label: "Token" }],
    consume_item:  [{ k: "consume_item", kind: "item", label: "Item" }],
    give_item:     [{ k: "give_item", kind: "item", label: "Item" }],
    give_gold:     [{ k: "give_gold", kind: "num", label: "Gold" }],
    charge_gold:   [{ k: "charge_gold", kind: "num", label: "Gold" }],
    actor:         [{ k: "actor", kind: "longtext", label: "Text (player only)" }],
    observer:      [{ k: "observer", kind: "longtext", label: "Text (whole room)" }],
    teach_spell:   [{ k: "teach_spell", kind: "spell", label: "Spell" }],
    teleport:      [{ k: "teleport", kind: "room", label: "Room" }],
    give_mutation: [{ k: "give_mutation", kind: "bool", label: "Roll a random mutation" }],
    discover:      [{ k: "discover", kind: "text", label: "Hidden noun key (trigger's room)" }],
    npc_say:       "custom",
    spawn_mob:     [{ k: "id", kind: "mob", label: "Mob", nest: true }, { k: "room", kind: "room", label: "Room", nest: true }],
    spawn_item:    [{ k: "id", kind: "item", label: "Item", nest: true }, { k: "room", kind: "room", label: "Room", nest: true }],
    train_skill:   [{ k: "skill", kind: "skill", label: "Skill", nest: true }, { k: "level", kind: "num", label: "Level", nest: true }],
    train_stat:    [{ k: "stat", kind: "stat", label: "Stat", nest: true }, { k: "amount", kind: "num", label: "Amount", nest: true }],
    learn_recipe:  [{ k: "recipe", kind: "recipe", label: "Recipe", nest: true }],
    apply_condition:    [{ k: "condition", kind: "condition", label: "Status condition", nest: true }, { k: "source", kind: "text", label: "Source (optional)", nest: true }],
    set_flag:      [{ k: "key", kind: "flagkey", label: "Flag key", nest: true }, { k: "value", kind: "text", label: "Value", nest: true }],
    bump_rep:      [{ k: "faction", kind: "faction", label: "Faction", nest: true }, { k: "delta", kind: "num", label: "Delta", nest: true }],
    sequence:      "custom",
    declare_bounty: "custom"
  };

  var KIND_DL = { token: "q-token-dl", item: "q-item-dl", mob: "q-mob-dl", condition: "q-condition-dl",
    spell: "q-spell-dl", recipe: "q-recipe-dl", faction: "q-faction-dl", skill: "q-skill-dl",
    stat: "q-stat-dl", flagkey: "q-flagkey-dl" };
  var NUMERIC_KINDS = { item: true, mob: true, room: true, condition: true, num: true };

  function kindInput(kind, val) {
    if (kind === "bool") return boolInput(val);
    if (kind === "text") return textInput(val);
    // longtext: player-facing prose that runs over lines. A text input drops
    // line breaks and readKind trims, so a one-line control would flatten it.
    if (kind === "longtext") return textArea(val);
    if (kind === "target" || kind === "reason") {
      var sel = ce("select", {});
      [""].concat(kind === "target" ? TARGET_KINDS : GAIN_REASONS).forEach(function (k) {
        var o = ce("option", { value: k, text: k || "(any)" }); if ((val || "") === k) o.selected = true; sel.appendChild(o);
      });
      sel.addEventListener("change", markDirty);
      return { el: sel, read: function () { return sel.value; } };
    }
    if (kind === "room") {
      var n = numInput(val);
      return { el: ce("span", {}, [n, roomAssist(n)]), read: function () { return toInt(n.value); } };
    }
    if (NUMERIC_KINDS[kind]) return numInput(val, KIND_DL[kind]);
    return strPick(val, KIND_DL[kind]);
  }
  function readKind(kind, input) {
    if (input && input.read) return input.read();
    if (kind === "bool") return !!input.checked;
    if (NUMERIC_KINDS[kind]) return toInt(input.value);
    if (kind === "longtext") return input.value;
    return input.value.trim();
  }

  // saylines: the npc_say / sequence line list (delay, text, speaker, emote).
  function sayLines(lines) {
    var box = ce("div", {});
    function row(l) {
      l = l || {};
      var delay = numInput(l.delay);
      var text = textInput(l.text);
      text.style.minWidth = "260px";
      var speaker = numInput(l.speaker, "q-mob-dl");
      var emote = boolInput(l.emote);
      var rm = ce("button", { "class": "mini rm", text: "×" });
      var wrap = ce("div", { style: "margin:2px 0;" });
      [["delay", delay], ["line", text], ["speaker (0=main)", speaker]].forEach(function (p) {
        wrap.appendChild(ce("span", { style: "font-size:10px;color:var(--gold-dim);margin:0 3px;", text: p[0] }));
        wrap.appendChild(p[1]);
      });
      wrap.appendChild(ce("span", { style: "font-size:10px;color:var(--gold-dim);margin:0 3px;", text: "emote" }));
      wrap.appendChild(emote);
      wrap.appendChild(rm);
      rm.addEventListener("click", function () { box.removeChild(wrap); markDirty(); });
      wrap._line = function () {
        return QM.merge(l, { text: text.value, delay: orUndef(toInt(delay.value)),
          speaker: orUndef(toInt(speaker.value)), emote: orUndef(emote.checked) });
      };
      box.insertBefore(wrap, addBtn);
    }
    var addBtn = ce("button", { "class": "mini", text: "+ line" });
    addBtn.addEventListener("click", function () { row(null); markDirty(); });
    box.appendChild(addBtn);
    (lines || []).forEach(row);
    return {
      el: box,
      get: function () {
        var out = [];
        Array.prototype.forEach.call(box.children, function (c) {
          if (c._line) { var l = c._line(); if (l.text) out.push(l); }
        });
        return out;
      }
    };
  }

  function actionTypeOf(a) {
    for (var t in ACTION_FORMS) {
      var v = a[t];
      if (v === undefined || v === null || v === "" || v === 0 || v === false) continue;
      return t;
    }
    return "";
  }

  // buildActionBody renders one action's sub-form into body; returns gather().
  function buildActionBody(type, a, body, allowSequence) {
    a = a || {};
    if (type === "npc_say") {
      var src = a.npc_say || {};
      var mob = numInput(src.mob, "q-mob-dl");
      body.appendChild(field("Mob (speaker)", mob));
      var lines = sayLines(src.lines);
      body.appendChild(field("Lines", lines.el));
      return function () { return { npc_say: QM.merge(src, { mob: toInt(mob.value), lines: lines.get() }) }; };
    }
    if (type === "sequence") {
      var sq = a.sequence || {};
      var delayB = numInput(sq.delay_between);
      body.appendChild(field("Delay between lines (seconds)", delayB));
      var sqLines = sayLines(sq.lines);
      body.appendChild(field("Lines", sqLines.el));
      var lockMsg = textInput(sq.lock_message);
      body.appendChild(field("Lock message (blocks movement while running; empty = no lock)", lockMsg));
      body.appendChild(ce("div", { style: "font-size:10px;color:var(--gold-dim);margin-top:6px;", text: "On complete, actions run after the last line (a sequence cannot nest another sequence):" }));
      var onC = actionList(sq.on_complete, body, false);
      return function () {
        var oc = onC.get();
        return { sequence: QM.merge(sq, { delay_between: toInt(delayB.value), lines: sqLines.get(),
          on_complete: oc.length ? oc : undefined, lock_message: orUndef(lockMsg.value.trim()) }) };
      };
    }
    if (type === "declare_bounty") {
      var db = a.declare_bounty || {}; var issuer = db.issuer || {}; var target = db.target || {};
      var isType = ce("select", {});
      ["faction", "quest", "npc"].forEach(function (t) {
        var o = ce("option", { value: t, text: t }); if (issuer.type === t) o.selected = true; isType.appendChild(o);
      });
      isType.addEventListener("change", markDirty);
      var isId = textInput(issuer.id);
      body.appendChild(field("Issuer type", isType));
      body.appendChild(field("Issuer id (\"<self>\" with type=quest auto-fills this quest)", isId));
      var tp = boolInput(db.target_player);
      body.appendChild(field("Target the quest holder", tp));
      var tType = ce("select", {});
      ["", "player", "mob"].forEach(function (t) {
        var o = ce("option", { value: t, text: t || "(none)" }); if (target.type === t) o.selected = true; tType.appendChild(o);
      });
      tType.addEventListener("change", markDirty);
      var tId = numInput(target.id, "q-mob-dl");
      body.appendChild(field("Explicit target type (instead of the holder)", tType));
      body.appendChild(field("Explicit target id", tId));
      var cond = textInput(db.condition || "kill");
      body.appendChild(field("Condition", cond, "\"kill\" is the only condition the engine evaluates today"));
      var exp = numInput(db.expiry_rounds); var gOv = numInput(db.gold_override); var rOv = numInput(db.rep_override);
      body.appendChild(field("Expiry rounds (0 = default)", exp));
      body.appendChild(field("Gold override", gOv));
      body.appendChild(field("Rep override", rOv));
      var reason = textInput(db.reason);
      body.appendChild(field("Reason", reason));
      return function () {
        return { declare_bounty: QM.merge(db, {
          issuer: QM.merge(issuer, { type: isType.value, id: isId.value.trim() }),
          condition: cond.value.trim(),
          target_player: orUndef(tp.checked),
          target: tType.value ? QM.merge(target, { type: tType.value, id: toInt(tId.value) }) : undefined,
          expiry_rounds: orUndef(toInt(exp.value)), gold_override: orUndef(toInt(gOv.value)),
          rep_override: orUndef(toInt(rOv.value)), reason: orUndef(reason.value.trim())
        }) };
      };
    }

    // Table-driven simple forms.
    var spec = ACTION_FORMS[type];
    var readers = [];
    var nested = spec.length && spec[0].nest;
    spec.forEach(function (f) {
      var src = nested ? ((a[type] || {})[f.k]) : a[f.k];
      var input = kindInput(f.kind, src);
      var el = input.el || input;
      body.appendChild(field(f.label, el));
      readers.push({ f: f, input: input });
    });
    return function () {
      var out = {};
      if (nested) {
        var inner = {};
        readers.forEach(function (r) { inner[r.f.k] = readKind(r.f.kind, r.input); });
        out[type] = QM.merge(a[type], inner);
      } else {
        readers.forEach(function (r) { out[r.f.k] = readKind(r.f.kind, r.input); });
      }
      return out;
    };
  }

  // actionList: the ORDERED action list (top-level per trigger, and nested
  // once inside sequence on_complete).
  function actionList(actions, host, allowSequence) {
    var box = ce("div", { style: "border-left:2px solid var(--tooled);padding-left:8px;" });
    function actionRow(a) {
      var type = a ? actionTypeOf(a) : "";
      if (!type) type = "actor";
      collapsible(box, function () { return "action: " + type; }, function (body) {
        var gather = buildActionBody(type, a, body, allowSequence);
        return { gather: function () { return QM.merge(a, gather()); } };
      }, true);
    }
    (actions || []).forEach(actionRow);
    var addSel = ce("select", {});
    addSel.appendChild(ce("option", { value: "", text: "+ add action…" }));
    ((Panel.enums.actions) || []).forEach(function (v) {
      if (!allowSequence && v.key === "sequence") return;
      addSel.appendChild(ce("option", { value: v.key, text: v.key + ": " + v.description }));
    });
    addSel.addEventListener("change", function () {
      if (!addSel.value) return;
      var type = addSel.value; addSel.value = "";
      var stub = {};
      // Seed the stub so actionTypeOf resolves the chosen type on render.
      var spec = ACTION_FORMS[type];
      if (spec === "custom" || (spec.length && spec[0].nest)) stub[type] = {};
      else if (spec.length && spec[0].kind === "bool") stub[spec[0].k] = true;
      else stub[spec[0].k] = spec[0].kind === "num" || NUMERIC_KINDS[spec[0].kind] ? 0 : " ";
      // Render with the explicit type (stub values may be zero/empty).
      collapsible(box, function () { return "action: " + type; }, function (body) {
        var gather = buildActionBody(type, stub, body, allowSequence);
        return { gather: function () { return QM.merge({}, gather()); } };
      }, true);
      markDirty();
    });
    var wrap = ce("div", {}, [box, addSel]);
    host.appendChild(wrap);
    return {
      get: function () {
        var out = [];
        Array.prototype.forEach.call(box.children, function (c) { if (c._gather) out.push(c._gather()); });
        return out;
      }
    };
  }

  // ---- triggers ---------------------------------------------------------

  // The trigger fields each event offers. Only listed fields get a control;
  // a field missing here survives a save untouched (questmerge.js), except
  // that a filter another event offers is cleared when the event changes.
  // Every event takes area and zone; count and label are refused on room_gate,
  // look_at, search_at and item_give (a partial count would swallow the
  // command);
  // distinct and explored_line are room_enter's (an explore step). Kept in
  // step with questEventVocab in modules/gmcp/gmcp.Quest.go by
  // TestQuestBuilderFiltersMatchTheVocab.
  var EVENT_FILTERS = {
    room_enter: ["room", "area", "zone", "count", "label", "distinct", "explored_line"],
    room_gate: ["room", "area", "zone"],
    item_give: ["mob", "item", "area", "zone"],
    skill_use: ["skill", "area", "zone", "count", "label"],
    mob_death: ["mob", "room", "area", "zone", "count", "label"],
    command: ["command", "noun", "room", "target", "mob", "item", "area", "zone", "count", "label"],
    command_issued: ["command", "room", "area", "zone", "count", "label"],
    item_gain: ["item", "room", "reason", "area", "zone", "count", "label", "held_line"],
    quest_granted: ["quest_token", "area", "zone", "count", "label"],
    look_at: ["room", "noun", "area", "zone"],
    search_at: ["room", "noun", "area", "zone"],
    spell_landed: ["noun", "effect", "target", "mob", "item", "room", "area", "zone", "count", "label"],
    topic_heard: ["mob", "topic", "area", "zone", "count", "label"]
  };
  var FILTER_KINDS = { room: "room", mob: "mob", item: "item", skill: "skill",
    command: "text", quest_token: "token", noun: "text",
    effect: "text", topic: "text", target: "target", reason: "reason",
    area: "text", zone: "text", count: "num", label: "text",
    distinct: "text", explored_line: "text", held_line: "text" };
  // Fields that shape the trigger rather than filter its event get their own
  // label in the form.
  var FIELD_LABELS = {
    count: "Count: fire on the Nth match (empty = first)",
    label: "Progress label (with count or distinct)",
    distinct: "Distinct: rooms of the area to map (all or a number)",
    explored_line: "Explored line (said when the area is already mapped)",
    held_line: "Held line (said when the step starts with the item already held)"
  };
  // The values a trigger's target filter may take (quests.TriggerTargetKinds);
  // empty means any target.
  var TARGET_KINDS = ["self", "mob", "player", "item", "container"];
  // The values an item_gain trigger's reason filter may take
  // (quests.TriggerGainReasons); empty means any way of arriving.
  var GAIN_REASONS = ["get", "give", "buy", "loot", "forage", "harvest", "steal",
    "search_find", "storage", "vault", "auction", "topic_give", "craft", "salvage",
    "reward", "quest_give", "tree_give", "mail", "shield_swap", "plant"];

  function triggerRow(box, t) {
    t = t || { event: "room_enter", actions: [] };
    collapsible(box, function () {
      return "trigger: " + (t.event || "?") + " · " + ((t.actions || []).length) + " action(s)";
    }, function (body) {
      var evSel = ce("select", {});
      ((Panel.enums.events) || []).forEach(function (v) {
        var o = ce("option", { value: v.key, text: v.key });
        if (t.event === v.key) o.selected = true;
        evSel.appendChild(o);
      });
      var evDesc = ce("div", { style: "font-size:10px;color:var(--gold-dim);margin:2px 0 6px;" });
      function descFor(key) {
        var hit = (Panel.enums.events || []).filter(function (v) { return v.key === key; })[0];
        return hit ? hit.description : "";
      }
      evDesc.textContent = descFor(evSel.value || t.event);
      body.appendChild(field("Event", evSel));
      body.appendChild(evDesc);

      var filterHost = ce("div", {});
      body.appendChild(filterHost);
      var filterReaders = {};
      function renderFilters(ev) {
        filterHost.innerHTML = "";
        filterReaders = {};
        (EVENT_FILTERS[ev] || []).forEach(function (fk) {
          var kind = FILTER_KINDS[fk];
          var input = kindInput(kind, t[fk]);
          filterHost.appendChild(field(FIELD_LABELS[fk] || ("Filter: " + fk + " (empty = any)"), input.el || input));
          filterReaders[fk] = { kind: kind, input: input };
        });
      }
      renderFilters(evSel.value || t.event);
      evSel.addEventListener("change", function () {
        evDesc.textContent = descFor(evSel.value);
        renderFilters(evSel.value);
        markDirty();
      });

      // conditions drawer
      var cond = t.conditions || {};
      var cBody = ce("div", { style: "display:none;padding:4px 0 6px 10px;" });
      var cHead = ce("div", { style: "cursor:pointer;color:var(--gold-dim);font-size:11px;margin-top:6px;", text: "▸ Conditions (gate when this trigger may fire)" });
      cHead.addEventListener("click", function () {
        var open = cBody.style.display !== "none";
        cBody.style.display = open ? "none" : "";
        cHead.textContent = (open ? "▸" : "▾") + " Conditions (gate when this trigger may fire)";
      });
      var has = chips(cond.has, "q-token-dl");
      cBody.appendChild(field("Has ALL tokens", has.el));
      var missing = chips(cond.missing, "q-token-dl");
      cBody.appendChild(field("Missing ALL tokens", missing.el));
      var inRoom = kindInput("room", cond.in_room);
      cBody.appendChild(field("In room", inRoom.el));
      var hasItem = numInput(cond.has_item, "q-item-dl");
      cBody.appendChild(field("Has item", hasItem));
      var missItem = numInput(cond.missing_item, "q-item-dl");
      cBody.appendChild(field("Missing item", missItem));
      var hasFlag = flagMapRows(cond.has_flag);
      cBody.appendChild(field("Has flags (key = value)", hasFlag.el));
      var missFlag = flagMapRows(cond.missing_flag);
      cBody.appendChild(field("Missing flags (key = value)", missFlag.el));
      var hasGold = numInput(cond.has_gold);
      cBody.appendChild(field("Has gold (at least)", hasGold));
      var hasMw = numInput(cond.has_masterwork);
      cBody.appendChild(field("Has masterwork (craft skill ≥)", hasMw));
      body.appendChild(cHead); body.appendChild(cBody);

      body.appendChild(ce("div", { style: "font-weight:bold;color:var(--gold-dim);font-size:11px;margin-top:8px;", text: "Actions run in ORDER when the trigger fires:" }));
      var acts = actionList(t.actions, body, true);

      return { gather: function () {
        var filters = {};
        for (var fk in filterReaders) {
          filters[fk] = readKind(filterReaders[fk].kind, filterReaders[fk].input);
        }
        // Filters another event offers go with that event.
        var offered = EVENT_FILTERS[evSel.value] || [];
        var dropped = [];
        for (var ev in EVENT_FILTERS) {
          EVENT_FILTERS[ev].forEach(function (f) {
            if (offered.indexOf(f) < 0 && dropped.indexOf(f) < 0) dropped.push(f);
          });
        }
        var hasTokens = has.get(), missTokens = missing.get();
        var hf = hasFlag.get(), mf = missFlag.get();
        var c = {
          has: hasTokens.length ? hasTokens : undefined,
          missing: missTokens.length ? missTokens : undefined,
          in_room: orUndef(inRoom.read()),
          has_item: orUndef(toInt(hasItem.value)),
          missing_item: orUndef(toInt(missItem.value)),
          has_flag: Object.keys(hf).length ? hf : undefined,
          missing_flag: Object.keys(mf).length ? mf : undefined,
          has_gold: orUndef(toInt(hasGold.value)),
          has_masterwork: orUndef(toInt(hasMw.value))
        };
        return QM.trigger(t, { event: evSel.value, actions: acts.get(), filters: filters,
          dropped: dropped, conditions: c });
      } };
    }, true);
  }

  function flagMapRows(m) {
    var box = ce("div", {});
    function row(k, v) {
      var key = strPick(k, "q-flagkey-dl");
      var val = ce("input", { type: "text", placeholder: "value", style: "max-width:110px;" });
      val.value = v || "";
      val.addEventListener("input", markDirty);
      var rm = ce("button", { "class": "mini rm", text: "×" });
      var wrap = ce("div", {}, [key, val, rm]);
      rm.addEventListener("click", function () { box.removeChild(wrap); markDirty(); });
      wrap._kv = function () { return key.value.trim() ? [key.value.trim(), val.value] : null; };
      box.insertBefore(wrap, addBtn);
    }
    var addBtn = ce("button", { "class": "mini", text: "+ flag" });
    addBtn.addEventListener("click", function () { row("", ""); markDirty(); });
    box.appendChild(addBtn);
    if (m) for (var k in m) row(k, m[k]);
    return {
      el: box,
      get: function () {
        var out = {};
        Array.prototype.forEach.call(box.children, function (c) {
          if (c._kv) { var kv = c._kv(); if (kv) out[kv[0]] = kv[1]; }
        });
        return out;
      }
    };
  }

  // ---- detail ----------------------------------------------------------

  Panel.renderDetail = function (obj) {
    Panel.enums = (obj && obj.enums) || {};
    var insp = document.getElementById("inspector");
    insp.innerHTML = "";
    Panel.dirty = false;
    Panel._roomSelects = [];

    if (!obj || !obj.found) {
      insp.appendChild(ce("div", { "class": "empty", text: "Quest not found." }));
      return;
    }
    var q = obj.quest || {};
    Panel.selectedId = q.questid || Panel.selectedId;

    insp.appendChild(ce("h2", { text: "Quest #" + q.questid }));
    insp.appendChild(ce("div", { style: "font-size:11px;color:var(--gold-dim);margin:2px 0 8px;",
      text: "Saves are live immediately: no reboot. First save of a hand-authored quest produces a one-time formatting diff (canonicalization); every save after is minimal." }));

    // datalists
    var dls = [
      ["q-token-dl", (Panel.enums.questTokens || []).map(function (t) { return { v: t.token, t: t.questName }; })],
      ["q-item-dl", ((window.Builder && window.Builder.itemRows) || []).map(function (r) { return { v: String(r.id), t: r.name || "" }; })],
      ["q-mob-dl", (Panel.mobRows || []).map(function (r) { return { v: String(r.id), t: (r.name || "") + " (" + (r.zone || "no zone") + ")" }; })],
      ["q-condition-dl", (Panel.enums.statusConditions || []).map(function (b) { return { v: String(b.id), t: b.name }; })],
      ["q-spell-dl", (Panel.enums.spells || []).map(function (s) { return { v: s.id, t: s.name }; })],
      ["q-recipe-dl", (Panel.enums.recipes || []).map(function (r) { return { v: r, t: "" }; })],
      ["q-faction-dl", (Panel.enums.factions || []).map(function (f) { return { v: f.id, t: f.name }; })],
      ["q-skill-dl", (Panel.enums.skills || []).map(function (s) { return { v: s, t: "" }; })],
      ["q-stat-dl", (Panel.enums.stats || []).map(function (s) { return { v: s, t: "" }; })]
    ];
    dls.forEach(function (d) {
      var dl = ce("datalist", { id: d[0] });
      d[1].forEach(function (o) { dl.appendChild(ce("option", { value: o.v, text: o.t })); });
      insp.appendChild(dl);
    });
    var flagDl = ce("datalist", { id: "q-flagkey-dl" });
    var fk = Panel.enums.flagKeys || {};
    for (var k in fk) flagDl.appendChild(ce("option", { value: k, text: (fk[k] || []).join("/") }));
    insp.appendChild(flagDl);

    insp.appendChild(ce("div", { id: "q-warnings", style: "color:#c90;font-size:12px;white-space:pre-wrap;" }));
    insp.appendChild(ce("div", { id: "q-errors", style: "color:var(--danger);font-size:12px;white-space:pre-wrap;" }));

    // ---- identity ----
    insp.appendChild(sectionTitle("Identity"));
    var name = textInput(q.name);
    insp.appendChild(field("Name", name, "renaming moves the file"));
    var desc = textArea(q.description);
    insp.appendChild(field("Description (quest log blurb)", desc));
    var secret = boolInput(q.secret);
    insp.appendChild(field("Secret (tracks progress without showing the player)", secret));
    var repeatable = boolInput(q.repeatable);
    insp.appendChild(field("Repeatable", repeatable));
    var cooldown = numInput(q.cooldown_rounds);
    insp.appendChild(field("Cooldown rounds (repeatable only)", cooldown));

    // ---- steps ----
    insp.appendChild(sectionTitle("Steps"));
    insp.appendChild(ce("div", { style: "font-size:11px;color:var(--gold-dim);margin:2px 0 6px;",
      text: "Step order is the order within a path. A step with a branch belongs to that side only; leave the branch blank for a step every side takes. A new step's destination comes from its triggers, or is set in the quest's YAML file; the destination shown under each step is read only here." }));
    var stepBox = ce("div", {});
    // gatherFlags reads the flag rows as the form stands; before the flag rows
    // exist (the steps are built first) it answers with the loaded flags.
    var flagBox = null;
    function gatherFlags() {
      if (!flagBox) return q.flags || [];
      var flags = [];
      Array.prototype.forEach.call(flagBox.children, function (r) {
        if (r._gather) { var f = r._gather(); if (f.key) flags.push(f); }
      });
      return flags;
    }
    insp.appendChild(stepBox);
    function stepRow(s) {
      s = s || {};
      collapsible(stepBox, function () { return "step: " + (s.id || "(new)"); }, function (body) {
        var id = textInput(s.id);
        body.appendChild(field("Id", id));
        var sd = textArea(s.description);
        body.appendChild(field("Description (quest log, after this step is granted)", sd));
        var hint = textArea(s.hint);
        body.appendChild(field("Hint (the quest log's nudge)", hint));
        // Branch: a value of one of this quest's flags, read from the flag
        // rows as they stand now (refilled when the picker opens). A loaded
        // value no flag offers stays selectable, so a save does not lose it.
        var branch = ce("select", {});
        function fillBranch() {
          // The loaded branch only seeds the first fill; after that the
          // picker's own value stands, so choosing "(none)" sticks.
          var keep = branch.options.length ? branch.value : (s.branch || "");
          var vals = QM.branchValues(gatherFlags());
          if (keep && vals.indexOf(keep) < 0) vals.push(keep);
          branch.innerHTML = "";
          [""].concat(vals).forEach(function (v) {
            var o = ce("option", { value: v, text: v || "(none: every side)" });
            if (v === keep) o.selected = true;
            branch.appendChild(o);
          });
        }
        fillBranch();
        branch.addEventListener("focus", fillBranch);
        branch.addEventListener("mousedown", fillBranch);
        branch.addEventListener("change", markDirty);
        body.appendChild(field("Branch (the side this step belongs to)", branch,
          "a value of one of this quest's flags; blank = every side takes the step"));
        body.appendChild(ce("div", { style: "font-size:11px;color:var(--gold-dim);margin-top:6px;",
          text: QM.destinationText(s.destination) }));
        return { gather: function () {
          return QM.step(s, { id: id.value.trim(), description: sd.value, hint: hint.value,
            branch: branch.value });
        } };
      }, true);
    }
    (q.steps || []).forEach(stepRow);
    var addStep = ce("button", { "class": "mini", text: "+ step" });
    addStep.addEventListener("click", function () { stepRow(null); markDirty(); });
    insp.appendChild(addStep);

    // ---- rewards ----
    insp.appendChild(sectionTitle("Rewards (on completion)"));
    var rw = q.rewards || {};
    var rGold = numInput(rw.gold);
    insp.appendChild(field("Gold", rGold));
    var rItem = numInput(rw.itemid, "q-item-dl");
    insp.appendChild(field("Item", rItem));
    var rCondition = numInput(rw.conditionid, "q-condition-dl");
    insp.appendChild(field("Status condition", rCondition));
    var rSpell = strPick(rw.spellid, "q-spell-dl");
    insp.appendChild(field("Spell taught", rSpell));
    var rSkill = pairRows(rw.skillinfo, "q-skill-dl", "skill", "level");
    insp.appendChild(field("Skills (skill → level)", rSkill.el));
    var rStat = pairRows(rw.stat_info, "q-stat-dl", "stat", "amount");
    insp.appendChild(field("Stats (stat → amount)", rStat.el));
    var rRecipe = chips((rw.recipe_info || "").split(",").map(function (s) { return s.trim(); }).filter(Boolean), "q-recipe-dl");
    insp.appendChild(field("Recipes granted", rRecipe.el));
    var rItems = pairRows(rw.item_info, "q-item-dl", "item id", "qty");
    insp.appendChild(field("Item stockpile (item → qty)", rItems.el));
    var rPmsg = textArea(rw.actor);
    insp.appendChild(field("Player message", rPmsg));
    var rRmsg = textArea(rw.observer);
    insp.appendChild(field("Room message", rRmsg));
    var rRoom = numInput(rw.roomid);
    insp.appendChild(field("Move player to room (0 = none)", ce("span", {}, [rRoom, roomAssist(rRoom)])));
    var rChain = strPick(rw.questid, "q-token-dl");
    insp.appendChild(field("Chain quest token (granted on completion)", rChain));
    var rFaction = strPick(rw.rep_faction, "q-faction-dl");
    insp.appendChild(field("Rep faction", rFaction));
    var rRep = numInput(rw.rep_amount);
    insp.appendChild(field("Rep amount", rRep));

    // ---- flags ----
    insp.appendChild(sectionTitle("Flags (branch tracking)"));
    insp.appendChild(ce("div", { style: "font-size:11px;color:var(--gold-dim);margin:2px 0 6px;",
      text: "Declare every flag this quest's triggers or NPC dialogue set/gate on: an undeclared reference is refused at save. Dialogue setsQuestFlag keys must match these (full key: " + q.questid + "-<key>)." }));
    flagBox = ce("div", {});
    insp.appendChild(flagBox);
    function flagRow(f) {
      f = f || {};
      collapsible(flagBox, function () { return "flag: " + (f.key || "(new)"); }, function (body) {
        var key = textInput(f.key);
        body.appendChild(field("Key (short form; the full key is " + q.questid + "-<key>)", key));
        var vals = chips(f.values);
        body.appendChild(field("Allowed values", vals.el));
        var fd = textInput(f.description);
        body.appendChild(field("Description", fd));
        return { gather: function () {
          return QM.merge(f, { key: key.value.trim(), values: vals.get(), description: fd.value.trim() });
        } };
      });
    }
    (q.flags || []).forEach(flagRow);
    var addFlag = ce("button", { "class": "mini", text: "+ flag" });
    addFlag.addEventListener("click", function () { flagRow(null); markDirty(); });
    insp.appendChild(addFlag);

    // ---- triggers ----
    insp.appendChild(sectionTitle("Triggers (the quest's machinery)"));
    insp.appendChild(ce("div", { style: "font-size:11px;color:var(--gold-dim);margin:2px 0 6px;",
      text: "Each trigger: an event, optional filters and conditions, and an ordered action list. Dialogue-granted steps live in the NPC's dialogue file, not here: triggers cover everything else (deliveries, kills, arrivals, chained set-pieces)." }));
    var trigBox = ce("div", {});
    insp.appendChild(trigBox);
    (q.triggers || []).forEach(function (t) { triggerRow(trigBox, t); });
    var addTrig = ce("button", { "class": "mini", text: "+ trigger" });
    addTrig.addEventListener("click", function () { triggerRow(trigBox, null); markDirty(); });
    insp.appendChild(addTrig);

    // ---- save / delete ----
    var save = ce("button", { id: "q-save", text: "Save quest" });
    save.addEventListener("click", function () {
      Panel.saving = true;
      gmcp("Build.Quest.Update", gatherFile());
    });
    var del = ce("button", { "class": "mini rm", text: "Delete quest", style: "margin-left:12px;" });
    del.addEventListener("click", function () {
      if (!window.confirm("Delete quest #" + q.questid + "? Anything referencing it (dialogue grants, other quests) blocks the delete and will be listed.")) return;
      Panel.deleting = true;
      gmcp("Build.Quest.Delete", { questId: q.questid });
    });
    insp.appendChild(ce("div", { "class": "save-row", style: "margin-top:12px;display:flex;align-items:center;" }, [save, ce("span", { style: "flex:1 1 auto;" }), del]));

    function gatherFile() {
      var steps = [];
      Array.prototype.forEach.call(stepBox.children, function (r) { if (r._gather) steps.push(r._gather()); });
      var triggers = [];
      Array.prototype.forEach.call(trigBox.children, function (r) { if (r._gather) triggers.push(r._gather()); });
      // Overlay the controls on the quest as loaded, so a field with no
      // control (a step's destination, repeat_limit, rewards.summary and
      // surprise, anything added later) survives the save.
      return QM.merge(q, {
        questid: q.questid, name: name.value.trim(), description: desc.value,
        secret: secret.checked, repeatable: repeatable.checked,
        cooldown_rounds: toInt(cooldown.value),
        steps: steps, flags: gatherFlags(), triggers: triggers,
        rewards: QM.merge(q.rewards, {
          gold: toInt(rGold.value), itemid: toInt(rItem.value), conditionid: toInt(rCondition.value),
          spellid: rSpell.value.trim(), skillinfo: rSkill.get(), stat_info: rStat.get(),
          recipe_info: rRecipe.get().join(","), item_info: rItems.get(),
          actor: rPmsg.value, observer: rRmsg.value,
          roomid: toInt(rRoom.value), questid: rChain.value.trim(),
          rep_faction: rFaction.value.trim(), rep_amount: toInt(rRep.value)
        })
      });
    }
  };

  // ---- GMCP feeds ------------------------------------------------------

  Panel.onResult = function (res) {
    var errEl = document.getElementById("q-errors");
    var warnEl = document.getElementById("q-warnings");
    if (res && res.ok) {
      if (Panel.deleting) { Panel.deleting = false; toast("Quest deleted.", false); gmcp("Build.Quest.List", {}); return; }
      Panel.saving = false; Panel.dirty = false;
      toast("Quest saved: live immediately, no reboot.", false);
      if (warnEl) warnEl.textContent = (res.warnings && res.warnings.length)
        ? ("Warnings:\n" + res.warnings.join("\n")) : "";
      if (errEl) errEl.textContent = "";
      gmcp("Build.Quest.List", {});
      return;
    }
    Panel.saving = false; Panel.deleting = false;
    var msg = (res && res.error) || "Quest error";
    if (errEl) errEl.textContent = msg;
    toast("Quest refused: see the errors above the form.", true);
  };

  Panel.onRoomList = function (rows) {
    if (Panel._pendingRoomZone) {
      Panel.roomsByZone[Panel._pendingRoomZone] = rows || [];
      var z = Panel._pendingRoomZone;
      Panel._pendingRoomZone = "";
      Panel._roomSelects.forEach(function (rs) {
        if (rs.zoneSel.value === z) rs.fill(z);
      });
    }
  };

  Panel.onMobList = function (rows) { Panel.mobRows = rows || []; };
  Panel.onZoneList = function (obj) { Panel.zoneRows = (obj && obj.zones) || []; };

  window.Builder = window.Builder || {};
  window.Builder.QuestsPanel = Panel;
})();
