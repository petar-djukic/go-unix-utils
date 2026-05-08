#!/usr/bin/env python3
"""Parse markdown run reports under docs/reports/ into runs.yaml.

Reports vary widely in schema across the 31 files; this parser is best-effort.
Each field is extracted with a focused regex and recorded as null when not found.
The resulting yaml is keyed by report_id (e.g. "42", "40b") parsed from the
filename. A `generation_branch` field links to runs.csv via the matching run_id.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

REPORT_FILE_RE = re.compile(r"generation-run-(?P<id>[a-z0-9]+)\.md$")

SCAFFOLD_VERSION_RE = re.compile(r"\bv0\.\d{8}\.\d+\b")
GENERATION_BRANCH_RE = re.compile(
    r"\bgeneration-(?:\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}|gh-[A-Za-z0-9-]+|run-\d+|[A-Za-z0-9-]+-run-\d+)\b"
)
DOLLAR_AMOUNT_RE = re.compile(r"\$([0-9]+(?:,[0-9]{3})*(?:\.[0-9]+)?)")
REQUIREMENTS_RATIO_RE = re.compile(
    r"(\d{1,3}(?:,\d{3})*)\s*/\s*(\d{1,3}(?:,\d{3})*)\s*\(?\s*(\d+)?%?\)?"
)
RATE_LIMIT_RE = re.compile(
    r"(?:rate-?limit(?:ed|ing)?|throttl)\D{0,40}?(\d+h)?\s*(\d+)m(\d+)s",
    re.IGNORECASE,
)
INCIDENT_HEADER_RE = re.compile(r"^###\s+(.+?)\s*$", re.MULTILINE)
SCAFFOLD_ISSUE_TABLE_RE = re.compile(
    r"^\|\s*#?(\d+)\s*\|\s*(.+?)\s*\|.+\|\s*$",
    re.MULTILINE,
)


def parse_dollar(text: str) -> float:
    return float(text.replace(",", ""))


def find_total_cost(text: str) -> tuple[float | None, float | None, float | None]:
    """Return (total_cost, stitch_cost, measure_cost) — best-effort.

    Order matters: lines like "Total cost (stitch) | $536" mention both keywords;
    the stitch check must run before the bare-total check so it captures the
    stitch sub-total instead of misclassifying it as the grand total.
    """
    total = stitch = measure = None
    for line in text.splitlines():
        lower = line.lower()
        if "$" not in line:
            continue
        amounts = [parse_dollar(m) for m in DOLLAR_AMOUNT_RE.findall(line)]
        if not amounts:
            continue
        amt = amounts[-1]
        if "stitch" in lower and "cost" in lower:
            if stitch is None:
                stitch = amt
        elif "measure" in lower and "cost" in lower:
            if measure is None:
                measure = amt
        elif "total cost" in lower:
            if total is None:
                total = amt
    return total, stitch, measure


def find_requirements(text: str) -> tuple[int | None, int | None]:
    completed = total = None
    for line in text.splitlines():
        if "requirement" not in line.lower():
            continue
        m = REQUIREMENTS_RATIO_RE.search(line)
        if not m:
            continue
        a = int(m.group(1).replace(",", ""))
        b = int(m.group(2).replace(",", ""))
        if a > b or a < 0:
            continue
        if completed is None or a > completed:
            completed = a
            total = b
    return completed, total


def find_rate_limited_seconds(text: str) -> int | None:
    secs = None
    for m in RATE_LIMIT_RE.finditer(text):
        h = int(m.group(1)[:-1]) if m.group(1) else 0
        mn = int(m.group(2))
        s = int(m.group(3))
        candidate = h * 3600 + mn * 60 + s
        if secs is None or candidate > secs:
            secs = candidate
    return secs


def find_incidents(text: str) -> list[str]:
    """Headers under any "## Incidents" section."""
    out = []
    in_section = False
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.lower().startswith("## "):
            in_section = "incident" in stripped.lower()
            continue
        if in_section:
            m = INCIDENT_HEADER_RE.match(line)
            if m:
                out.append(m.group(1))
    return out


def find_scaffold_issues(text: str) -> list[dict]:
    """Rows under "## Cobbler-Scaffold Issues Filed" or similar table."""
    out: list[dict] = []
    in_section = False
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.lower().startswith("## "):
            in_section = (
                "cobbler-scaffold" in stripped.lower()
                or "scaffold issues" in stripped.lower()
            )
            continue
        if not in_section:
            continue
        m = SCAFFOLD_ISSUE_TABLE_RE.match(line)
        if not m:
            continue
        # Skip header/separator rows
        if m.group(2).startswith("---") or m.group(2).lower() == "title":
            continue
        out.append({"number": int(m.group(1)), "title": m.group(2)})
    return out


def parse_report(path: Path) -> dict:
    text = path.read_text()
    fname_m = REPORT_FILE_RE.search(path.name)
    report_id = fname_m.group("id") if fname_m else path.stem

    title_m = re.search(r"^#\s+(.+?)\s*$", text, re.MULTILINE)
    title = title_m.group(1) if title_m else None

    scaffold_m = SCAFFOLD_VERSION_RE.search(text)
    scaffold_version = scaffold_m.group(0) if scaffold_m else None

    branch_m = GENERATION_BRANCH_RE.search(text)
    generation_branch = branch_m.group(0) if branch_m else None
    run_id = (
        generation_branch.removeprefix("generation-") if generation_branch else None
    )

    total_cost, stitch_cost, measure_cost = find_total_cost(text)
    completed, total_reqs = find_requirements(text)
    rate_limited = find_rate_limited_seconds(text)
    incidents = find_incidents(text)
    scaffold_issues = find_scaffold_issues(text)

    return {
        "report_id": report_id,
        "report_path": str(path.relative_to(path.parent.parent.parent)),
        "report_title": title,
        "generation_branch": generation_branch,
        "run_id": run_id,
        "scaffold_version": scaffold_version,
        "total_cost_usd": total_cost,
        "stitch_cost_usd": stitch_cost,
        "measure_cost_usd": measure_cost,
        "requirements_completed": completed,
        "requirements_total": total_reqs,
        "rate_limited_seconds": rate_limited,
        "incidents": incidents,
        "scaffold_issues_filed": scaffold_issues,
    }


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent.parent
    reports_dir = repo_root / "docs" / "reports"
    if not reports_dir.exists():
        print(f"reports dir not found: {reports_dir}", file=sys.stderr)
        return 1

    files = sorted(reports_dir.glob("generation-run-*.md"))
    print(f"Parsing {len(files)} report(s)")

    entries = [parse_report(f) for f in files]

    # Sort by report_id treating numeric prefix numerically (40b vs 41 → 40, 41)
    def sort_key(e: dict) -> tuple[int, str]:
        rid = e["report_id"] or ""
        m = re.match(r"(\d+)", rid)
        return (int(m.group(1)) if m else -1, rid)

    entries.sort(key=sort_key)

    out_path = repo_root / "analysis" / "datasets" / "runs.yaml"
    with out_path.open("w") as f:
        yaml.safe_dump(
            entries, f, sort_keys=False, allow_unicode=True, default_flow_style=False
        )

    print(f"\nWrote {len(entries)} entries to {out_path}")
    print("\nField completeness:")
    for field in (
        "scaffold_version", "generation_branch", "total_cost_usd",
        "requirements_completed", "rate_limited_seconds",
    ):
        n_set = sum(1 for e in entries if e.get(field) is not None)
        print(f"  {field:30s} {n_set}/{len(entries)}")
    print(f"\nIncident counts: total {sum(len(e['incidents']) for e in entries)} across all reports")
    return 0


if __name__ == "__main__":
    sys.exit(main())
