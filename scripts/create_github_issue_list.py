#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable


EPIC_RE = re.compile(r"^## EPIC (E\d+) - (.+)$")
ISSUE_RE = re.compile(r"^### (NOM-\d+) - (.+)$")
SECTION_HEADERS = {"Description :", "BLOCKS :", "ENABLES :", "DoD :", "But :"}


@dataclass
class Epic:
    code: str
    title: str
    goal: list[str] = field(default_factory=list)
    issues: list["Issue"] = field(default_factory=list)

    @property
    def issue_title(self) -> str:
        return f"EPIC {self.code} - {self.title}"

    def body(self) -> str:
        lines: list[str] = []
        lines.append("Source: `docs/15-product-backlog.md`")
        lines.append("")
        lines.append(f"Epic code: `{self.code}`")
        lines.append("")
        lines.append("Goal:")
        lines.extend(render_lines(self.goal))
        lines.append("")
        lines.append("Planned child issues:")
        for issue in self.issues:
            lines.append(f"- {issue.issue_title}")
        return "\n".join(lines).strip() + "\n"


@dataclass
class Issue:
    code: str
    title: str
    epic_code: str
    epic_title: str
    description: list[str] = field(default_factory=list)
    blocks: list[str] = field(default_factory=list)
    enables: list[str] = field(default_factory=list)
    dod: list[str] = field(default_factory=list)

    @property
    def issue_title(self) -> str:
        return f"{self.code} - {self.title}"

    def body(self) -> str:
        lines: list[str] = []
        lines.append("Source: `docs/15-product-backlog.md`")
        lines.append("")
        lines.append(f"Epic: `EPIC {self.epic_code} - {self.epic_title}`")
        lines.append("")
        if self.description:
            lines.append("Description:")
            lines.extend(render_lines(self.description))
            lines.append("")
        lines.append("BLOCKS:")
        lines.extend(render_lines(self.blocks))
        lines.append("")
        lines.append("ENABLES:")
        lines.extend(render_lines(self.enables))
        lines.append("")
        lines.append("DoD:")
        lines.extend(render_lines(self.dod))
        return "\n".join(lines).strip() + "\n"


def render_lines(values: Iterable[str]) -> list[str]:
    items = [normalize_spaces(v) for v in values if normalize_spaces(v)]
    if not items:
        return ["- none"]
    rendered = []
    for item in items:
        if item.startswith("- "):
            rendered.append(item)
        else:
            rendered.append(f"- {item}")
    return rendered


def normalize_spaces(value: str) -> str:
    return re.sub(r"\s+", " ", value.strip())


def parse_backlog(path: Path) -> tuple[list[Epic], list[Issue]]:
    lines = path.read_text(encoding="utf-8").splitlines()
    i = 0
    epics: list[Epic] = []
    issues: list[Issue] = []
    current_epic: Epic | None = None

    while i < len(lines):
        line = lines[i].rstrip()
        epic_match = EPIC_RE.match(line)
        issue_match = ISSUE_RE.match(line)

        if epic_match:
            current_epic = Epic(code=epic_match.group(1), title=epic_match.group(2))
            epics.append(current_epic)
            i += 1
            section, i = parse_sections(lines, i, stop_prefixes=("## ", "### "))
            current_epic.goal = section.get("But :", [])
            continue

        if issue_match and current_epic is not None:
            issue = Issue(
                code=issue_match.group(1),
                title=issue_match.group(2),
                epic_code=current_epic.code,
                epic_title=current_epic.title,
            )
            i += 1
            section, i = parse_sections(lines, i, stop_prefixes=("## ", "### "))
            issue.description = section.get("Description :", [])
            issue.blocks = section.get("BLOCKS :", [])
            issue.enables = section.get("ENABLES :", [])
            issue.dod = section.get("DoD :", [])
            current_epic.issues.append(issue)
            issues.append(issue)
            continue

        i += 1

    return epics, issues


def parse_sections(lines: list[str], start: int, stop_prefixes: tuple[str, ...]) -> tuple[dict[str, list[str]], int]:
    data: dict[str, list[str]] = {}
    current_section: str | None = None
    i = start

    while i < len(lines):
        raw = lines[i].rstrip()
        stripped = raw.strip()

        if any(raw.startswith(prefix) for prefix in stop_prefixes):
            break

        if stripped in SECTION_HEADERS:
            current_section = stripped
            data.setdefault(current_section, [])
            i += 1
            continue

        if stripped and current_section is not None:
            data[current_section].append(stripped)

        i += 1

    return data, i


def gh_json(args: list[str]) -> list[dict]:
    result = subprocess.run(
        ["gh", *args],
        check=True,
        text=True,
        capture_output=True,
    )
    return json.loads(result.stdout or "[]")


def gh_create_issue(repo: str, title: str, body: str) -> str:
    result = subprocess.run(
        ["gh", "issue", "create", "--repo", repo, "--title", title, "--body", body],
        check=True,
        text=True,
        capture_output=True,
    )
    return result.stdout.strip()


def main() -> int:
    parser = argparse.ArgumentParser(description="Create GitHub issues from the Nomos product backlog.")
    parser.add_argument("--repo", required=True, help="GitHub repository, e.g. RBOKproject/Nomos")
    parser.add_argument(
        "--backlog",
        default="docs/15-product-backlog.md",
        help="Path to the backlog markdown file",
    )
    parser.add_argument("--dry-run", action="store_true", help="Print the issues that would be created")
    args = parser.parse_args()

    backlog_path = Path(args.backlog)
    epics, issues = parse_backlog(backlog_path)

    existing = {
        item["title"]
        for item in gh_json(["issue", "list", "--repo", args.repo, "--state", "all", "--limit", "500", "--json", "title"])
    }

    created = 0
    skipped = 0

    for epic in epics:
        title = epic.issue_title
        if title in existing:
            print(f"SKIP {title}")
            skipped += 1
            continue
        if args.dry_run:
            print(f"DRYRUN {title}")
        else:
            url = gh_create_issue(args.repo, title, epic.body())
            print(f"CREATED {title} -> {url}")
            existing.add(title)
        created += 1

    for issue in issues:
        title = issue.issue_title
        if title in existing:
            print(f"SKIP {title}")
            skipped += 1
            continue
        if args.dry_run:
            print(f"DRYRUN {title}")
        else:
            url = gh_create_issue(args.repo, title, issue.body())
            print(f"CREATED {title} -> {url}")
            existing.add(title)
        created += 1

    print(f"SUMMARY created={created} skipped={skipped} epics={len(epics)} issues={len(issues)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
