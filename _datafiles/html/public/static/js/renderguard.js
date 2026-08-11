// renderguard.js — skip DOM rebuilds that would not change anything.
//
// Review finding 19. Several web-client panels tear their contents down and
// rebuild them from scratch on every GMCP push:
//
//   updateStatusPanel   container.innerHTML = html            (Char.Conditions)
//   renderQuests        host.innerHTML = html                 (Char.Quests)
//   renderInventory     removeChild loop, then rebuild every  (Char.Inventory)
//                       tile, icon and label
//
// Char.Conditions ships alongside Char.Vitals, so the status panel was being
// destroyed and recreated every round whether or not a single condition had
// changed -- which is the common case, because conditions change rarely and
// vitals change constantly. Inventory is the largest subtree and changes least
// often of all.
//
// The fix is deliberately NOT a node-diffing renderer. The cheapest correct
// change is to notice that the output is identical to what is already on screen
// and do nothing, which is trivially state-preserving: if the markup we were
// about to write matches the markup already there, not writing it cannot change
// what the player sees.
//
// It also fixes things the rebuild was quietly breaking: a tooltip being read,
// a hover state, and keyboard focus inside a panel all survive now, because the
// nodes holding them are no longer replaced underneath the user.
//
// Signatures are held against the ELEMENT, not in a map keyed by panel name. If
// a panel is torn down and recreated (a window reopened, a layout reset), the
// fresh element carries no signature and renders normally, with no cache to
// invalidate by hand.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.RenderGuard = api;    // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {

    var HAS_WEAKMAP = (typeof WeakMap === 'function');
    var store = HAS_WEAKMAP ? new WeakMap() : null;

    // Fallback for environments without WeakMap: a non-enumerable-ish property.
    // Prefixed to make its origin obvious in a debugger.
    var PROP = '__renderGuardSignature';

    function readSig(el) {
        if (!el) return undefined;
        if (store) return store.get(el);
        return el[PROP];
    }

    function writeSig(el, sig) {
        if (!el) return;
        if (store) { store.set(el, sig); return; }
        el[PROP] = sig;
    }

    // changed reports whether this signature differs from the last one recorded
    // for the element, and records it.
    //
    // Call it when the rendered output is expensive to build: compute a cheap
    // signature of the INPUTS first, and skip the build entirely when it
    // matches. Returns true when the caller should render.
    function changed(el, signature) {
        if (!el) return true;                       // nowhere to remember; always render
        if (readSig(el) === signature) return false;
        writeSig(el, signature);
        return true;
    }

    // setHTML assigns innerHTML only when it differs from the last value this
    // guard wrote for the element. Returns true when it wrote.
    //
    // Compares against the recorded signature rather than reading back
    // el.innerHTML: the browser normalises markup on write, so a read-back
    // comparison reports a difference on every call for input that never
    // changed, which is the opposite of the point.
    function setHTML(el, html) {
        if (!el) return false;
        if (readSig(el) === html) return false;
        writeSig(el, html);
        el.innerHTML = html;
        return true;
    }

    // forget drops an element's signature, forcing the next render. For a caller
    // that mutates a panel's DOM behind the guard's back.
    function forget(el) {
        if (!el) return;
        if (store) { store.delete(el); return; }
        delete el[PROP];
    }

    return {
        changed: changed,
        setHTML: setHTML,
        forget: forget
    };
}));
