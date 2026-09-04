#!/usr/bin/env python3
"""VRC-42 (#578) — reference substrate adapter for the NOMOS rule protocol.

WHAT THIS IS NOT
================
This is **not a rule engine**, and NOMOS must never grow one — anti-goal §10.3
of the closure plan is explicit about it. This script exists to prove the
protocol carries a computation and its source trace end to end. It evaluates one
deliberately trivial shape and answers `unsupported` for everything else, which
is the honest answer and the one the protocol is built to carry.

For real work, point `--substrate-cmd` at a borrowed engine — OpenFisca, Catala,
an L4 service. The licence register (`docs/regulated/ip-governance/
license-register.yaml`) sets the terms: OpenFisca is AGPL, its integration
policy is `process_api_boundary`, and `may_link_in_process` is false. This
protocol IS that boundary: NOMOS spawns a process and exchanges JSON on stdin
and stdout. Nothing is imported, linked or vendored, in Go or in Python.

**This adapter imports no third-party module on purpose.** An adapter that did
`import openfisca` would put AGPL code in the same process as NOMOS-authored
code, which is exactly what the register forbids. An adapter for a real engine
must reach it the same way NOMOS reaches this one: across a process or network
boundary.

PROTOCOL
========
Reads a `nomos-rule-substrate-request-v1` object on stdin, writes a
`nomos-rule-substrate-response-v1` object on stdout. Every formula is answered
exactly once, with `computed` (carrying a value) or `unsupported` (carrying a
reason). Never both, never neither.

The supported shape is a sum of signed integer literals and named parameters,
e.g. `12 + 30` or `base + supplement`. That is all. Anything else — a product, a
condition, a date, prose — is `unsupported`, by design rather than by omission.
"""

from __future__ import annotations

import json
import re
import sys

REQUEST_SCHEMA = "nomos-rule-substrate-request-v1"
RESPONSE_SCHEMA = "nomos-rule-substrate-response-v1"
SUBSTRATE = "nomos-demo-substrate/1.0 (protocol conformance fixture, not a rule engine)"

# The one shape this fixture evaluates: integers and parameter names joined by
# + or -. Anchored, so nothing else slips through.
_TERM = re.compile(r"^[+-]?\s*(?:\d+|[A-Za-z_][A-Za-z0-9_]*)$")
_SPLIT = re.compile(r"(?=[+-])")


def evaluate(expression: str, parameters: dict[str, str]) -> tuple[str, object, str]:
    """Return ``(status, value, reason)`` for one expression."""
    text = " ".join(expression.split())
    if not text:
        return "unsupported", None, "empty expression"

    parts = [p.strip() for p in _SPLIT.split(text) if p.strip()]
    if not parts:
        return "unsupported", None, "no term found"

    total = 0
    for part in parts:
        if not _TERM.match(part):
            return (
                "unsupported",
                None,
                f"term {part!r} is outside the shape this fixture evaluates "
                "(signed integers and named parameters joined by + or -)",
            )
        sign = -1 if part.startswith("-") else 1
        body = part.lstrip("+-").strip()
        if body.isdigit():
            total += sign * int(body)
            continue
        if body not in parameters:
            return "unsupported", None, f"parameter {body!r} was not supplied"
        raw = str(parameters[body]).strip()
        try:
            total += sign * int(raw)
        except ValueError:
            return "unsupported", None, f"parameter {body!r} is not an integer: {raw!r}"
    return "computed", total, ""


def main() -> int:
    try:
        request = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        print(f"substrate: stdin is not JSON: {exc}", file=sys.stderr)
        return 2

    if request.get("schema_version") != REQUEST_SCHEMA:
        print(
            f"substrate: request schema_version is {request.get('schema_version')!r}, "
            f"expected {REQUEST_SCHEMA!r}",
            file=sys.stderr,
        )
        return 2

    results = []
    for formula in request.get("formulas", []) or []:
        status, value, reason = evaluate(
            str(formula.get("expression", "")),
            dict(formula.get("parameters") or {}),
        )
        entry: dict[str, object] = {"atom_id": formula.get("atom_id"), "status": status}
        if status == "computed":
            entry["value"] = value
            entry["unit"] = "scalar"
        else:
            entry["reason"] = reason
        results.append(entry)

    json.dump(
        {"schema_version": RESPONSE_SCHEMA, "substrate": SUBSTRATE, "results": results},
        sys.stdout,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
