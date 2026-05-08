#!/usr/bin/env python3
"""Derive task_attempts.csv and utilities.csv from tasks.csv + runs.csv.

Two outputs grouping the per-commit task data along the two natural axes:

- task_id  -> task_attempts.csv: reveals retries (attempt_count > 1) and the
              zero-LOC tasks that produced no code change.
- target   -> utilities.csv: cumulative cost per cmd/X or pkg/X across all
              runs, plus consistency metrics like cost variance per run.

wasted_cost_usd is computed as `total_cost - productive_cost` where
productive_cost is the cost of the LATEST commit for that task_id whose
loc_prod_delta > 0 OR loc_test_delta > 0 (else 0). This unifies the two
buckets in the issue spec ("zero-LOC attempts" and "all-but-last attempt
of multi-attempt tasks") with no double-counting.
"""
from __future__ import annotations

import sys
from pathlib import Path

import pandas as pd


def assign_run_id(tasks: pd.DataFrame, runs: pd.DataFrame) -> pd.DataFrame:
    """Add a `run_id` column to tasks based on the run window enclosing commit_date.

    Half-open interval [start_at, end_at). If multiple runs match (16 known
    overlap cases per #5018), take the first chronologically. If none match
    (4 known orphan commits), leave NaN.
    """
    runs_sorted = runs.sort_values("start_at").reset_index(drop=True)
    starts = runs_sorted["start_at"].to_numpy()
    ends = runs_sorted["end_at"].to_numpy()
    rids = runs_sorted["run_id"].to_numpy()

    def lookup(dt):
        for s, e, rid in zip(starts, ends, rids):
            if s <= dt < e:
                return rid
        return None

    tasks = tasks.copy()
    tasks["run_id"] = tasks["commit_date"].apply(lookup)
    return tasks


def build_task_attempts(tasks: pd.DataFrame) -> pd.DataFrame:
    rows = []
    for task_id, group in tasks.sort_values(["task_id", "commit_date"]).groupby("task_id", sort=True):
        group = group.sort_values("commit_date")
        last = group.iloc[-1]
        productive_mask = (group["loc_prod_delta"] > 0) | (group["loc_test_delta"] > 0)
        productive = group.loc[productive_mask]
        productive_cost = (
            float(productive.iloc[-1]["cost_usd"]) if len(productive) > 0 else 0.0
        )
        total_cost = float(group["cost_usd"].sum())
        rows.append({
            "task_id": int(task_id),
            "attempt_count": int(len(group)),
            "final_target": last["target"],
            "final_srd_id": last["srd_id"],
            "total_cost_usd": round(total_cost, 6),
            "total_duration_s": int(group["duration_seconds"].sum()),
            "total_tokens_input": int(group["tokens_input"].sum()),
            "productive_attempts": int(productive_mask.sum()),
            "zero_loc_attempts": int((~productive_mask).sum()),
            "wasted_cost_usd": round(total_cost - productive_cost, 6),
            "first_attempt_at": group.iloc[0]["commit_date"].isoformat(),
            "last_attempt_at": last["commit_date"].isoformat(),
            "span_hours": round(
                (last["commit_date"] - group.iloc[0]["commit_date"]).total_seconds() / 3600,
                2,
            ),
        })
    return pd.DataFrame(rows)


def build_utilities(tasks: pd.DataFrame) -> pd.DataFrame:
    rows = []
    for target, group in tasks.sort_values(["target", "commit_date"]).groupby(
        "target", dropna=True, sort=True
    ):
        group = group.sort_values("commit_date")
        last = group.iloc[-1]
        srd_mode = group["srd_id"].mode(dropna=True)
        srd_id = srd_mode.iloc[0] if len(srd_mode) > 0 else None
        per_run = group.dropna(subset=["run_id"]).groupby("run_id")["cost_usd"].sum()
        cost_var = float(per_run.std()) if len(per_run) > 1 else 0.0
        zero = ((group["loc_prod_delta"] == 0) & (group["loc_test_delta"] == 0)).sum()
        rows.append({
            "target": target,
            "target_kind": last["target_kind"],
            "srd_id": srd_id,
            "total_runs_touched": int(group["run_id"].nunique()),
            "total_tasks": int(len(group)),
            "total_attempts": int(len(group)),
            "unique_task_ids": int(group["task_id"].nunique()),
            "total_cost_usd": round(float(group["cost_usd"].sum()), 6),
            "cost_variance_per_run": round(cost_var, 6),
            "final_loc_prod": int(last["loc_prod_after"]),
            "final_loc_test": int(last["loc_test_after"]),
            "mean_duration_s": round(float(group["duration_seconds"].mean()), 2),
            "p95_duration_s": round(float(group["duration_seconds"].quantile(0.95)), 2),
            "zero_loc_count": int(zero),
        })
    return pd.DataFrame(rows)


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent.parent
    datasets = repo_root / "analysis" / "datasets"
    tasks_path = datasets / "tasks.csv"
    runs_path = datasets / "runs.csv"
    if not tasks_path.exists() or not runs_path.exists():
        print(f"Required inputs missing: {tasks_path}, {runs_path}", file=sys.stderr)
        return 1

    tasks = pd.read_csv(tasks_path)
    tasks["commit_date"] = pd.to_datetime(tasks["commit_date"], utc=True)
    runs = pd.read_csv(runs_path)
    runs["start_at"] = pd.to_datetime(runs["start_at"], utc=True)
    runs["end_at"] = pd.to_datetime(runs["end_at"], utc=True)

    tasks = assign_run_id(tasks, runs)
    print(f"Loaded {len(tasks)} task rows")
    print(f"  Mapped to a run: {tasks['run_id'].notna().sum()}")
    print(f"  Orphan (outside any run window): {tasks['run_id'].isna().sum()}")
    print(f"  Distinct targets (non-null): {tasks['target'].notna().sum() and tasks['target'].nunique()}")

    attempts = build_task_attempts(tasks)
    utils = build_utilities(tasks)

    out_attempts = datasets / "task_attempts.csv"
    out_utils = datasets / "utilities.csv"
    attempts.to_csv(out_attempts, index=False)
    utils.to_csv(out_utils, index=False)

    print(f"\nWrote {len(attempts)} rows to {out_attempts.relative_to(repo_root)}")
    counts = attempts["attempt_count"].value_counts().sort_index()
    print(f"  attempt_count distribution: {counts.to_dict()}")
    print(f"  task_ids with zero-LOC attempts: {(attempts['zero_loc_attempts'] > 0).sum()}")
    print(f"  total wasted cost: ${attempts['wasted_cost_usd'].sum():.2f}")

    print(f"\nWrote {len(utils)} rows to {out_utils.relative_to(repo_root)}")
    print(f"  target_kind: {utils['target_kind'].value_counts().to_dict()}")
    print(f"  total cost (utilities, excludes orphan target=None): ${utils['total_cost_usd'].sum():.2f}")
    print(f"  total cost in tasks.csv: ${tasks['cost_usd'].sum():.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
