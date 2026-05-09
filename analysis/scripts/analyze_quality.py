#!/usr/bin/env python3
"""Quality charts for #5025.

Tests output volume relative to production code, both as a per-utility
heatmap across runs and as run-level distributions. The end-of-run
test/prod ratio is computed from the last commit per (utility, run); a
ratio of 0 with positive prod LOC flags utilities that compiled but had
no tests written that run.
"""
from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns
import yaml


def setup_style() -> None:
    sns.set_theme(style="whitegrid", context="talk")
    plt.rcParams["figure.dpi"] = 100
    plt.rcParams["savefig.dpi"] = 300
    plt.rcParams["savefig.bbox"] = "tight"


def save(fig: plt.Figure, out_dir: Path, name: str) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    fig.savefig(out_dir / f"{name}.png")
    fig.savefig(out_dir / f"{name}.svg")
    plt.close(fig)


def titled(ax: plt.Axes, title: str, subtitle: str) -> None:
    ax.set_title(title, pad=24, loc="center", fontsize=14)
    ax.text(
        0, 1.01, subtitle,
        transform=ax.transAxes, fontsize=9, color="#555", va="bottom",
    )


def load(repo_root: Path) -> dict:
    d = repo_root / "analysis" / "datasets"
    runs = pd.read_csv(d / "runs.csv")
    runs["start_at"] = pd.to_datetime(runs["start_at"], utc=True)
    runs["end_at"] = pd.to_datetime(runs["end_at"], utc=True, errors="coerce")
    runs = runs.sort_values("start_at").reset_index(drop=True)
    runs["run_order"] = runs.index + 1
    runs["short_id"] = _short_run_id(runs)
    tasks = pd.read_csv(d / "tasks.csv")
    tasks["commit_date"] = pd.to_datetime(tasks["commit_date"], utc=True)
    utilities = pd.read_csv(d / "utilities.csv")
    return {"runs": runs, "tasks": tasks, "utilities": utilities,
            "repo_root": repo_root}


def _short_run_id(runs: pd.DataFrame) -> pd.Series:
    """Try to abbreviate run_id to something readable. Prefer trailing numeric
    suffix (run42, run40b, ...) when present."""
    pattern = re.compile(r"(run[\d.]+[a-z]*)$|generation-run-([\d]+)$")
    out = []
    for rid in runs["run_id"].astype(str):
        m = pattern.search(rid)
        if m:
            out.append(m.group(1) or f"run{m.group(2)}")
        else:
            out.append(rid[:12])
    return pd.Series(out, index=runs.index)


def _assign_run_to_tasks(tasks: pd.DataFrame, runs: pd.DataFrame) -> pd.DataFrame:
    """Add `run_id` column to tasks by matching commit_date into the run window
    [start_at, end_at). Tasks outside any window get run_id=None."""
    t = tasks.copy()
    t["run_id"] = None
    for _, run in runs.iterrows():
        end = run["end_at"] if pd.notna(run["end_at"]) else tasks["commit_date"].max() + pd.Timedelta(seconds=1)
        mask = (t["commit_date"] >= run["start_at"]) & (t["commit_date"] < end) & t["run_id"].isna()
        t.loc[mask, "run_id"] = run["run_id"]
    return t


def _final_per_target_per_run(tasks_with_run: pd.DataFrame) -> pd.DataFrame:
    """For each (run_id, target), aggregate the per-task loc_*_delta values.
    The trailer deltas reflect what the commit added or removed; summing across
    all tasks for that target in the run gives the per-utility net change
    during that run.

    `loc_prod_after` here is reused as the running total of prod LOC additions
    for the utility in the run (sum of deltas), and `loc_test_after` similarly
    for tests. This avoids the whole-repo trailer values that tasks.csv carries
    in the *_after columns, which are not per-utility.
    """
    sub = tasks_with_run[tasks_with_run["target"].notna() & tasks_with_run["run_id"].notna()].copy()
    grouped = sub.groupby(["run_id", "target"], as_index=False).agg(
        loc_prod_after=("loc_prod_delta", "sum"),
        loc_test_after=("loc_test_delta", "sum"),
        n_tasks=("task_id", "count"),
        cost_usd=("cost_usd", "sum"),
    )
    grouped["test_prod_ratio"] = (
        grouped["loc_test_after"] / grouped["loc_prod_after"].replace(0, np.nan)
    )
    return grouped


def chart_test_prod_ratio_heatmap(d: dict, final: pd.DataFrame, out_dir: Path) -> dict:
    runs = d["runs"]
    pivot = (
        final.pivot_table(index="target", columns="run_id",
                          values="test_prod_ratio", aggfunc="mean")
        .reindex(columns=runs["run_id"])
    )
    # keep only utilities with at least 3 runs of data, top 40 by activity
    counts = pivot.notna().sum(axis=1)
    keep = counts.sort_values(ascending=False).head(40).index
    pivot = pivot.loc[keep]
    short_map = runs.set_index("run_id")["short_id"].to_dict()
    pivot.columns = [short_map.get(c, c) for c in pivot.columns]

    fig, ax = plt.subplots(figsize=(16, 10))
    sns.heatmap(pivot, ax=ax, cmap="RdYlGn", vmin=0, vmax=1.5, center=0.5,
                cbar_kws={"label": "loc_test_after / loc_prod_after"},
                linewidths=0.4, linecolor="white")
    ax.set_xlabel("")
    ax.set_ylabel("")
    plt.setp(ax.get_xticklabels(), rotation=45, ha="right", fontsize=8)
    plt.setp(ax.get_yticklabels(), fontsize=9)
    titled(
        ax, "Test/prod LOC ratio by utility and run",
        "source: tasks.csv last-commit per (run, target); top 40 utilities by run coverage; "
        "blank cells = utility not touched in that run",
    )
    save(fig, out_dir, "test_prod_ratio_heatmap")
    return {"n_utilities_in_heatmap": int(pivot.shape[0]),
            "n_runs_in_heatmap": int(pivot.shape[1])}


def chart_utilities_with_zero_tests_per_run(d: dict, final: pd.DataFrame,
                                            out_dir: Path) -> dict:
    runs = d["runs"]
    zero = final[final["loc_prod_after"] > 0].assign(
        zero_test=lambda x: x["loc_test_after"] == 0
    )
    per_run = zero.groupby("run_id")["zero_test"].sum().astype(int)
    per_run = per_run.reindex(runs["run_id"]).fillna(0).astype(int)
    short = runs.set_index("run_id")["short_id"].reindex(per_run.index)

    fig, ax = plt.subplots(figsize=(14, 6))
    bars = ax.bar(range(len(per_run)), per_run.values, color="#c44e52")
    for i, (idx, v) in enumerate(per_run.items()):
        if v > 0:
            ax.text(i, v + 0.1, str(v), ha="center", va="bottom", fontsize=8, color="#333")
    ax.set_xticks(range(len(per_run)))
    ax.set_xticklabels(short.values, rotation=60, ha="right", fontsize=8)
    ax.set_ylabel("count of utilities with loc_test_after == 0")
    titled(
        ax, "Utilities with no tests at end of run (despite positive prod LOC)",
        f"source: tasks.csv last-commit per (run, target) where loc_prod_after > 0",
    )
    save(fig, out_dir, "utilities_with_zero_tests_per_run")
    return {"max_zero_test_count": int(per_run.max()),
            "runs_with_any_zero_test": int((per_run > 0).sum())}


def chart_median_test_prod_ratio_drift(d: dict, final: pd.DataFrame,
                                       out_dir: Path) -> dict:
    runs = d["runs"]
    median = (
        final[final["loc_prod_after"] > 0]
        .groupby("run_id")["test_prod_ratio"].median()
        .reindex(runs["run_id"])
    )
    short = runs.set_index("run_id")["short_id"].reindex(median.index)

    fig, ax = plt.subplots(figsize=(14, 6))
    valid = median.dropna()
    valid_x = [list(median.index).index(idx) for idx in valid.index]
    ax.plot(valid_x, valid.values, marker="o", color="#4c72b0")
    ax.set_xticks(range(len(median)))
    ax.set_xticklabels(short.values, rotation=60, ha="right", fontsize=8)
    ax.axhline(0.86, color="#55a868", linestyle="--", alpha=0.5, label="run-40b target 0.86")
    ax.axhline(0.70, color="#dd8452", linestyle="--", alpha=0.5, label="run-42 target 0.70")
    ax.set_ylabel("median test/prod ratio across utilities")
    ax.legend(loc="upper left", fontsize=9)
    titled(
        ax, "Median test/prod LOC ratio per run",
        f"source: tasks.csv; median over utilities with loc_prod_after > 0 in each run",
    )
    save(fig, out_dir, "median_test_prod_ratio_drift")
    median_dict = {str(k): (round(float(v), 4) if pd.notna(v) else None)
                   for k, v in median.items()}
    return {"median_per_run": median_dict}


def chart_diff_test_skip_rate(d: dict, out_dir: Path) -> dict:
    """Counts current cmd/* directories whose *_test.go contains t.Skip patterns."""
    repo = d["repo_root"]
    cmd_dir = repo / "cmd"
    skip_pattern = re.compile(r"\bt\.Skip\b")
    counts = {"cmd_dirs_total": 0, "with_test_file": 0, "with_t_skip": 0,
              "skipping_files": []}
    if not cmd_dir.exists():
        return counts
    for d_path in sorted(cmd_dir.iterdir()):
        if not d_path.is_dir():
            continue
        counts["cmd_dirs_total"] += 1
        test_files = list(d_path.glob("*_test.go"))
        if not test_files:
            continue
        counts["with_test_file"] += 1
        for tf in test_files:
            try:
                content = tf.read_text()
            except OSError:
                continue
            if skip_pattern.search(content):
                counts["with_t_skip"] += 1
                counts["skipping_files"].append(str(tf.relative_to(repo)))
                break

    fig, ax = plt.subplots(figsize=(8, 6))
    cats = ["cmd dirs", "with test file", "with t.Skip"]
    vals = [counts["cmd_dirs_total"], counts["with_test_file"], counts["with_t_skip"]]
    bars = ax.bar(cats, vals, color=["#4c72b0", "#55a868", "#dd8452"])
    for bar, v in zip(bars, vals):
        ax.text(bar.get_x() + bar.get_width() / 2, v + 0.5, str(v),
                ha="center", va="bottom", fontsize=11, color="#333")
    ax.set_ylabel("count")
    titled(
        ax, "Diff-test skip presence in current cmd/ tests",
        f"source: cmd/*/*_test.go on main; t.Skip is the marker for `gX` reference-binary missing",
    )
    save(fig, out_dir, "diff_test_skip_rate")
    return counts


def chart_test_addition_trajectory(d: dict, tasks_with_run: pd.DataFrame,
                                   out_dir: Path) -> dict:
    counts = (
        tasks_with_run[tasks_with_run["target"].notna()]
        .groupby("target").size().sort_values(ascending=False).head(10)
    )
    targets = counts.index.tolist()
    sub = tasks_with_run[tasks_with_run["target"].isin(targets)].sort_values("commit_date")

    fig, axes = plt.subplots(2, 5, figsize=(18, 8), sharex=False, sharey=False)
    for ax, target in zip(axes.flatten(), targets):
        s = sub[sub["target"] == target].copy()
        s["seq"] = range(1, len(s) + 1)
        ax.plot(s["seq"], s["loc_test_after"], color="#4c72b0", marker="o", markersize=3,
                label="test")
        ax.plot(s["seq"], s["loc_prod_after"], color="#dd8452", marker="x", markersize=3,
                label="prod")
        ax.set_title(target, fontsize=10)
        ax.set_xlabel("commit seq", fontsize=8)
        ax.set_ylabel("loc_after", fontsize=8)
        ax.tick_params(axis="both", labelsize=7)
        if target == targets[0]:
            ax.legend(loc="upper left", fontsize=7)
    fig.suptitle("Production and test LOC trajectory for top-10 most-touched utilities",
                 fontsize=13, y=1.0)
    fig.text(0.5, -0.01,
             "source: tasks.csv chronological per target; seq is commit index across all runs.",
             ha="center", fontsize=8, color="#555")
    plt.tight_layout()
    save(fig, out_dir, "test_addition_trajectory")
    return {"trajectory_targets": targets}


def chart_test_prod_ratio_distribution_per_run(d: dict, final: pd.DataFrame,
                                               out_dir: Path) -> dict:
    runs = d["runs"]
    sub = final[final["loc_prod_after"] > 0].copy()
    # keep only runs with >=5 utilities to make boxplots meaningful
    counts = sub.groupby("run_id").size()
    keep = counts[counts >= 5].index
    sub = sub[sub["run_id"].isin(keep)].copy()
    short = runs.set_index("run_id")["short_id"]
    sub["short"] = sub["run_id"].map(short)
    order = (
        runs[runs["run_id"].isin(keep)].sort_values("start_at")["short_id"].tolist()
    )

    fig, ax = plt.subplots(figsize=(16, 7))
    sns.boxplot(data=sub, x="short", y="test_prod_ratio", ax=ax, order=order,
                color="#8172b3", fliersize=2)
    ax.set_xlabel("")
    ax.set_ylabel("test/prod ratio per utility")
    ax.set_ylim(-0.05, 2.0)
    plt.setp(ax.get_xticklabels(), rotation=60, ha="right", fontsize=8)
    titled(
        ax, "Per-utility test/prod ratio distribution by run",
        f"source: tasks.csv last-commit per (run, target) with prod>0; runs with >=5 utilities only",
    )
    save(fig, out_dir, "test_prod_ratio_distribution_per_run")
    return {"n_runs_in_boxplot": int(len(keep))}


def build_run_summary(final: pd.DataFrame, runs: pd.DataFrame) -> dict:
    """Per-run quality summary keyed by short id where possible."""
    short = runs.set_index("run_id")["short_id"]
    out = {}
    for run_id, group in final.groupby("run_id"):
        g = group[group["loc_prod_after"] > 0]
        if g.empty:
            continue
        zero_t = g[g["loc_test_after"] == 0]
        out[short.get(run_id, run_id)] = {
            "median_test_prod_ratio": round(float(g["test_prod_ratio"].median()), 4),
            "utilities_zero_test": int(len(zero_t)),
            "zero_test_targets": zero_t["target"].tolist(),
        }
    return out


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    out_dir = repo_root / "analysis" / "charts" / "quality"
    summary_path = repo_root / "analysis" / "datasets" / "quality_summary.yaml"
    setup_style()
    d = load(repo_root)

    tasks_with_run = _assign_run_to_tasks(d["tasks"], d["runs"])
    final = _final_per_target_per_run(tasks_with_run)

    summary = {
        "n_runs_total": int(len(d["runs"])),
        "n_tasks_total": int(len(d["tasks"])),
        "tasks_assigned_to_run": int(tasks_with_run["run_id"].notna().sum()),
        "heatmap": chart_test_prod_ratio_heatmap(d, final, out_dir),
        "zero_tests_per_run": chart_utilities_with_zero_tests_per_run(d, final, out_dir),
        "median_drift": chart_median_test_prod_ratio_drift(d, final, out_dir),
        "diff_test_skip": chart_diff_test_skip_rate(d, out_dir),
        "trajectory": chart_test_addition_trajectory(d, tasks_with_run, out_dir),
        "ratio_distribution": chart_test_prod_ratio_distribution_per_run(d, final, out_dir),
        "runs": build_run_summary(final, d["runs"]),
    }
    with summary_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    print("=== Quality summary ===")
    for k, v in summary.items():
        if k == "runs":
            print(f"  runs: {len(v)} entries")
            continue
        print(f"  {k}: {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
