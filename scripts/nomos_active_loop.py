#!/usr/bin/env python3
"""Nomos Supervisor Active Loop — dispatch queue with dependency tracking."""

import json
import os
import subprocess
import sys
import time
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

POLL_INTERVAL = int(os.environ.get("NOMOS_POLL_INTERVAL", "120"))
STATE_FILE = Path(__file__).parent / "nomos_active_loop_state.json"
LOG_FILE = Path(__file__).parent / "nomos_active_loop.log"
REPOS_ROOT = Path("/root/repos")

AGENTS = ["claude", "codex", "copilot", "cursor", "gemini"]

# ---------------------------------------------------------------------------
# Backlog: ticket definitions with dependencies and file zones
# ---------------------------------------------------------------------------

BACKLOG = [
    # ===== Phase 1 (NOM-001 to NOM-903) — ALL DONE =====
    {"id": "NOM-001", "done": True}, {"id": "NOM-002", "done": True},
    {"id": "NOM-101", "done": True}, {"id": "NOM-102", "done": True},
    {"id": "NOM-103", "done": True}, {"id": "NOM-104", "done": True},
    {"id": "NOM-105", "done": True}, {"id": "NOM-201", "done": True},
    {"id": "NOM-202", "done": True}, {"id": "NOM-203", "done": True},
    {"id": "NOM-204", "done": True}, {"id": "NOM-301", "done": True},
    {"id": "NOM-302", "done": True}, {"id": "NOM-303", "done": True},
    {"id": "NOM-401", "done": True}, {"id": "NOM-402", "done": True},
    {"id": "NOM-403", "done": True}, {"id": "NOM-404", "done": True},
    {"id": "NOM-405", "done": True}, {"id": "NOM-501", "done": True},
    {"id": "NOM-502", "done": True}, {"id": "NOM-503", "done": True},
    {"id": "NOM-504", "done": True}, {"id": "NOM-601", "done": True},
    {"id": "NOM-602", "done": True}, {"id": "NOM-701", "done": True},
    {"id": "NOM-702", "done": True}, {"id": "NOM-703", "done": True},
    {"id": "NOM-801", "done": True}, {"id": "NOM-802", "done": True},
    {"id": "NOM-803", "done": True}, {"id": "NOM-804", "done": True},
    {"id": "NOM-901", "done": True}, {"id": "NOM-902", "done": True},
    {"id": "NOM-903", "done": True},
    # ===== Phase 2: Self-Canonicalization & Release v0.1 =====
    # Wave 0
    {"id": "NOM-1203", "title": "Stabiliser environnement Windows", "deps": [], "zone": "scripts", "agent": None, "branch_slug": "feat/nomos-windows-env-1203", "validation": "go vet ./...", "done": False},
    # Wave 1
    {"id": "NOM-1001", "title": "Ajouter nomos.project.yaml", "deps": ["NOM-1203"], "zone": "nomos.project.yaml", "agent": None, "branch_slug": "feat/nomos-self-project-1001", "validation": "cd cli && go test ./...", "done": False},
    {"id": "NOM-1202", "title": "Script E2E local canonique", "deps": ["NOM-1203"], "zone": "scripts/e2e", "agent": None, "branch_slug": "feat/nomos-e2e-script-1202", "validation": "bash scripts/e2e.sh", "done": False},
    # Wave 2
    {"id": "NOM-1002", "title": "Source manifest canonique Nomos", "deps": ["NOM-1001"], "zone": "docs/canonical/source-manifest", "agent": None, "branch_slug": "feat/nomos-source-manifest-1002", "validation": "cd cli && go test ./...", "done": False},
    {"id": "NOM-1004", "title": "Decision records release v0.1", "deps": ["NOM-1001"], "zone": "docs/decisions", "agent": None, "branch_slug": "feat/nomos-adr-1004", "validation": "ls docs/decisions/*.md", "done": False},
    {"id": "NOM-1101", "title": "Exposer nomos admit CLI", "deps": ["NOM-1001", "NOM-1203"], "zone": "cli/internal/app", "agent": None, "branch_slug": "feat/nomos-admit-cli-1101", "validation": "cd cli && go test ./internal/app/... ./internal/admit/...", "done": False},
    # Wave 3
    {"id": "NOM-1003", "title": "Canonical matrix Nomos", "deps": ["NOM-1002"], "zone": "docs/canonical/nomos-matrix", "agent": None, "branch_slug": "feat/nomos-matrix-1003", "validation": "cd cli && go test ./...", "done": False},
    # Wave 4
    {"id": "NOM-1102", "title": "Exposer checks canoniques CLI", "deps": ["NOM-1002", "NOM-1003"], "zone": "cli/internal/app", "agent": None, "branch_slug": "feat/nomos-checks-cli-1102", "validation": "cd cli && go test ./...", "done": False},
    {"id": "NOM-1103", "title": "Exposer product-check CLI", "deps": ["NOM-1003"], "zone": "cli/internal/app", "agent": None, "branch_slug": "feat/nomos-productcheck-cli-1103", "validation": "cd cli && go test ./...", "done": False},
    # Wave 5
    {"id": "NOM-1104", "title": "Exposer nomos strict CLI", "deps": ["NOM-1101", "NOM-1102", "NOM-1103"], "zone": "cli/internal/app", "agent": None, "branch_slug": "feat/nomos-strict-cli-1104", "validation": "cd cli && go test ./...", "done": False},
    # Wave 6
    {"id": "NOM-1105", "title": "Exposer report, attestations, exports", "deps": ["NOM-1104"], "zone": "cli/internal/app", "agent": None, "branch_slug": "feat/nomos-report-cli-1105", "validation": "cd cli && go test ./...", "done": False},
    {"id": "NOM-1201", "title": "GitHub Actions CI active", "deps": ["NOM-1104", "NOM-1202"], "zone": ".github/workflows", "agent": None, "branch_slug": "feat/nomos-ci-active-1201", "validation": "cat .github/workflows/ci.yml", "done": False},
    # Wave 7
    {"id": "NOM-1301", "title": "Faire passer diagnose a pass", "deps": ["NOM-1003", "NOM-1104", "NOM-1201"], "zone": "cli/internal/diagnose", "agent": None, "branch_slug": "feat/nomos-diagnose-pass-1301", "validation": "cd cli && go test ./...", "done": False},
    # Wave 8
    {"id": "NOM-1302", "title": "Premier nomos-report.json", "deps": ["NOM-1004", "NOM-1105", "NOM-1301"], "zone": "reports", "agent": None, "branch_slug": "feat/nomos-first-report-1302", "validation": "cd cli && go test ./...", "done": False},
    # Wave 9
    {"id": "NOM-1303", "title": "Attestation et SBOM release", "deps": ["NOM-1302"], "zone": "reports/attestations", "agent": None, "branch_slug": "feat/nomos-attest-release-1303", "validation": "cd cli && go test ./...", "done": False},
    # Wave 10
    {"id": "NOM-1401", "title": "PR de readiness sur #46", "deps": ["NOM-1301", "NOM-1302", "NOM-1303"], "zone": "pr", "agent": None, "branch_slug": "feat/nomos-readiness-1401", "validation": "cd cli && go test ./...", "done": False},
    # Wave 11
    {"id": "NOM-1402", "title": "Promouvoir #46 vers main", "deps": ["NOM-1401"], "zone": "pr", "agent": None, "branch_slug": "feat/nomos-promote-main-1402", "validation": "echo ok", "done": False},
]


def log(msg: str):
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    line = f"[{ts}] {msg}"
    print(line, flush=True)
    with open(LOG_FILE, "a") as f:
        f.write(line + "\n")


def load_state() -> dict:
    if STATE_FILE.exists():
        return json.loads(STATE_FILE.read_text())
    return {
        "assignments": {},       # agent -> ticket_id
        "head_at_dispatch": {},  # agent -> commit_sha_at_dispatch
        "done_tickets": [],
        "last_poll": None,
    }


def save_state(state: dict):
    state["last_poll"] = datetime.now(timezone.utc).isoformat()
    STATE_FILE.write_text(json.dumps(state, indent=2) + "\n")


def get_head(agent: str) -> Optional[str]:
    repo = REPOS_ROOT / f"Nomos-{agent}"
    if not repo.exists():
        return None
    try:
        r = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=repo, capture_output=True, text=True, timeout=10,
        )
        return r.stdout.strip() if r.returncode == 0 else None
    except Exception:
        return None


def get_branch(agent: str) -> Optional[str]:
    repo = REPOS_ROOT / f"Nomos-{agent}"
    try:
        r = subprocess.run(
            ["git", "branch", "--show-current"],
            cwd=repo, capture_output=True, text=True, timeout=10,
        )
        return r.stdout.strip() if r.returncode == 0 else None
    except Exception:
        return None


def agent_is_idle(agent: str) -> bool:
    """Check if agent pane shows a prompt (not actively running a command)."""
    try:
        r = subprocess.run(
            ["tmux", "capture-pane", "-t", f"rbok-{agent}:0", "-p"],
            capture_output=True, text=True, timeout=5,
        )
        output = r.stdout.strip()
        last_lines = output.split("\n")[-5:]
        for line in last_lines:
            stripped = line.strip()
            # Claude CLI idle indicators
            if stripped.startswith(">") or stripped.endswith(">"):
                return True
            if "waiting" in stripped.lower():
                return True
            # NOT idle if "esc to interrupt" visible
            if "esc to interrupt" in stripped.lower():
                return False
        return False
    except Exception:
        return False


def ticket_by_id(tid: str) -> Optional[dict]:
    for t in BACKLOG:
        if t["id"] == tid:
            return t
    return None


def is_done(tid: str, state: dict) -> bool:
    t = ticket_by_id(tid)
    if t and t.get("done"):
        return True
    return tid in state.get("done_tickets", [])


def deps_met(ticket: dict, state: dict) -> bool:
    for dep in ticket.get("deps", []):
        if not is_done(dep, state):
            return False
    return True


def zones_overlap(zone_a: str, zone_b: str) -> bool:
    if not zone_a or not zone_b:
        return False
    return zone_a.startswith(zone_b) or zone_b.startswith(zone_a)


def active_zones(state: dict) -> list[str]:
    zones = []
    for agent, tid in state["assignments"].items():
        t = ticket_by_id(tid)
        if t and t.get("zone"):
            zones.append(t["zone"])
    return zones


def next_ready_tickets(state: dict) -> list[dict]:
    assigned_ids = set(state["assignments"].values())
    done_ids = set(state.get("done_tickets", []))
    az = active_zones(state)
    ready = []
    for t in BACKLOG:
        if t.get("done") or t["id"] in done_ids or t["id"] in assigned_ids:
            continue
        if not t.get("title"):
            continue
        if not deps_met(t, state):
            continue
        # Check zone overlap with active assignments
        if any(zones_overlap(t.get("zone", ""), z) for z in az):
            continue
        ready.append(t)
    return ready


def dispatch_to_agent(agent: str, ticket: dict):
    """Send a prompt to the agent via tmux, then a separate Enter to submit."""
    prompt = (
        f"Nouveau ticket {ticket['id']}: {ticket['title']}. "
        f"Branche: `{ticket['branch_slug']}` (git checkout -b depuis ta branche actuelle). "
        f"Zone fichiers: `{ticket['zone']}/*`. "
        f"NE TOUCHE PAS a `cli/internal/app/app.go`. "
        f"Regles: PAS de push, PAS de main, commit local uniquement. "
        f"Convention: feat({ticket['id']}): ... "
        f"Validation: {ticket['validation']}. "
        f"Commence par lire les fichiers existants dans la zone pour comprendre le pattern, puis implemente."
    )
    target = f"rbok-{agent}:0"
    try:
        # Step 1: send the text (no Enter yet — tmux buffers it)
        subprocess.run(["tmux", "send-keys", "-t", target, prompt], timeout=10)
        time.sleep(1)
        # Step 2: send Enter separately to submit
        subprocess.run(["tmux", "send-keys", "-t", target, "Enter"], timeout=10)
        log(f"DISPATCH {ticket['id']} -> {agent} (zone: {ticket['zone']})")
    except Exception as e:
        log(f"ERROR dispatching {ticket['id']} to {agent}: {e}")


def check_agent_done(agent: str, state: dict) -> bool:
    """Check if agent has new commits since dispatch (work done)."""
    dispatch_head = state["head_at_dispatch"].get(agent)
    if not dispatch_head:
        return False
    current_head = get_head(agent)
    if not current_head:
        return False
    if current_head != dispatch_head:
        # Verify the commit message references the ticket
        tid = state["assignments"].get(agent, "")
        repo = REPOS_ROOT / f"Nomos-{agent}"
        try:
            r = subprocess.run(
                ["git", "log", "--oneline", f"{dispatch_head}..HEAD"],
                cwd=repo, capture_output=True, text=True, timeout=10,
            )
            commits = r.stdout.strip()
            if tid.replace("NOM-", "") in commits or tid in commits:
                return True
            # Even without ticket ref, new commits = progress
            if commits:
                return True
        except Exception:
            pass
    return False


def run_loop():
    log("=== Nomos Supervisor Active Loop started ===")
    log(f"Poll interval: {POLL_INTERVAL}s")

    state = load_state()

    # Bootstrap: record current assignments from initial dispatch
    initial_assignments = {
        "claude": "NOM-501",
        "codex": "NOM-403",
        "copilot": "NOM-502",
        "cursor": "NOM-303",
        "gemini": "NOM-404",
    }
    if not state["assignments"]:
        state["assignments"] = dict(initial_assignments)
        for agent in AGENTS:
            head = get_head(agent)
            if head:
                state["head_at_dispatch"][agent] = head
        save_state(state)
        log(f"Bootstrapped state with initial assignments: {state['assignments']}")

    while True:
        try:
            poll_cycle(state)
        except KeyboardInterrupt:
            log("Supervisor stopped by user.")
            save_state(state)
            sys.exit(0)
        except Exception as e:
            log(f"ERROR in poll cycle: {e}")
        save_state(state)
        time.sleep(POLL_INTERVAL)


def poll_cycle(state: dict):
    log("--- Poll cycle start ---")

    free_agents = []

    for agent in AGENTS:
        tid = state["assignments"].get(agent)
        if not tid:
            free_agents.append(agent)
            continue

        # Check if agent finished their ticket
        if check_agent_done(agent, state):
            current_head = get_head(agent)
            branch = get_branch(agent)
            log(f"DONE {tid} by {agent} (head: {current_head}, branch: {branch})")

            if tid not in state["done_tickets"]:
                state["done_tickets"].append(tid)

            # Mark backlog entry
            t = ticket_by_id(tid)
            if t:
                t["done"] = True
                t["agent"] = agent

            del state["assignments"][agent]
            if agent in state["head_at_dispatch"]:
                del state["head_at_dispatch"][agent]
            free_agents.append(agent)
        else:
            log(f"WIP  {tid} by {agent} (head: {get_head(agent)})")

    # Dispatch next ready tickets to free agents
    if free_agents:
        ready = next_ready_tickets(state)
        log(f"Free agents: {free_agents}, Ready tickets: {[t['id'] for t in ready]}")

        for agent in free_agents:
            if not ready:
                break
            ticket = ready.pop(0)
            state["assignments"][agent] = ticket["id"]
            state["head_at_dispatch"][agent] = get_head(agent)
            dispatch_to_agent(agent, ticket)

    # Summary
    done_count = len(state.get("done_tickets", []))
    active_count = len(state.get("assignments", {}))
    pending = [t for t in BACKLOG if not t.get("done") and t["id"] not in state.get("done_tickets", []) and t["id"] not in state.get("assignments", {}).values() and t.get("title")]
    log(f"STATUS done={done_count} active={active_count} queued={len(pending)}")
    log("--- Poll cycle end ---")


if __name__ == "__main__":
    run_loop()
