#!/usr/bin/env python3
"""Extract GitHub issues and pull requests into CSVs.

Walks `gh issue list --state all` and `gh pr list --state all`, parses each
JSON record, and emits:

  analysis/datasets/issues.csv
  analysis/datasets/prs.csv

These power downstream charts: issue close-reason mix, problem-report rate,
PR cycle time, generation-PR identification, etc. Aborts if `gh auth status`
fails.
"""
from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path

import pandas as pd

REPO = "petar-djukic/go-unix-utils"
LIMIT = 6000

ISSUE_FIELDS = [
    "number", "title", "state", "labels", "createdAt", "closedAt",
    "closed", "body", "assignees", "milestone",
]
PR_FIELDS = [
    "number", "title", "state", "labels", "createdAt", "closedAt",
    "mergedAt", "baseRefName", "headRefName",
    "additions", "deletions", "changedFiles",
]

_TASK_TITLE_RE = re.compile(r"^Task (\d+):")
_STITCH_TITLE_RE = re.compile(r"^\[stitch\]")
_MEASURE_TITLE_RE = re.compile(r"^\[measure\]")
_TARGET_RE = re.compile(r"\b(cmd|pkg)/([A-Za-z0-9_.-]+)")
_PROBLEM_TITLE_RE = re.compile(r"\b(problem|fail|crash|broken)\b", re.IGNORECASE)


def check_gh_auth() -> None:
    res = subprocess.run(
        ["gh", "auth", "status"], capture_output=True, text=True
    )
    if res.returncode != 0:
        sys.stderr.write(
            "gh auth status failed; run `gh auth login` first.\n"
            f"{res.stderr}\n"
        )
        sys.exit(1)


def gh_list(kind: str, fields: list[str]) -> list[dict]:
    """kind in {issue, pr}. Returns parsed JSON list."""
    cmd = [
        "gh", kind, "list", "--state", "all",
        "--limit", str(LIMIT),
        "--repo", REPO,
        "--json", ",".join(fields),
    ]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        raise RuntimeError(
            f"gh {kind} list failed (code={res.returncode}): {res.stderr}"
        )
    return json.loads(res.stdout)


def parse_iso(value) -> pd.Timestamp | None:
    if not value:
        return None
    try:
        return pd.to_datetime(value, utc=True)
    except (ValueError, TypeError):
        return None


def cycle_hours(start: pd.Timestamp | None, end: pd.Timestamp | None) -> float | None:
    if start is None or end is None:
        return None
    return round((end - start).total_seconds() / 3600.0, 4)


def parse_target(title: str) -> str | None:
    m = _TARGET_RE.search(title or "")
    if m:
        return f"{m.group(1)}/{m.group(2)}"
    return None


def parse_task_id(title: str) -> int | None:
    m = _TASK_TITLE_RE.match(title or "")
    return int(m.group(1)) if m else None


def parse_task_subtype(title: str) -> str | None:
    if _STITCH_TITLE_RE.match(title or ""):
        return "stitch"
    if _MEASURE_TITLE_RE.match(title or ""):
        return "measure"
    return None


def cobbler_gen_label(labels: list[str]) -> str | None:
    for l in labels:
        if l.startswith("cobbler-gen-"):
            return l[len("cobbler-gen-"):]
    return None


def label_names(labels) -> list[str]:
    if not labels:
        return []
    return [l.get("name", "") for l in labels if isinstance(l, dict)]


def is_problem_report(title: str, labels: list[str]) -> bool:
    if any("problem-report" in l.lower() for l in labels):
        return True
    return bool(_PROBLEM_TITLE_RE.search(title or ""))


def is_recurring(title: str, labels: list[str]) -> bool:
    if title and title.startswith("Recurring:"):
        return True
    return any("recurring" in l.lower() for l in labels)


def issues_to_df(rows: list[dict]) -> pd.DataFrame:
    out = []
    for r in rows:
        title = r.get("title", "") or ""
        labels = label_names(r.get("labels"))
        created = parse_iso(r.get("createdAt"))
        closed = parse_iso(r.get("closedAt"))
        subtype = parse_task_subtype(title)
        is_task = subtype is not None or any(l == "cobbler-ready" for l in labels)
        out.append({
            "number": r.get("number"),
            "title": title,
            "state": r.get("state"),
            "created_at": created.isoformat() if created is not None else None,
            "closed_at": closed.isoformat() if closed is not None else None,
            "cycle_time_hours": cycle_hours(created, closed),
            "labels": ";".join(labels),
            "is_task_issue": is_task,
            "task_subtype": subtype,
            "cobbler_run_id": cobbler_gen_label(labels),
            "is_problem_report": is_problem_report(title, labels),
            "is_recurring": is_recurring(title, labels),
            "target": parse_target(title),
            "body_length": len(r.get("body") or ""),
        })
    return pd.DataFrame(out, columns=[
        "number", "title", "state", "created_at", "closed_at",
        "cycle_time_hours", "labels", "is_task_issue", "task_subtype",
        "cobbler_run_id", "is_problem_report", "is_recurring",
        "target", "body_length",
    ])


def prs_to_df(rows: list[dict]) -> pd.DataFrame:
    out = []
    for r in rows:
        title = r.get("title", "") or ""
        labels = label_names(r.get("labels"))
        created = parse_iso(r.get("createdAt"))
        closed = parse_iso(r.get("closedAt"))
        merged_at = parse_iso(r.get("mergedAt"))
        end = merged_at if merged_at is not None else closed
        head_ref = r.get("headRefName") or ""
        state = r.get("state")
        out.append({
            "number": r.get("number"),
            "title": title,
            "state": state,
            "merged": state == "MERGED",
            "created_at": created.isoformat() if created is not None else None,
            "closed_at": closed.isoformat() if closed is not None else None,
            "merged_at": merged_at.isoformat() if merged_at is not None else None,
            "cycle_time_hours": cycle_hours(created, end),
            "base_ref": r.get("baseRefName"),
            "head_ref": head_ref,
            "additions": r.get("additions"),
            "deletions": r.get("deletions"),
            "changed_files": r.get("changedFiles"),
            "labels": ";".join(labels),
            "is_task_pr": bool(_TASK_TITLE_RE.match(title)),
            "task_id": parse_task_id(title),
            "target": parse_target(title),
            "is_generation_pr": head_ref.startswith("generation-"),
            "is_recurring_pr": is_recurring(title, labels),
        })
    return pd.DataFrame(out, columns=[
        "number", "title", "state", "merged", "created_at", "closed_at",
        "merged_at", "cycle_time_hours", "base_ref", "head_ref",
        "additions", "deletions", "changed_files", "labels",
        "is_task_pr", "task_id", "target",
        "is_generation_pr", "is_recurring_pr",
    ])


def main(repo_root: Path) -> int:
    check_gh_auth()
    out_dir = repo_root / "analysis" / "datasets"
    out_dir.mkdir(parents=True, exist_ok=True)

    print(f"fetching issues from {REPO} ...")
    issues_raw = gh_list("issue", ISSUE_FIELDS)
    print(f"  {len(issues_raw)} issues")
    issues = issues_to_df(issues_raw)
    issues.to_csv(out_dir / "issues.csv", index=False)
    print(f"  wrote {out_dir / 'issues.csv'}")

    print(f"fetching pull requests from {REPO} ...")
    prs_raw = gh_list("pr", PR_FIELDS)
    print(f"  {len(prs_raw)} pull requests")
    prs = prs_to_df(prs_raw)
    prs.to_csv(out_dir / "prs.csv", index=False)
    print(f"  wrote {out_dir / 'prs.csv'}")

    print()
    print(f"issues: {len(issues)} rows")
    print(f"  task issues:    {int(issues['is_task_issue'].sum())}")
    print(f"    stitch:       {int((issues['task_subtype']=='stitch').sum())}")
    print(f"    measure:      {int((issues['task_subtype']=='measure').sum())}")
    print(f"  problem reports:{int(issues['is_problem_report'].sum())}")
    print(f"  recurring:      {int(issues['is_recurring'].sum())}")
    print(f"  closed:         {int((issues['state'] == 'CLOSED').sum())}")
    print(f"  unique cobbler runs: {issues['cobbler_run_id'].nunique()}")
    print(f"prs: {len(prs)} rows")
    print(f"  generation PRs: {int(prs['is_generation_pr'].sum())}")
    print(f"  merged:         {int(prs['merged'].sum())}")
    return 0


if __name__ == "__main__":
    repo = Path(__file__).resolve().parents[2]
    sys.exit(main(repo))
