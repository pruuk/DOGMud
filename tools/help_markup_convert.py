"""One-time conversion of DOGMud help files to the help markup (plan 4, 2026-11-12).

Reads every player help template (the live world and every module), writes the converted
copy under OUT at the same relative path, and prints a report. It never writes into the
repo: land its output with cp (global rule H3). A file whose words would change is written
unchanged and listed. Spots it cannot convert safely are left as they are and listed under
FLAGGED; Task 5 converts them by hand. The guard (help_markup_guard_test.go) is the judge.

A row's continuation is a line at the row's text column (where its text starts after the
label's gap). A line anywhere else is never glued onto the row above: it ends the block and
is flagged. A paragraph line that ends without punctuation above a line starting with a
capital or a digit is flagged too ("line break may be meaningful"), never changed.

usage: python -I tools/help_markup_convert.py <repo> <out>
"""
import glob, os, re, sys

TAG = re.compile(r"</?ansi[^>]*>")
GAP = re.compile(r"(?<=\S) {2,}(?=\S)")
LIST = re.compile(r"^(- |\* |\d+\. )")
COLONLABEL = re.compile(r"^([A-Z][^:]{0,28}:) (?=\S)")
COLONGAP = re.compile(r"^[A-Z][^:]{0,28}: {2,}\S")
# The text of a colour span: anything but an ansi tag, so "set x <a|b>" is one span.
SPANTEXT = r"(?:[^<]|<(?!/?ansi\b)[^>]*>)*"
DASHROW = re.compile(r'^(\s*<ansi fg="(?:command|skill)">' + SPANTEXT + r'</ansi>) - (?=\S)')
FIRSTSPAN = re.compile(r"^<ansi[^>]*>(" + SPANTEXT + r")</ansi>")
LONEUNIT = re.compile(r'^\s*<ansi fg="(command|skill)">' + SPANTEXT + r'</ansi>\s*$')
LABELSPAN = re.compile(r'^\s*<ansi fg="(command|skill)">')
# A line that ends on one of these is not relying on its break.
ENDPUNCT = tuple(".!?:,;)]}\"'”’")
ART = re.compile(r"[┌┐└┘╔╗╚╝│║╒╕╲╱█░]")
RULE = re.compile(r"[━─]")
LOGIC = re.compile(r"\{\{-?\s*(range|with|if)\b")
MARKERS = ("::rows", "::list", "::keep", "::")


def vis(s):
    return TAG.sub("", s)


def indent(s):
    v = vis(s)
    return len(v) - len(v.lstrip(" "))


def classify(line):
    v = vis(line)
    if not v.strip():
        return "blank"
    ind = indent(line)
    body = v[ind:]
    if ART.search(v):
        return "art"
    if ind == 0 and (v.startswith(".:") or RULE.search(v)):
        return "para"
    gaps = GAP.findall(body.rstrip())
    if len(gaps) >= 2:
        return "table"
    if gaps:
        if ind > 0:
            return "row"
        m = FIRSTSPAN.match(line)
        if m and GAP.search(body).start() <= len(m.group(1).rstrip()):
            return "row"
    if ind == 0:
        # A bare "See Also:" label (nothing after the colon) leads a list below it: a paragraph.
        if body.lower().startswith("see also:") and body[len("see also:"):].strip():
            return "seealso"
        # "Label: value" at column 0: a row only in a run of two or more (see convert).
        m = COLONLABEL.match(body)
        if m and len(m.group(1).split()) <= 3:
            return "colon0"
        return "para"
    if LIST.match(body):
        return "item"
    if LONEUNIT.match(line):
        return "example"
    if DASHROW.match(line):
        return "dashrow"
    m = COLONLABEL.match(body)
    if m:
        # A label of more than three words is a sentence with a colon in it.
        return "colonrow" if len(m.group(1).split()) <= 3 else "colonsentence"
    return "iprose"


def text_col(line):
    """The visible column where a row's text starts, after its label's gap; None without one."""
    v = vis(line)
    ind = indent(line)
    m = GAP.search(v[ind:].rstrip())
    return ind + m.end() if m else None


def reindent(line, col):
    """line moved to start at visible column col, or None when its indent sits inside a tag."""
    k = len(line) - len(line.lstrip(" "))
    if k != indent(line):
        return None
    return " " * col + line[k:]


def unpunctuated_break(a, b):
    """a ends without punctuation and b starts with a capital or a digit: the break may matter."""
    va, vb = vis(a).rstrip(), vis(b).lstrip()
    return bool(va) and not va.endswith(ENDPUNCT) and bool(re.match(r"[A-Z0-9]", vb))


def widen_after_colon(line):
    """'Label: text' becomes 'Label:  text' (the gap a row needs). The space goes right after
    the colon's visible character, inside its colour span if it has one."""
    v = vis(line)
    ind = indent(line)
    m = COLONLABEL.match(v[ind:])
    target = ind + len(m.group(1))  # visible index just after the colon
    seen = 0
    i = 0
    while i < len(line):
        t = TAG.match(line, i)
        if t:
            i = t.end()
            continue
        seen += 1
        i += 1
        if seen == target:
            return line[:i] + " " + line[i:]
    return line


def convert(lines, name, flags):
    kinds = [classify(l) for l in lines]
    n = len(lines)

    def colonish(j):
        # a column-0 "Label: value" line, with one space after the colon or already a gap
        return 0 <= j < n and (kinds[j] in ("colon0", "colonrow") or (
            kinds[j] == "row" and indent(lines[j]) == 0 and COLONGAP.match(vis(lines[j]))))

    for j in range(n):
        # "Label: value" at column 0 is a row only in a run of two or more.
        if kinds[j] == "colon0":
            kinds[j] = "colonrow" if colonish(j - 1) or colonish(j + 1) else "para"
        # A lone command after a line ending in "," finishes that sentence: paragraph text.
        if kinds[j] == "example" and j > 0 and vis(lines[j - 1]).rstrip().endswith(","):
            kinds[j] = "iprose"
    out, i = [], 0

    def flag(j, why):
        flags.append("%s:%d %s" % (name, j + 1, why))

    for j in range(n):
        if kinds[j] == "colonsentence":
            flag(j, "colon sentence, check")

    def entry_block(i, starts, marker, transform):
        """A ::list block: an item at the block's indent, deeper lines continue it."""
        base = indent(lines[i])
        block = []
        while i < n:
            if kinds[i] in starts and indent(lines[i]) == base:
                block.append(transform(lines[i], kinds[i]))
                i += 1
                while i < n and kinds[i] != "blank":
                    k, ind = kinds[i], indent(lines[i])
                    if k in starts and ind == base:
                        break
                    if ind > base and k != "item":
                        block.append(lines[i])  # deeper: the entry continues
                    else:
                        if ind == base and k == "iprose":
                            flag(i, "a same-indent line after a list item")
                        break
                    i += 1
                # the next entry follows at once: it joins this block
                if i < n and kinds[i] in starts and indent(lines[i]) == base:
                    continue
                # a blank line followed by another entry at the same indent stays in the block
                if i + 1 < n and kinds[i] == "blank" and kinds[i + 1] in starts and indent(lines[i + 1]) == base:
                    block.append(lines[i])
                    i += 1
                    continue
                break
            else:
                break
        out.append(marker)
        out.extend(block)
        out.append("::")
        return i

    def rows_block(i, starts, transform, seealso=False):
        """A ::rows block. Returns (next line, block lines, entries, flags). An entry's
        continuation is a line at its text column. A deeper continuation whose lines share
        one indent is moved to the text column (flagged); any other line ends the block
        and is flagged, never glued onto the row above."""
        base = indent(lines[i])
        block, fl, entries = [], [], 0

        def is_start(j):
            return kinds[j] in starts and indent(lines[j]) == base

        def labelish(j):
            # A same-indent line that starts like a label (a command or skill span, "{")
            # is a new entry, even without a gap of its own.
            if indent(lines[j]) != base or kinds[j] not in ("iprose", "para", "colonsentence"):
                return False
            vb = vis(lines[j]).lstrip(" ")
            return bool(LABELSPAN.match(lines[j])) or (vb.startswith("{") and not vb.startswith("{{"))

        while i < n and (is_start(i) or (block and labelish(i))):
            if is_start(i):
                head = transform(lines[i], kinds[i])
            else:
                head = lines[i]
                fl.append((i, "row label without a two-space gap"))
            block.append(head)
            entries += 1
            tc = text_col(head)
            first = i
            i += 1
            cand = []
            while i < n and kinds[i] != "blank":
                ind = indent(lines[i])
                if ind < base or is_start(i) or kinds[i] in ("item", "table", "art"):
                    break
                if ind == base and not (seealso and base == 0 and kinds[i] == "para"):
                    break
                cand.append(i)
                i += 1
            if cand and tc is not None:
                fixed = {}
                for c in cand:
                    if seealso and base == 0 and indent(lines[c]) == 0:
                        fixed[c] = " " * tc + lines[c]  # a See also list's next line
                deeper = [c for c in cand if c not in fixed]
                inds = set(indent(lines[c]) for c in deeper)
                if deeper and inds != {tc} and len(inds) == 1:
                    moved = {c: reindent(lines[c], tc) for c in deeper}
                    if all(v is not None for v in moved.values()):
                        fixed.update(moved)
                        fl.append((deeper[0], "row continuation re-indented to the text column"))
                stop = None
                for c in cand:
                    line = fixed.get(c, lines[c])
                    if indent(line) != tc:
                        stop = c
                        break
                    block.append(line)
                if stop is not None:
                    fl.append((stop, "row continuation not at the text column"))
                    return stop, block, entries, fl
                text = block[-len(cand):]
                prev = head
                for c, line in zip(cand, text):
                    if unpunctuated_break(prev, line) or (vis(prev).rstrip().endswith(":") and re.match(r"[A-Z0-9]", vis(line).lstrip())):
                        fl.append((c, "line break may be meaningful"))
                        break
                    prev = line
            elif cand:
                block.extend(lines[c] for c in cand)  # an entry already flagged for its gap
            # a same-indent line that is neither an entry nor a See also line: not glued on
            if i < n and indent(lines[i]) == base and kinds[i] in ("iprose", "para", "colonsentence") \
                    and not labelish(i) and not is_start(i):
                fl.append((i, "row continuation not at the text column"))
                return i, block, entries, fl
            # a blank line followed by another entry at the same indent stays in the block
            if i + 1 < n and kinds[i] == "blank" and is_start(i + 1):
                block.append(lines[i])
                i += 1
        return i, block, entries, fl

    def emit_rows(i, starts, transform, seealso=False):
        start = i
        i, block, entries, fl = rows_block(i, starts, transform, seealso)
        # A one-row block straight above a table or art region is that region's header.
        if len(block) == 1 and i < n and kinds[i] in ("table", "art"):
            j = i
            while j < n and kinds[j] != "blank":
                j += 1
            flag(start, "%s: convert by hand" % kinds[i])
            out.extend(lines[start:j])
            return j
        for j, why in fl:
            flag(j, why)
        out.append("::rows")
        out.extend(block)
        out.append("::")
        return i

    def row_t(line, kind):
        if kind == "colonrow":
            return widen_after_colon(line)
        if kind == "dashrow":
            return DASHROW.sub(lambda m: m.group(1) + "  ", line, count=1)
        if kind == "seealso":
            return seealso_t(line, kind)
        return line

    def seealso_t(line, kind):
        return re.sub(r"(?i)(see also:)(</ansi>)? ", lambda m: m.group(1) + (m.group(2) or "") + "  ", line, count=1)

    while i < n:
        k = kinds[i]
        if k == "blank":
            out.append(lines[i])
            i += 1
        elif k in ("row", "colonrow", "dashrow"):
            i = emit_rows(i, {"row", "colonrow", "dashrow"}, row_t)
        elif k == "seealso":
            i = emit_rows(i, {"seealso"}, seealso_t, seealso=True)
        elif k == "item":
            i = entry_block(i, {"item"}, "::list", lambda l, _: l)
        elif k == "example":
            j = i
            while j < n and kinds[j] == "example" and indent(lines[j]) == indent(lines[i]):
                j += 1
            if all(len(vis(l).rstrip()) <= 30 for l in lines[i:j]):
                out.append("::keep")
                out.extend(lines[i:j])
                out.append("::")
            else:
                flag(i, "example lines wider than 30")
                out.extend(lines[i:j])
            i = j
        elif k in ("table", "art"):
            j = i
            while j < n and kinds[j] != "blank":
                j += 1
            flag(i, "%s: convert by hand" % k)
            out.extend(lines[i:j])
            i = j
        else:  # para, iprose, colonsentence: a paragraph run
            j = i
            ind = indent(lines[i])
            while j < n and kinds[j] in ("para", "iprose", "colonsentence"):
                if j > i and indent(lines[j]) != ind:
                    if not vis(lines[j - 1]).rstrip().endswith(":"):
                        flag(j, "the indent changes inside a paragraph")
                    break
                if RULE.search(vis(lines[j])) and j > i:
                    flag(j, "a heading inside a paragraph")
                if j > i and unpunctuated_break(lines[j - 1], lines[j]):
                    flag(j, "line break may be meaningful")
                j += 1
            if j == i:  # a kind no branch takes: keep the line as it is
                j = i + 1
            out.extend(lines[i:j])
            i = j
    return out


def words(text):
    keep = [l for l in text.split("\n") if l not in MARKERS]
    return [w for w in vis("\n".join(keep)).split() if w != "-"]


def main():
    repo, outroot = sys.argv[1], sys.argv[2]
    paths = sorted(glob.glob(os.path.join(repo, "_datafiles/world/dogmud/templates/help/*.template")) +
                   glob.glob(os.path.join(repo, "modules/*/files/datafiles/templates/help/*.template")))
    flags, logic, changed, unchanged, wordfail = [], [], 0, 0, []
    for p in paths:
        rel = os.path.relpath(p, repo)
        raw = open(p, encoding="utf-8", newline="").read()
        text = raw.replace("\r\n", "\n").replace("\r", "")
        name = rel.replace("\\", "/")
        if LOGIC.search(text):
            logic.append(name)
            new = text
        else:
            new = "\n".join(convert(text.split("\n"), name, flags))
            if words(new) != words(text):
                wordfail.append(name)
                new = text
        dst = os.path.join(outroot, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        with open(dst, "w", encoding="utf-8", newline="\n") as f:
            f.write(new)
        if new != raw.replace("\r\n", "\n"):
            changed += 1
        else:
            unchanged += 1
    print("files %d, changed %d, unchanged %d" % (len(paths), changed, unchanged))
    print("LOGIC (convert by hand): %d" % len(logic))
    for x in logic:
        print("  " + x)
    print("WORDS CHANGED (left as they were): %d" % len(wordfail))
    for x in wordfail:
        print("  " + x)
    print("FLAGGED: %d" % len(flags))
    for x in flags:
        print("  " + x)


if __name__ == "__main__":
    main()
