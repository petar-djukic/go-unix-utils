#!/usr/bin/env python3
"""Aggregate per-run metrics from tasks.csv and generation tags into runs.csv.

Generation tags follow `generation-<run_id>-<state>` where:
  - run_id is either a timestamp (YYYY-MM-DD-HH-MM-SS) or a named slug
    (e.g. `gh-4994-run43`).
  - state is one of: start, finished, merged, abandoned.

A run is delimited by its start tag and the highest-precedence terminal tag with
the same run_id (precedence: merged > abandoned > finished). Runs missing a
terminal tag are recorded as `in_progress`; their effective window extends to
the next run's start (or to the latest commit date if no later run exists).

Per-run metrics aggregate tasks.csv rows whose commit_date falls inside the run
window.
"""
from __future__ import annotations

import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

import pandas as pd

TAG_RE = re.compile(
    r"^generation-(?P<run_id>.+)-(?P<state>start|finished|merged|abandoned)$"
)
TERMINAL_PRECEDENCE = ("merged", "abandoned", "finished")


def fetch_tag_dates() -> list[tuple[str, pd.Timestamp]]:
    """Return list of (tag, committer_date_utc) for all annotated/light tags."""
    out = subprocess.run(
        [
            "git", "for-each-ref",
            "--format=%(refname:short)\t%(committerdate:iso8601-strict)",
            "refs/tags/",
        ],
        capture_output=True, text=True, check=True,
    ).stdout
    pairs = []
    for line in out.strip().split("\n"):
        if not line.strip():
            continue
        name, date = line.split("\t", 1)
        if not name.startswith("generation-"):
            continue
        ts = pd.Timestamp(date).tz_convert("UTC")
        pairs.append((name, ts))
    return pairs


def group_runs(tags: list[tuple[str, pd.Timestamp]]) -> list[dict]:
    grouped: dict[str, dict[str, pd.Timestamp]] = defaultdict(dict)
    unmatched: list[str] = []
    for name, ts in tags:
        m = TAG_RE.match(name)
        if not m:
            unmatched.append(name)
            continue
        grouped[m.group("run_id")][m.group("state")] = ts

    if unmatched:
        print(f"Skipped {len(unmatched)} non-conforming tags (e.g. {unmatched[:2]})", file=sys.stderr)

    runs = []
    for run_id, states in grouped.items():
        if "start" not in states:
            continue
        end_state = "in_progress"
        end_at: pd.Timestamp | None = None
        for s in TERMINAL_PRECEDENCE:
            if s in states:
                end_state = s
                end_at = states[s]
                break
        runs.append({
            "run_id": run_id,
            "start_at": states["start"],
            "end_at": end_at,
            "end_state": end_state,
        })
    runs.sort(key=lambda r: r["start_at"])
    return runs


def fill_in_progress_ends(runs: list[dict], latest_commit_date: pd.Timestamp) -> None:
    """Set end_at for in_progress runs to the next run's start (or latest commit + 1s)."""
    for i, r in enumerate(runs):
        if r["end_at"] is not None:
            continue
        if i + 1 < len(runs):
            r["end_at"] = runs[i + 1]["start_at"]
        else:
            # Final in_progress run: extend past the latest commit so the
            # half-open interval still captures it.
            r["end_at"] = max(r["start_at"], latest_commit_date) + pd.Timedelta(seconds=1)


def aggregate(df: pd.DataFrame, start: pd.Timestamp, end: pd.Timestamp) -> dict:
    # Half-open interval [start, end) so back-to-back runs do not double-count
    # a commit landing on the boundary timestamp.
    mask = (df["commit_date"] >= start) & (df["commit_date"] < end)
    sub = df.loc[mask]
    zero = (sub["loc_prod_delta"] == 0) & (sub["loc_test_delta"] == 0)
    return {
        "task_count": int(len(sub)),
        "unique_task_ids": int(sub["task_id"].nunique()),
        "retry_count": int(len(sub) - sub["task_id"].nunique()),
        "stitch_count": int((sub["task_subtype"] == "stitch").sum()),
        "measure_count": int((sub["task_subtype"] == "measure").sum()),
        "total_cost_usd": float(sub["cost_usd"].sum()),
        "total_tokens_input": int(sub["tokens_input"].sum()),
        "total_tokens_output": int(sub["tokens_output"].sum()),
        "total_tokens_cache_creation": int(sub["tokens_cache_creation"].sum()),
        "total_tokens_cache_read": int(sub["tokens_cache_read"].sum()),
        "total_loc_prod_delta": int(sub["loc_prod_delta"].sum()),
        "total_loc_test_delta": int(sub["loc_test_delta"].sum()),
        "total_duration_s": int(sub["duration_seconds"].sum()),
        "zero_loc_count": int(zero.sum()),
        "zero_loc_cost_usd": float(sub.loc[zero, "cost_usd"].sum()),
        "unique_targets": int(sub["target"].nunique()),
    }


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent.parent
    tasks_path = repo_root / "analysis" / "datasets" / "tasks.csv"
    if not tasks_path.exists():
        print(f"tasks.csv not found at {tasks_path}", file=sys.stderr)
        return 1

    df = pd.read_csv(tasks_path)
    df["commit_date"] = pd.to_datetime(df["commit_date"], utc=True)

    tags = fetch_tag_dates()
    print(f"Found {len(tags)} generation-* tags")

    runs = group_runs(tags)
    print(f"Grouped into {len(runs)} runs")

    fill_in_progress_ends(runs, df["commit_date"].max())

    rows = []
    for r in runs:
        start = r["start_at"]
        end = r["end_at"]
        agg = aggregate(df, start, end)
        rows.append({
            "run_id": r["run_id"],
            "start_at": start.isoformat(),
            "end_at": end.isoformat(),
            "end_state": r["end_state"],
            "wall_clock_hours": round((end - start).total_seconds() / 3600, 2),
            **agg,
        })

    columns = [
        "run_id", "start_at", "end_at", "end_state", "wall_clock_hours",
        "task_count", "unique_task_ids", "retry_count",
        "stitch_count", "measure_count",
        "total_cost_usd",
        "total_tokens_input", "total_tokens_output",
        "total_tokens_cache_creation", "total_tokens_cache_read",
        "total_loc_prod_delta", "total_loc_test_delta",
        "total_duration_s",
        "zero_loc_count", "zero_loc_cost_usd",
        "unique_targets",
    ]
    out_df = pd.DataFrame(rows, columns=columns)

    out_path = repo_root / "analysis" / "datasets" / "runs.csv"
    out_df.to_csv(out_path, index=False)

    print(f"\nWrote {len(out_df)} rows to {out_path}")
    print("End-state breakdown:")
    print(out_df["end_state"].value_counts().to_string())
    print(f"\nTotal cost (all runs): ${out_df['total_cost_usd'].sum():.2f}")
    print(f"Total tasks counted: {out_df['task_count'].sum()}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
