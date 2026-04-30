#!/usr/bin/env python3
"""Post Nomos findings as GitLab merge request discussion notes.

Usage:
    python3 annotate-mr.py <nomos-report.json>

Requires environment variables set by GitLab CI:
    CI_API_V4_URL, CI_PROJECT_ID, CI_MERGE_REQUEST_IID, GITLAB_TOKEN
"""

import json
import os
import sys
import urllib.request
import urllib.error


def main() -> int:
    if len(sys.argv) < 2:
        print("Usage: annotate-mr.py <report-path>", file=sys.stderr)
        return 1

    report_path = sys.argv[1]
    if not os.path.isfile(report_path):
        print(f"Report file not found: {report_path}", file=sys.stderr)
        return 1

    api_url = os.environ.get("CI_API_V4_URL", "")
    project_id = os.environ.get("CI_PROJECT_ID", "")
    mr_iid = os.environ.get("CI_MERGE_REQUEST_IID", "")
    token = os.environ.get("GITLAB_TOKEN", "")

    if not all([api_url, project_id, mr_iid, token]):
        print("Missing GitLab CI environment variables, skipping MR annotation.")
        return 0

    with open(report_path, encoding="utf-8") as f:
        report = json.load(f)

    findings = report.get("findings", [])
    if not findings:
        print("No findings to annotate.")
        return 0

    severity_emoji = {
        "critical": "🔴",
        "high": "🟠",
        "medium": "🟡",
        "low": "🔵",
        "info": "ℹ️",
    }

    lines = ["## Nomos Strict Gate Findings", ""]
    for finding in findings:
        sev = finding.get("severity", "info")
        emoji = severity_emoji.get(sev, "ℹ️")
        code = finding.get("code", "")
        msg = finding.get("message", "")
        file_ref = finding.get("file", "")
        line_ref = finding.get("line", "")

        location = ""
        if file_ref:
            location = f" (`{file_ref}"
            if line_ref:
                location += f":{line_ref}"
            location += "`)"

        lines.append(f"- {emoji} **[{sev.upper()}]** `{code}`{location}: {msg}")

    verdict = report.get("verdict", {})
    verdict_status = verdict.get("status", "unknown")
    lines.append("")
    lines.append(f"**Verdict:** {verdict_status}")

    body = "\n".join(lines)

    url = f"{api_url}/projects/{project_id}/merge_requests/{mr_iid}/notes"
    data = json.dumps({"body": body}).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={
            "PRIVATE-TOKEN": token,
            "Content-Type": "application/json",
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(req) as resp:
            if resp.status in (200, 201):
                print(f"Posted {len(findings)} finding(s) to MR !{mr_iid}.")
            else:
                print(f"Unexpected status {resp.status} from GitLab API.")
                return 1
    except urllib.error.HTTPError as e:
        print(f"GitLab API error: {e.code} {e.reason}", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
