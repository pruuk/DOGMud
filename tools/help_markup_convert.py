"""One-time conversion of DOGMud help files to the help markup (plan 4, 2026-11-12).

Reads every player help template (the live world and every module), writes the converted
copy under OUT at the same relative path, and prints a report. It never writes into the
repo: land its output with cp (global rule H3). A file whose words would change is written
unchanged and listed. Spots it cannot convert safely are left as they are and listed under
FLAGGED; Task 5 converts them by hand. The guard (help_markup_guard_test.go) is the judge.

usage: python -I tools/help_markup_convert.py <repo> <out>
"""
import glob, os, re, sys

TAG = re.compile(r"</?ansi[^>]*>")
GAP = re.compile(r"(?<=\S) {2,}(?=\S)")
LIST = re.compile(r"^(- |\* |\d+\. )")
COLONLABEL = re.compile(r"^([A-Z][^:]{0,28}:) (?=\S)")
DASHROW = re.compile(r'^(\s*<ansi fg="(?:command|skill)">[^<]*</ansi>) - (?=\S)')
FIRSTSPAN = re.compile(r"^<ansi[^>]*>([^<]*)</ansi>")
LONEUNIT = re.compile(r'^\s*<ansi fg="(command|skill)">[^<]*</ansi>\s*$')
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
        return "para"
    if LIST.match(body):
        return "item"
    if LONEUNIT.match(line):
        return "example"
    if DASHROW.match(line):
        return "dashrow"
    if COLONLABEL.match(body):
        return "colonrow"
    return "iprose"


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
    out, i, n = [], 0, len(lines)

    def flag(j, why):
        flags.append("%s:%d %s" % (name, j + 1, why))

    def entry_block(i, starts, marker, transform, seealso=False):
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
                    elif marker == "::rows" and ind == base and k == "iprose":
                        block.append("  " + lines[i])  # same-indent continuation, made deeper
                    elif seealso and ind == 0 and k == "para":
                        block.append(" " * (base + 2) + lines[i])  # a See also list's next line
                    else:
                        if marker == "::list" and ind == base and k == "iprose":
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
            i = entry_block(i, {"row", "colonrow", "dashrow"}, "::rows", row_t)
        elif k == "seealso":
            i = entry_block(i, {"seealso"}, "::rows", seealso_t, seealso=True)
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
        else:  # para, iprose: a paragraph run
            j = i
            ind = indent(lines[i])
            while j < n and kinds[j] in ("para", "iprose"):
                if j > i and indent(lines[j]) != ind:
                    if not vis(lines[j - 1]).rstrip().endswith(":"):
                        flag(j, "the indent changes inside a paragraph")
                    break
                if RULE.search(vis(lines[j])) and j > i:
                    flag(j, "a heading inside a paragraph")
                j += 1
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
