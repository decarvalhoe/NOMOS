"""Silence guard — docs/43 principle 8, docs/48 §3: « ce qui se tait ment ».

An exception handler in `scripts/` that catches an error and leaves no trace —
no raise, no message, no recorded problem, no error value returned — is the
class of defect learned from a neighbouring system (a deduplication that
silently disabled itself when its library was missing, a broad except that
produced zero chunks without an error). This guard refuses it.

Rules, checked on the AST — two classes, the ones actually learned:

* a BROAD handler (bare `except:`, `except Exception`, `except BaseException`)
  whose body carries no SIGNAL is silent. A signal is: a `raise`; a call to
  print/log/warn/exit or to `.append(...)` on a collection whose name says
  problems/findings/errors/failures/warnings/issues/refusals; an assignment to
  a name that says error/problem/failure/reason; a `return` of something other
  than a bare None/False/empty literal;
* a PURE SWALLOW — a handler of any type whose body is only `pass`, `continue`
  or `break` — is silent whatever it catches: the error changes nothing and
  the run goes on as if it had succeeded.

A handler that catches a specific exception and returns a sentinel hands the
decision to its caller; that is a different question and not flagged here.
Every silent handler is refused unless its FILE is allowlisted below with a
justification — and the allowlist must stay exact: an entry that no longer
matches a silent handler is refused too (no stale exemptions).

Test-only helpers and fixtures are out of scope; production scripts are in.
"""

from __future__ import annotations

import ast
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"

# Files allowed to swallow errors, each with the reason. Not gates, not proofs.
ALLOWLIST: dict[str, str] = {
    "scripts/nomos_active_loop.py": "agent orchestration heuristics (git/tmux probes): a probe that cannot answer returns None/False by design; no evidence flows through it",
    "scripts/seam_deliver_demo.py": "demo client: a non-JSON HTTP error body falls back to an empty detail before the error is reported to the user",
}

SIGNAL_CALLS = {"print", "exit", "warn", "warning", "error", "critical", "fatal", "log", "info", "debug", "exception", "fail"}
SIGNAL_COLLECTIONS = ("problem", "finding", "error", "failure", "warning", "issue", "refusal", "reason", "unreadable", "skipped")
SIGNAL_ASSIGN = ("error", "problem", "failure", "reason", "unreadable", "skipped")


def _is_trivial_return(node: ast.Return) -> bool:
    value = node.value
    if value is None:
        return True
    if isinstance(value, ast.Constant) and value.value in (None, False, "", 0):
        return True
    if isinstance(value, (ast.List, ast.Dict, ast.Tuple, ast.Set)) and not getattr(value, "elts", None) and not getattr(value, "keys", None):
        return True
    return False


def _name_of(node: ast.AST) -> str:
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        return node.attr
    return ""


def _has_signal(handler: ast.ExceptHandler) -> bool:
    # A handler that USES the caught exception (records it, formats it into a
    # status, a reason, a report) is not swallowing it.
    if handler.name:
        for node in ast.walk(handler):
            if isinstance(node, ast.Name) and node.id == handler.name and isinstance(node.ctx, ast.Load):
                return True
    for node in ast.walk(handler):
        if isinstance(node, ast.Raise):
            return True
        if isinstance(node, ast.Return) and not _is_trivial_return(node):
            return True
        if isinstance(node, ast.Call):
            func = node.func
            name = _name_of(func).lower()
            if name in SIGNAL_CALLS or name.endswith(("_error", "_problem", "_finding")):
                return True
            if name == "append" and isinstance(func, ast.Attribute):
                target = _name_of(func.value).lower()
                if any(key in target for key in SIGNAL_COLLECTIONS):
                    return True
            if isinstance(func, ast.Attribute) and _name_of(func.value) in {"sys", "logging", "logger", "log", "warnings"}:
                return True
        if isinstance(node, (ast.Assign, ast.AugAssign, ast.AnnAssign)):
            targets = node.targets if isinstance(node, ast.Assign) else [node.target]
            for target in targets:
                if any(key in _name_of(target).lower() for key in SIGNAL_ASSIGN):
                    return True
    return False


BROAD = {"Exception", "BaseException"}


def _caught_names(handler: ast.ExceptHandler) -> set[str]:
    if handler.type is None:
        return {"<bare>"}
    nodes = handler.type.elts if isinstance(handler.type, ast.Tuple) else [handler.type]
    return {_name_of(n) or "<bare>" for n in nodes}


def _is_broad(handler: ast.ExceptHandler) -> bool:
    return bool(_caught_names(handler) & (BROAD | {"<bare>"}))


def _is_pure_swallow(handler: ast.ExceptHandler) -> bool:
    return all(isinstance(stmt, (ast.Pass, ast.Continue, ast.Break)) for stmt in handler.body)


def silent_handlers(path: Path) -> list[str]:
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    found = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Try):
            for handler in node.handlers:
                if _has_signal(handler):
                    continue
                caught = ast.unparse(handler.type) if handler.type is not None else "<bare>"
                if _is_pure_swallow(handler):
                    found.append(f"{path.relative_to(ROOT)}:{handler.lineno} except {caught}: pure swallow (pass/continue/break), the run goes on as if it had succeeded")
                elif _is_broad(handler):
                    found.append(f"{path.relative_to(ROOT)}:{handler.lineno} except {caught}: broad catch with no raise, no message, no recorded problem, no error value")
    return found


def production_scripts() -> list[Path]:
    return sorted(p for p in SCRIPTS.glob("*.py") if not p.name.startswith("test_"))


class SilenceGuardTests(unittest.TestCase):
    def test_no_exception_is_swallowed_silently_outside_the_allowlist(self) -> None:
        offenders = []
        for path in production_scripts():
            rel = str(path.relative_to(ROOT))
            if rel in ALLOWLIST:
                continue
            offenders.extend(silent_handlers(path))
        self.assertEqual(offenders, [], "silent exception handler(s) — say something, or justify the file in ALLOWLIST:\n" + "\n".join(offenders))

    def test_allowlist_entries_are_exact_and_justified(self) -> None:
        for rel, reason in ALLOWLIST.items():
            path = ROOT / rel
            self.assertTrue(path.is_file(), f"{rel}: allowlisted file no longer exists — remove the entry")
            self.assertGreaterEqual(len(reason), 30, f"{rel}: justify the exemption")
            self.assertTrue(silent_handlers(path), f"{rel}: no silent handler left — remove the stale exemption")

    def test_the_guard_recognises_a_silent_handler(self) -> None:
        # The proof that the guard bites: a synthetic module with a swallowed error.
        sample = ROOT / "scripts" / "__silence_guard_probe__.py"
        try:
            sample.write_text("def f():\n    try:\n        return 1\n    except Exception:\n        return None\n", encoding="utf-8")
            self.assertTrue(silent_handlers(sample), "a broad catch returning nothing must be flagged")
            sample.write_text("def f(paths):\n    for p in paths:\n        try:\n            open(p)\n        except OSError:\n            continue\n", encoding="utf-8")
            self.assertTrue(silent_handlers(sample), "a pure swallow must be flagged whatever it catches")
            sample.write_text("def f(p):\n    try:\n        return open(p)\n    except OSError:\n        return None\n", encoding="utf-8")
            self.assertEqual(silent_handlers(sample), [], "a specific catch returning a sentinel is the caller's question")
            sample.write_text("def f(problems):\n    try:\n        return 1\n    except Exception as exc:\n        problems.append(str(exc))\n        return None\n", encoding="utf-8")
            self.assertEqual(silent_handlers(sample), [])
        finally:
            if sample.exists():
                sample.unlink()


if __name__ == "__main__":
    unittest.main()
