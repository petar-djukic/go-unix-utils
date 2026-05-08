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
    """Group by task_id over commit-trailer rows.

    NOTE: tasks.csv uses commit author dates (%aI) which are preserved across
    rebases and cherry-picks. Multiple commits with the same task_id at the
    same author date are usually git artifacts (e.g., a generation-branch
    commit cherry-picked onto main), not real Claude retries. The fields here
    are named with a `git_` prefix to make this provenance explicit.

    For real-retry analysis, see task_retries.csv (built from
    *-stitch-stats.yaml log files via extract_stitch_invocations.py).
    """
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
            "git_commit_count": int(len(group)),
            "final_target": last["target"],
            "final_srd_id": last["srd_id"],
            "total_cost_usd": round(total_cost, 6),
            "total_duration_s": int(group["duration_seconds"].sum()),
            "total_tokens_input": int(group["tokens_input"].sum()),
            "productive_commits": int(productive_mask.sum()),
            "zero_loc_commits": int((~productive_mask).sum()),
            "git_wasted_cost_proxy": round(total_cost - productive_cost, 6),
            "first_commit_at": group.iloc[0]["commit_date"].isoformat(),
            "last_commit_at": last["commit_date"].isoformat(),
            "span_hours": round(
                (last["commit_date"] - group.iloc[0]["commit_date"]).total_seconds() / 3600,
                2,
            ),
        })
    return pd.DataFrame(rows)


def build_task_retries(invocations: pd.DataFrame) -> pd.DataFrame:
    """Group stitch_invocations.csv by task_id — the authoritative retry source.

    Each row in stitch_invocations.csv represents one Claude Code stitch
    invocation. invocation_count > 1 means the orchestrator invoked Claude
    multiple times on the same task_id — a real retry.
    """
    rows = []
    for task_id, group in invocations.sort_values(["task_id", "started_at"]).groupby("task_id", sort=True):
        group = group.sort_values("started_at")
        productive_mask = (group["loc_prod_delta"] > 0) | (group["loc_test_delta"] > 0)
        productive = group.loc[productive_mask]
        productive_cost = (
            float(productive.iloc[-1]["cost_usd"]) if len(productive) > 0 else 0.0
        )
        total_cost = float(group["cost_usd"].sum())
        first = group.iloc[0]
        last = group.iloc[-1]
        first_started = pd.Timestamp(first["started_at"])
        last_started = pd.Timestamp(last["started_at"])
        rows.append({
            "task_id": str(task_id),
            "task_id_kind": first["task_id_kind"],
            "invocation_count": int(len(group)),
            "status_history": ";".join(group["status"].astype(str).tolist()),
            "successful_invocations": int((group["status"] == "success").sum()),
            "productive_invocations": int(productive_mask.sum()),
            "total_cost_usd": round(total_cost, 6),
            "wasted_cost_usd": round(total_cost - productive_cost, 6),
            "total_duration_s": int(group["duration_s"].sum()),
            "total_num_turns": int(group["num_turns"].sum()),
            "first_started_at": first["started_at"],
            "last_started_at": last["started_at"],
            "span_hours": round((last_started - first_started).total_seconds() / 3600, 2),
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
    counts = attempts["git_commit_count"].value_counts().sort_index()
    print(f"  git_commit_count distribution: {counts.to_dict()}")
    print(f"  task_ids with zero_loc_commits: {(attempts['zero_loc_commits'] > 0).sum()}")
    print(f"  total git_wasted_cost_proxy: ${attempts['git_wasted_cost_proxy'].sum():.2f}")

    print(f"\nWrote {len(utils)} rows to {out_utils.relative_to(repo_root)}")
    print(f"  target_kind: {utils['target_kind'].value_counts().to_dict()}")
    print(f"  total cost (utilities, excludes orphan target=None): ${utils['total_cost_usd'].sum():.2f}")
    print(f"  total cost in tasks.csv: ${tasks['cost_usd'].sum():.2f}")

    invocations_path = datasets / "stitch_invocations.csv"
    if invocations_path.exists():
        invocations = pd.read_csv(invocations_path)
        retries = build_task_retries(invocations)
        out_retries = datasets / "task_retries.csv"
        retries.to_csv(out_retries, index=False)
        retry_counts = retries["invocation_count"].value_counts().sort_index()
        print(f"\nWrote {len(retries)} rows to {out_retries.relative_to(repo_root)}")
        print(f"  invocation_count distribution: {retry_counts.to_dict()}")
        print(f"  task_ids with retries (count > 1): {(retries['invocation_count'] > 1).sum()}")
        print(f"  total wasted_cost_usd (log-based): ${retries['wasted_cost_usd'].sum():.2f}")
    else:
        print(f"\n(skipped task_retries.csv: stitch_invocations.csv not found)")

    return 0


if __name__ == "__main__":
    sys.exit(main())
