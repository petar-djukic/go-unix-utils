#!/usr/bin/env python3
"""Extract one row per Claude Code stitch invocation from *-stitch-stats.yaml.

Each `*-stitch-stats.yaml` is the orchestrator's structured summary of one
stitch invocation. Multiple stats files with the same task_id at different
started_at timestamps mean Claude Code was actually invoked twice for the
same task — the only authoritative signal of a real retry.

Walks:
  analysis/raw/run-43-cobbler/history/*-stitch-stats.yaml
  analysis/raw/recovered/<run_id>/*-stitch-stats.yaml

Output: analysis/datasets/stitch_invocations.csv
"""
from __future__ import annotations

import sys
from pathlib import Path

import pandas as pd
import yaml

COLUMNS = [
    "stats_path", "run_id",
    "task_id", "task_id_kind", "task_title",
    "status", "started_at",
    "duration_s",
    "cost_usd",
    "tokens_input", "tokens_output", "cache_creation", "cache_read",
    "num_turns", "rate_limit_wait_s",
    "loc_prod_before", "loc_prod_after", "loc_prod_delta",
    "loc_test_before", "loc_test_after", "loc_test_delta",
    "files_changed", "insertions", "deletions",
]


def task_id_kind(task_id: str) -> str:
    return "numeric" if task_id.isdigit() else "slug"


def parse_one(path: Path, run_id: str, repo_root: Path) -> dict | None:
    try:
        with path.open() as f:
            data = yaml.safe_load(f)
    except Exception:
        return None
    if not isinstance(data, dict) or "task_id" not in data:
        return None

    task_id = str(data["task_id"])
    tokens = data.get("tokens", {}) or {}
    loc_b = data.get("loc_before", {}) or {}
    loc_a = data.get("loc_after", {}) or {}
    diff = data.get("diff", {}) or {}

    loc_prod_before = int(loc_b.get("production", 0))
    loc_prod_after = int(loc_a.get("production", 0))
    loc_test_before = int(loc_b.get("test", 0))
    loc_test_after = int(loc_a.get("test", 0))

    return {
        "stats_path": str(path.relative_to(repo_root)),
        "run_id": run_id,
        "task_id": task_id,
        "task_id_kind": task_id_kind(task_id),
        "task_title": data.get("task_title"),
        "status": data.get("status"),
        "started_at": data.get("started_at"),
        "duration_s": int(data.get("duration_s") or 0),
        "cost_usd": float(data.get("cost_usd") or 0.0),
        "tokens_input": int(tokens.get("input", 0) or 0),
        "tokens_output": int(tokens.get("output", 0) or 0),
        "cache_creation": int(tokens.get("cache_creation", 0) or 0),
        "cache_read": int(tokens.get("cache_read", 0) or 0),
        "num_turns": int(data.get("num_turns") or 0),
        "rate_limit_wait_s": int(data.get("rate_limit_wait_s") or 0),
        "loc_prod_before": loc_prod_before,
        "loc_prod_after": loc_prod_after,
        "loc_prod_delta": loc_prod_after - loc_prod_before,
        "loc_test_before": loc_test_before,
        "loc_test_after": loc_test_after,
        "loc_test_delta": loc_test_after - loc_test_before,
        "files_changed": int(diff.get("files", 0) or 0),
        "insertions": int(diff.get("insertions", 0) or 0),
        "deletions": int(diff.get("deletions", 0) or 0),
    }


def find_stats_files(repo_root: Path) -> list[tuple[str, Path]]:
    pairs: list[tuple[str, Path]] = []
    run43 = repo_root / "analysis" / "raw" / "run-43-cobbler" / "history"
    if run43.exists():
        for f in sorted(run43.glob("*-stitch-stats.yaml")):
            pairs.append(("gh-4994-run43", f))
    run44 = repo_root / ".cobbler" / "history"
    if run44.exists():
        for f in sorted(run44.glob("*-stitch-stats.yaml")):
            pairs.append(("gh-5059-run44", f))
    recovered = repo_root / "analysis" / "raw" / "recovered"
    if recovered.exists():
        for d in sorted(recovered.iterdir()):
            if not d.is_dir():
                continue
            for f in sorted(d.glob("*-stitch-stats.yaml")):
                pairs.append((d.name, f))
    return pairs


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent.parent
    pairs = find_stats_files(repo_root)
    rows: list[dict] = []
    skipped = 0
    for run_id, path in pairs:
        rec = parse_one(path, run_id, repo_root)
        if rec is None:
            skipped += 1
            continue
        rows.append(rec)
    df = pd.DataFrame(rows, columns=COLUMNS).sort_values(["task_id", "started_at"]).reset_index(drop=True)

    out = repo_root / "analysis" / "datasets" / "stitch_invocations.csv"
    df.to_csv(out, index=False)
    print(f"Found {len(pairs)} *-stitch-stats.yaml files; parsed {len(df)}, skipped {skipped}")
    print(f"Wrote {len(df)} rows to {out.relative_to(repo_root)}")
    print(f"Distinct task_ids: {df['task_id'].nunique()}")
    print(f"task_id_kind: {df['task_id_kind'].value_counts().to_dict()}")
    print(f"Total cost across invocations: ${df['cost_usd'].sum():.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
