#!/usr/bin/env python3
"""Extract per-task commit data from generation runs into tasks.csv.

Walks all commits matching `^Task NNN:` subject pattern across all branches via
`git log --all`, parses subjects + Git trailers + diff stats into one row per
task commit. Dedupes by commit SHA since git log --all visits commits reachable
from multiple refs.
"""
from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

import pandas as pd

COMMIT_SEP = "<<<<COBBLER_COMMIT>>>>"
END_MARKER = "<<<<COBBLER_END>>>>"

TASK_PREFIX_RE = re.compile(r"^Task\s+(?P<task_id>\d+):\s*(?P<rest>.+)$")
BRACKET_RE = re.compile(r"^\[(?P<label>[^\]]+)\]\s*(?P<rest>.*)$")
CMD_PKG_RE = re.compile(r"^(?P<kind>cmd|pkg)/(?P<name>[A-Za-z0-9_]+)")
PRD_RE = re.compile(r"^(?:prd|srd)(?P<num>\d+)-(?P<name>[A-Za-z0-9_]+)")
REL_RE = re.compile(
    r"^rel\d+\.\d+(?:-uc\d+)?(?:[\s-]+(?P<name>[a-z][A-Za-z0-9_]*))?"
)
FALLBACK_TARGET_RE = re.compile(r"\b(cmd|pkg)/([a-z][A-Za-z0-9_]*)")
SRDPRD_NAMED_RE = re.compile(r"(?:srd|prd)\d+-([a-z][a-z0-9_]*)")
SRDPRD_RE = re.compile(r"(?:srd|prd)(\d+)")
REQ_RE = re.compile(r"R\d+\.\d+(?:-R?\d+\.\d+)?")

# Known pkg/ targets — the rest of prdNNN-name commits map to cmd/.
PKG_NAMES = {"testutils", "format", "sys", "sizeparse", "encutil"}
TRAILER_RE = re.compile(
    r"^([A-Z][A-Za-z]+(?:-[A-Z][A-Za-z]+)*)\s*:\s*(.+)$", re.MULTILINE
)

REQUIRED_TRAILERS = (
    "Tokens-Input",
    "Tokens-Output",
    "Tokens-Cache-Creation",
    "Tokens-Cache-Read",
    "Tokens-Cost-USD",
    "Loc-Prod-Before",
    "Loc-Prod-After",
    "Loc-Test-Before",
    "Loc-Test-After",
    "Duration-Seconds",
)

COLUMNS = [
    "commit_sha", "commit_date",
    "task_id", "task_subtype", "target", "target_kind",
    "srd_id", "requirements", "is_contract", "commit_subject",
    "tokens_input", "tokens_output", "tokens_cache_creation", "tokens_cache_read",
    "cost_usd",
    "loc_prod_before", "loc_prod_after", "loc_prod_delta",
    "loc_test_before", "loc_test_after", "loc_test_delta",
    "duration_seconds",
    "files_changed", "insertions", "deletions",
]


def fetch_commits() -> str:
    fmt = f"{COMMIT_SEP}%H%n%aI%n%s%n%b%n{END_MARKER}"
    result = subprocess.run(
        [
            "git", "log",
            "--all", "-E", "--grep=^Task [0-9]+:",
            "--no-merges",
            f"--pretty=format:{fmt}",
            "--shortstat",
        ],
        capture_output=True, text=True, check=True,
    )
    return result.stdout


def normalize_srd(num: str) -> str:
    return f"srd{num.zfill(3)}" if len(num) < 3 else f"srd{num}"


def parse_int(s: str) -> int:
    return int(s.strip())


def parse_subject(subject: str) -> dict | None:
    m = TASK_PREFIX_RE.match(subject)
    if not m:
        return None
    task_id = int(m.group("task_id"))
    rest = m.group("rest")

    bm = BRACKET_RE.match(rest)
    inner_label: str | None = None
    if bm:
        subtype = bm.group("label")
        rest = bm.group("rest")
        inner_m = BRACKET_RE.match(rest)
        if inner_m:
            inner_label = inner_m.group("label")
            rest = inner_m.group("rest")
    else:
        # Pre-bracket-convention format (e.g. `Task 469: cmd/ts implementation: ...`).
        # These are all implementation commits; classify as stitch.
        subtype = "stitch"

    target: str | None = None
    target_kind = "other"
    cm = CMD_PKG_RE.match(rest)
    if cm:
        target = f"{cm.group('kind')}/{cm.group('name')}"
        target_kind = cm.group("kind")
    else:
        pm = PRD_RE.match(rest)
        if pm:
            name = pm.group("name")
            kind = "pkg" if name in PKG_NAMES else "cmd"
            target = f"{kind}/{name}"
            target_kind = kind
        else:
            rm = REL_RE.match(rest)
            if rm and rm.group("name"):
                name = rm.group("name")
                kind = "pkg" if name in PKG_NAMES else "cmd"
                target = f"{kind}/{name}"
                target_kind = kind

    if target is None:
        fb = FALLBACK_TARGET_RE.search(subject)
        if fb:
            target = f"{fb.group(1)}/{fb.group(2)}"
            target_kind = fb.group(1)
        else:
            named = SRDPRD_NAMED_RE.search(subject)
            if named:
                name = named.group(1)
                kind = "pkg" if name in PKG_NAMES else "cmd"
                target = f"{kind}/{name}"
                target_kind = kind

    srd_match = SRDPRD_RE.search(subject)
    srd_id = normalize_srd(srd_match.group(1)) if srd_match else None

    req_matches = REQ_RE.findall(subject)
    requirements = ", ".join(req_matches) if req_matches else None

    is_contract = "(contract)" in subject or inner_label == "contract"

    return {
        "task_id": task_id,
        "subtype": subtype,
        "target": target,
        "target_kind": target_kind,
        "srd_id": srd_id,
        "requirements": requirements,
        "is_contract": is_contract,
    }


def parse_chunk(chunk: str) -> tuple[dict | None, str | None]:
    lines = chunk.split("\n")
    if END_MARKER not in lines:
        return None, "no_end_marker"
    end_idx = lines.index(END_MARKER)
    if end_idx < 3:
        return None, "too_short"

    sha = lines[0]
    date = lines[1]
    subject = lines[2]
    body = "\n".join(lines[3:end_idx])
    stat_text = "\n".join(lines[end_idx + 1:])

    parsed = parse_subject(subject)
    if parsed is None:
        return None, "subject_unmatched"
    task_id = parsed["task_id"]
    subtype = parsed["subtype"]
    target = parsed["target"]
    target_kind = parsed["target_kind"]
    srd_id = parsed["srd_id"]
    requirements = parsed["requirements"]
    is_contract = parsed["is_contract"]

    trailers = {k: v for k, v in TRAILER_RE.findall(body)}
    missing = [t for t in REQUIRED_TRAILERS if t not in trailers]
    if missing:
        return None, f"missing_trailers:{','.join(missing)}"

    sm_files = re.search(r"(\d+)\s+files?\s+changed", stat_text)
    sm_ins = re.search(r"(\d+)\s+insertion", stat_text)
    sm_del = re.search(r"(\d+)\s+deletion", stat_text)
    files_changed = int(sm_files.group(1)) if sm_files else 0
    insertions = int(sm_ins.group(1)) if sm_ins else 0
    deletions = int(sm_del.group(1)) if sm_del else 0

    try:
        record = {
            "commit_sha": sha,
            "commit_date": date,
            "task_id": task_id,
            "task_subtype": subtype,
            "target": target,
            "target_kind": target_kind,
            "srd_id": srd_id,
            "requirements": requirements,
            "is_contract": is_contract,
            "commit_subject": subject,
            "tokens_input": parse_int(trailers["Tokens-Input"]),
            "tokens_output": parse_int(trailers["Tokens-Output"]),
            "tokens_cache_creation": parse_int(trailers["Tokens-Cache-Creation"]),
            "tokens_cache_read": parse_int(trailers["Tokens-Cache-Read"]),
            "cost_usd": float(trailers["Tokens-Cost-USD"]),
            "loc_prod_before": parse_int(trailers["Loc-Prod-Before"]),
            "loc_prod_after": parse_int(trailers["Loc-Prod-After"]),
            "loc_test_before": parse_int(trailers["Loc-Test-Before"]),
            "loc_test_after": parse_int(trailers["Loc-Test-After"]),
            "duration_seconds": parse_int(trailers["Duration-Seconds"]),
            "files_changed": files_changed,
            "insertions": insertions,
            "deletions": deletions,
        }
    except ValueError as e:
        return None, f"value_error:{e}"

    record["loc_prod_delta"] = record["loc_prod_after"] - record["loc_prod_before"]
    record["loc_test_delta"] = record["loc_test_after"] - record["loc_test_before"]
    return record, None


def main() -> int:
    raw = fetch_commits()
    chunks = [c for c in raw.split(COMMIT_SEP) if c.strip()]

    records: list[dict] = []
    skip_reasons: dict[str, int] = {}

    for chunk in chunks:
        rec, err = parse_chunk(chunk)
        if rec is None:
            skip_reasons[err] = skip_reasons.get(err, 0) + 1
            continue
        records.append(rec)

    seen: set[str] = set()
    unique: list[dict] = []
    for r in records:
        if r["commit_sha"] not in seen:
            seen.add(r["commit_sha"])
            unique.append(r)

    print(f"Visited {len(records)} commits, {len(unique)} unique after dedup")
    if skip_reasons:
        print(f"Skipped {sum(skip_reasons.values())} commits:")
        for reason, n in sorted(skip_reasons.items(), key=lambda x: -x[1]):
            print(f"  {n:5d}  {reason}")

    df = pd.DataFrame(unique, columns=COLUMNS).sort_values("commit_date").reset_index(drop=True)

    out_path = Path(__file__).resolve().parent.parent / "datasets" / "tasks.csv"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    df.to_csv(out_path, index=False)

    print(f"\nWrote {len(df)} rows to {out_path.relative_to(Path.cwd().resolve()) if out_path.is_relative_to(Path.cwd().resolve()) else out_path}")
    print(f"Total cost: ${df['cost_usd'].sum():.2f}")
    print(f"Date range: {df['commit_date'].min()} to {df['commit_date'].max()}")
    print(f"Unique task_ids: {df['task_id'].nunique()}")
    print(f"Unique targets: {df['target'].nunique()}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
