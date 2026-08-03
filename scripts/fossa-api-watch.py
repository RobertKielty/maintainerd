#!/usr/bin/env python3
"""Diff the FOSSA API surface we depend on against the live FOSSA OpenAPI spec.

Compares our committed plugins/fossa/swagger.json against the spec published
at https://app.fossa.com/api/api-docs/swagger.json, restricted to the
endpoints listed in plugins/fossa/fossa-api-watchlist.json. Used by
.github/workflows/fossa-api-watch.yml to catch breaking API changes before
they surface as onboarding failures.
"""
import argparse
import json
import sys
import urllib.request

DEPRECATION_KEYWORDS = ("deprecat", "sunset", "no longer supported", "will be removed")
NOISE_KEYS = {"example", "examples", "operationId", "tags", "summary", "x-readme"}


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def fetch_json(url):
    req = urllib.request.Request(url, headers={"User-Agent": "maintainer-d-fossa-api-watch"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read().decode("utf-8"))


def resolve_refs(node, root, seen=frozenset()):
    if isinstance(node, dict):
        ref = node.get("$ref")
        if isinstance(ref, str) and ref.startswith("#/"):
            if ref in seen:
                return {}
            target = root
            for part in ref[2:].split("/"):
                part = part.replace("~1", "/").replace("~0", "~")
                if not isinstance(target, dict) or part not in target:
                    return {}
                target = target[part]
            return resolve_refs(target, root, seen | {ref})
        return {k: resolve_refs(v, root, seen) for k, v in node.items()}
    if isinstance(node, list):
        return [resolve_refs(v, root, seen) for v in node]
    return node


def strip_noise(node):
    if isinstance(node, dict):
        return {k: strip_noise(v) for k, v in node.items() if k not in NOISE_KEYS}
    if isinstance(node, list):
        return [strip_noise(v) for v in node]
    return node


def get_operation(spec, path, method):
    op = (spec.get("paths") or {}).get(path)
    if not isinstance(op, dict):
        return None
    return op.get(method.lower())


def resolved_operation(spec, path, method):
    op = get_operation(spec, path, method)
    if op is None:
        return None
    return resolve_refs(op, spec)


def index_params(op):
    return {
        (p.get("name"), p.get("in")): p
        for p in (op.get("parameters") or [])
        if isinstance(p, dict)
    }


def diff_parameters(old_op, new_op):
    old_p, new_p = index_params(old_op), index_params(new_op)
    changes = []
    for name, loc in sorted(old_p.keys() - new_p.keys()):
        changes.append(f"parameter removed: {loc} '{name}'")
    for name, loc in sorted(new_p.keys() - old_p.keys()):
        changes.append(f"parameter added: {loc} '{name}'")
    for key in sorted(old_p.keys() & new_p.keys()):
        name, _loc = key
        o, n = old_p[key], new_p[key]
        if bool(o.get("required")) != bool(n.get("required")):
            changes.append(f"parameter '{name}' required changed: {o.get('required')} -> {n.get('required')}")
        ot, nt = (o.get("schema") or {}).get("type"), (n.get("schema") or {}).get("type")
        if ot != nt:
            changes.append(f"parameter '{name}' type changed: {ot} -> {nt}")
    return changes


def diff_schema(old_schema, new_schema, label="schema"):
    changes = []
    old_present, new_present = old_schema is not None, new_schema is not None
    if old_present != new_present:
        changes.append(f"{label} {'removed' if old_present else 'added'}")
        return changes
    if not old_present or not isinstance(old_schema, dict) or not isinstance(new_schema, dict):
        return changes

    ot, nt = old_schema.get("type"), new_schema.get("type")
    if ot != nt:
        changes.append(f"{label} type changed: {ot} -> {nt}")

    oe, ne = old_schema.get("enum"), new_schema.get("enum")
    if oe is not None or ne is not None:
        oe_set, ne_set = set(oe or []), set(ne or [])
        if oe_set != ne_set:
            added, removed = sorted(ne_set - oe_set, key=str), sorted(oe_set - ne_set, key=str)
            msg = f"{label} enum changed"
            if added:
                msg += f", added={added}"
            if removed:
                msg += f", removed={removed}"
            changes.append(msg)

    oreq, nreq = set(old_schema.get("required") or []), set(new_schema.get("required") or [])
    if oreq != nreq:
        changes.append(f"{label} required fields changed: {sorted(oreq)} -> {sorted(nreq)}")

    oprops, nprops = old_schema.get("properties") or {}, new_schema.get("properties") or {}
    for prop in sorted(oprops.keys() - nprops.keys()):
        changes.append(f"{label}.{prop} removed")
    for prop in sorted(nprops.keys() - oprops.keys()):
        changes.append(f"{label}.{prop} added")
    for prop in sorted(oprops.keys() & nprops.keys()):
        changes.extend(diff_schema(oprops[prop], nprops[prop], f"{label}.{prop}"))

    if "items" in old_schema or "items" in new_schema:
        changes.extend(diff_schema(old_schema.get("items"), new_schema.get("items"), f"{label}[]"))

    return changes


def get_json_schema(content):
    if not isinstance(content, dict):
        return None
    app_json = content.get("application/json")
    if isinstance(app_json, dict):
        return app_json.get("schema")
    return None


def diff_request_body(old_op, new_op):
    o = get_json_schema((old_op.get("requestBody") or {}).get("content"))
    n = get_json_schema((new_op.get("requestBody") or {}).get("content"))
    return diff_schema(o, n, "requestBody")


def diff_responses(old_op, new_op):
    changes = []
    o_resp, n_resp = old_op.get("responses") or {}, new_op.get("responses") or {}
    for code in sorted(set(o_resp) - set(n_resp)):
        changes.append(f"response {code} removed")
    for code in sorted(set(n_resp) - set(o_resp)):
        changes.append(f"response {code} added")
    for code in sorted(set(o_resp) & set(n_resp)):
        o_content = o_resp[code].get("content") if isinstance(o_resp[code], dict) else None
        n_content = n_resp[code].get("content") if isinstance(n_resp[code], dict) else None
        changes.extend(diff_schema(get_json_schema(o_content), get_json_schema(n_content), f"response[{code}]"))
    return changes


def find_deprecation_signals(node, label=""):
    signals = set()
    if isinstance(node, dict):
        if node.get("deprecated") is True:
            signals.add(f"{label or 'operation'}: deprecated=true")
        if node.get("x-internal") is True:
            signals.add(f"{label or 'operation'}: x-internal=true")
        for key in ("description", "summary"):
            text = node.get(key)
            if isinstance(text, str) and any(kw in text.lower() for kw in DEPRECATION_KEYWORDS):
                signals.add(f"{label or 'operation'}.{key} mentions deprecation: {text.strip()[:200]}")
        for k, v in node.items():
            signals |= find_deprecation_signals(v, f"{label}.{k}" if label else k)
    elif isinstance(node, list):
        for i, v in enumerate(node):
            signals |= find_deprecation_signals(v, f"{label}[{i}]")
    return signals


def diff_documented_endpoint(old_spec, new_spec, method, path):
    old_resolved = resolved_operation(old_spec, path, method)
    new_resolved = resolved_operation(new_spec, path, method)

    if old_resolved is None and new_resolved is None:
        return None
    if old_resolved is None and new_resolved is not None:
        return ["endpoint appeared (was absent in our baseline spec)"]
    if old_resolved is not None and new_resolved is None:
        return ["endpoint removed from the live spec"]

    old_op, new_op = strip_noise(old_resolved), strip_noise(new_resolved)

    changes = []
    changes.extend(diff_parameters(old_op, new_op))
    changes.extend(diff_request_body(old_op, new_op))
    changes.extend(diff_responses(old_op, new_op))

    # Deprecation signals are scanned on the resolved (non-stripped) operation
    # because "summary" is in NOISE_KEYS and may be the only place a
    # deprecation/sunset notice appears.
    old_signals = find_deprecation_signals(old_resolved)
    new_signals = find_deprecation_signals(new_resolved)
    for signal in sorted(new_signals - old_signals):
        changes.append(f"new deprecation signal: {signal}")

    return changes or None


def diff_undocumented_endpoint(old_spec, new_spec, method, path):
    old_present = get_operation(old_spec, path, method) is not None
    new_present = get_operation(new_spec, path, method) is not None
    if not old_present and new_present:
        return ["endpoint newly appears in the live FOSSA spec (previously undocumented)"]
    return None


def run(baseline_spec, live_spec, watchlist):
    findings = []
    for entry in watchlist["endpoints"]:
        method, path, documented = entry["method"], entry["path"], entry.get("documented", True)
        if documented:
            changes = diff_documented_endpoint(baseline_spec, live_spec, method, path)
        else:
            changes = diff_undocumented_endpoint(baseline_spec, live_spec, method, path)
        if changes:
            findings.append({"method": method, "path": path, "changes": changes})
    return findings


def render_report(findings, baseline_version, live_version):
    lines = [
        "## FOSSA API watch-list drift detected",
        "",
        f"Baseline spec version (committed `plugins/fossa/swagger.json`): `{baseline_version}`",
        f"Live spec version (`https://app.fossa.com/api/api-docs/swagger.json`): `{live_version}`",
        "",
    ]
    for finding in findings:
        lines.append(f"### {finding['method']} `{finding['path']}`")
        lines.extend(f"- {change}" for change in finding["changes"])
        lines.append("")
    lines.append(
        "_Generated by `scripts/fossa-api-watch.py` via the `fossa-api-watch` workflow. "
        "Update `plugins/fossa/fossa-api-watchlist.json` if our FOSSA usage has changed._"
    )
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", required=True, help="path to our committed swagger.json")
    parser.add_argument("--watchlist", required=True, help="path to the watch-list JSON config")
    parser.add_argument("--live-url", default="https://app.fossa.com/api/api-docs/swagger.json")
    parser.add_argument("--live-file", help="use a local file instead of fetching --live-url (for testing)")
    parser.add_argument("--report-file", required=True, help="where to write the markdown report")
    parser.add_argument("--github-output", help="path to $GITHUB_OUTPUT to write has_changes=true/false")
    args = parser.parse_args()

    baseline_spec = load_json(args.baseline)
    watchlist = load_json(args.watchlist)
    live_spec = load_json(args.live_file) if args.live_file else fetch_json(args.live_url)

    findings = run(baseline_spec, live_spec, watchlist)

    has_changes = bool(findings)
    if has_changes:
        report = render_report(
            findings,
            baseline_spec.get("info", {}).get("version", "unknown"),
            live_spec.get("info", {}).get("version", "unknown"),
        )
    else:
        report = "No relevant FOSSA API changes detected on the watch-list."

    with open(args.report_file, "w", encoding="utf-8") as f:
        f.write(report + "\n")

    print(report)

    if args.github_output:
        with open(args.github_output, "a", encoding="utf-8") as f:
            f.write(f"has_changes={'true' if has_changes else 'false'}\n")

    return 0


if __name__ == "__main__":
    sys.exit(main())
