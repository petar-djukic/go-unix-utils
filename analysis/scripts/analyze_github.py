#!/usr/bin/env python3
"""GitHub workflow charts for #5028.

Reads issues.csv, prs.csv, runs.csv, tasks.csv. Produces 7 charts under
analysis/charts/github/ plus github_summary.yaml.

Coverage caveat: of 4,888 issues, only 159 PRs were ever opened (most
generation tasks closed via direct commit, not PR). So the PR-derived
charts (cycle time, size, PR volume) are sparse and skewed toward the
recent analysis-pipeline PRs rather than the bulk of generation work.
"""
from __future__ import annotations

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


def _short_run_id(run_id: str) -> str:
    import re
    if not isinstance(run_id, str):
        return ""
    m = re.search(r"(run[\d.]+[a-z]*)$|generation-run-([\d]+)$", run_id)
    if m:
        return m.group(1) or f"run{m.group(2)}"
    return run_id[:14]


def load(repo_root: Path) -> dict:
    d = repo_root / "analysis" / "datasets"
    issues = pd.read_csv(d / "issues.csv")
    prs = pd.read_csv(d / "prs.csv")
    runs = pd.read_csv(d / "runs.csv")
    tasks = pd.read_csv(d / "tasks.csv")
    issues["created_at"] = pd.to_datetime(issues["created_at"], utc=True, errors="coerce")
    issues["closed_at"] = pd.to_datetime(issues["closed_at"], utc=True, errors="coerce")
    prs["created_at"] = pd.to_datetime(prs["created_at"], utc=True, errors="coerce")
    prs["closed_at"] = pd.to_datetime(prs["closed_at"], utc=True, errors="coerce")
    prs["merged_at"] = pd.to_datetime(prs["merged_at"], utc=True, errors="coerce")
    runs["start_at"] = pd.to_datetime(runs["start_at"], utc=True)
    runs["end_at"] = pd.to_datetime(runs["end_at"], utc=True, errors="coerce")
    runs = runs.sort_values("start_at").reset_index(drop=True)
    runs["run_order"] = runs.index + 1
    runs["short_id"] = runs["run_id"].apply(_short_run_id)
    tasks["commit_date"] = pd.to_datetime(tasks["commit_date"], utc=True)
    return {"issues": issues, "prs": prs, "runs": runs, "tasks": tasks}


def _label_task_issue_close(issues: pd.DataFrame, prs: pd.DataFrame) -> pd.Series:
    """Bucket each task issue into a close reason.

    - 'open': issue still open
    - 'closed_via_pr': issue body references a merged PR on its number (PR
      head_ref contains the issue's number) — rare in this repo since most
      task issues close via direct commit
    - 'closed_no_pr': closed but no matching PR found (direct commit close,
      cobbler auto-close, or manual close)
    """
    pr_issue_numbers: set[int] = set()
    for _, p in prs.iterrows():
        head = str(p.get("head_ref") or "")
        import re
        m = re.search(r"gh-(\d+)", head)
        if m:
            pr_issue_numbers.add(int(m.group(1)))

    def classify(row: pd.Series) -> str:
        if row["state"] == "OPEN":
            return "open"
        if int(row["number"]) in pr_issue_numbers:
            return "closed_via_pr"
        return "closed_no_pr"

    return issues.apply(classify, axis=1)


def chart_task_issue_close_reason(d: dict, out_dir: Path) -> dict:
    issues = d["issues"]
    ti = issues[issues["is_task_issue"]].copy()
    ti["close_bucket"] = _label_task_issue_close(ti, d["prs"])
    counts = ti["close_bucket"].value_counts()
    order = ["closed_via_pr", "closed_no_pr", "open"]
    counts = counts.reindex(order, fill_value=0)

    fig, ax = plt.subplots(figsize=(9, 6))
    palette = ["#55a868", "#4c72b0", "#dd8452"]
    bars = ax.bar(counts.index, counts.values, color=palette)
    total = int(counts.sum())
    for bar, v in zip(bars, counts.values):
        pct = v / total * 100 if total else 0
        ax.text(bar.get_x() + bar.get_width() / 2, v + total * 0.005,
                f"{v}\n({pct:.1f}%)", ha="center", va="bottom", fontsize=10)
    ax.set_ylabel("task issue count")
    titled(
        ax, "Task issue close reasons",
        f"source: issues.csv where is_task_issue; PR linkage via head_ref `gh-<number>`; N={total}",
    )
    save(fig, out_dir, "task_issue_close_reason")
    return {k: int(v) for k, v in counts.to_dict().items()}


def chart_problem_report_rate_per_run(d: dict, out_dir: Path) -> dict:
    issues = d["issues"]
    runs = d["runs"]
    sub = issues[issues["is_problem_report"]].copy()
    sub["run"] = sub["cobbler_run_id"].fillna("(no-run-label)")

    # match cobbler_run_id to runs.run_id when possible
    short_map = runs.set_index("run_id")["short_id"].to_dict()
    counts = sub["run"].apply(lambda r: short_map.get(r, _short_run_id(r))).value_counts()

    fig, ax = plt.subplots(figsize=(12, 6))
    if counts.empty:
        ax.text(0.5, 0.5, "no problem reports found", ha="center", va="center",
                transform=ax.transAxes, fontsize=14)
    else:
        ax.bar(range(len(counts)), counts.values, color="#c44e52")
        ax.set_xticks(range(len(counts)))
        ax.set_xticklabels(counts.index, rotation=60, ha="right", fontsize=8)
    ax.set_ylabel("problem-report issue count")
    titled(
        ax, "Problem-report issues by cobbler run",
        f"source: issues.csv where is_problem_report; total={int(counts.sum())}",
    )
    save(fig, out_dir, "problem_report_rate_per_run")
    return {"total_problem_reports": int(counts.sum()),
            "by_run": {str(k): int(v) for k, v in counts.to_dict().items()}}


def chart_pr_cycle_time_distribution(d: dict, out_dir: Path) -> dict:
    prs = d["prs"]
    sub = prs[(prs["merged"]) & (prs["cycle_time_hours"].notna())].copy()
    if sub.empty:
        return {}
    fig, ax = plt.subplots(figsize=(11, 6))
    ax.hist(sub["cycle_time_hours"], bins=40, color="#4c72b0", edgecolor="white")
    med = float(sub["cycle_time_hours"].median())
    p95 = float(sub["cycle_time_hours"].quantile(0.95))
    ax.axvline(med, color="#c44e52", linestyle="--", linewidth=1.5, label=f"median {med:.2f}h")
    ax.axvline(p95, color="#dd8452", linestyle="--", linewidth=1.5, label=f"p95 {p95:.2f}h")
    ax.set_xlabel("PR cycle time (hours; created → merged)")
    ax.set_ylabel("PR count")
    ax.legend(loc="upper right")
    titled(
        ax, "PR cycle time distribution",
        f"source: prs.csv where merged; N={len(sub)} PRs",
    )
    save(fig, out_dir, "pr_cycle_time_distribution")
    return {
        "n_merged_prs": int(len(sub)),
        "median_cycle_time_hours": round(med, 4),
        "p95_cycle_time_hours": round(p95, 4),
    }


def chart_pr_size_distribution(d: dict, out_dir: Path) -> dict:
    prs = d["prs"]
    sub = prs[(prs["additions"].notna()) | (prs["deletions"].notna())].copy()
    sub["size"] = sub["additions"].fillna(0) + sub["deletions"].fillna(0)
    sub = sub[sub["size"] > 0]
    if sub.empty:
        return {}
    fig, ax = plt.subplots(figsize=(11, 6))
    bins = np.logspace(0, np.log10(max(sub["size"].max(), 10)), 30)
    ax.hist(sub["size"].clip(lower=1), bins=bins, color="#8172b3", edgecolor="white", log=True)
    ax.set_xscale("log")
    med = float(sub["size"].median())
    ax.axvline(med, color="#c44e52", linestyle="--", linewidth=1.5, label=f"median {med:.0f} LOC")
    ax.set_xlabel("PR size = additions + deletions (LOC, log scale)")
    ax.set_ylabel("PR count (log scale)")
    ax.legend(loc="upper right")
    titled(
        ax, "PR size distribution",
        f"source: prs.csv additions+deletions; N={len(sub)} PRs with positive size",
    )
    save(fig, out_dir, "pr_size_distribution")
    return {
        "n_prs_with_size": int(len(sub)),
        "median_pr_size_loc": round(med, 1),
        "p95_pr_size_loc": round(float(sub["size"].quantile(0.95)), 1),
    }


def _assign_run_to_tasks(tasks: pd.DataFrame, runs: pd.DataFrame) -> pd.DataFrame:
    t = tasks.copy()
    t["run_id"] = None
    t["run_end_state"] = None
    for _, run in runs.iterrows():
        end = run["end_at"] if pd.notna(run["end_at"]) else tasks["commit_date"].max() + pd.Timedelta(seconds=1)
        mask = (t["commit_date"] >= run["start_at"]) & (t["commit_date"] < end) & t["run_id"].isna()
        t.loc[mask, "run_id"] = run["run_id"]
        t.loc[mask, "run_end_state"] = run["end_state"]
    return t


def chart_abandoned_vs_completed_comparison(d: dict, out_dir: Path) -> dict:
    tasks_with_run = _assign_run_to_tasks(d["tasks"], d["runs"])
    sub = tasks_with_run.dropna(subset=["run_end_state"]).copy()
    sub = sub[sub["task_subtype"] == "stitch"].copy()
    sub["bucket"] = sub["run_end_state"].map({
        "merged": "completed", "finished": "completed",
        "abandoned": "abandoned", "in_progress": "in_progress",
    }).fillna("other")
    # focus on completed vs abandoned
    sub = sub[sub["bucket"].isin(["completed", "abandoned"])]

    fig, axes = plt.subplots(1, 3, figsize=(16, 6))
    for ax, col, title in zip(
        axes,
        ["cost_usd", "duration_seconds", "loc_prod_delta"],
        ["cost (USD)", "duration (s)", "prod LOC delta"],
    ):
        order = ["completed", "abandoned"]
        sns.boxplot(data=sub, x="bucket", y=col, ax=ax, order=order,
                    palette=["#55a868", "#c44e52"], fliersize=2,
                    hue="bucket", legend=False)
        ax.set_xlabel("")
        ax.set_ylabel(title)
        if col == "cost_usd":
            ax.set_ylim(0, sub[col].quantile(0.99) * 1.05)
        elif col == "duration_seconds":
            ax.set_ylim(0, sub[col].quantile(0.99) * 1.05)
    fig.suptitle("Stitch task profiles in completed vs abandoned runs", fontsize=14, y=1.0)
    fig.text(0.5, -0.02,
             f"source: tasks.csv joined to runs.csv end_state; "
             f"completed N={int((sub['bucket']=='completed').sum())}, "
             f"abandoned N={int((sub['bucket']=='abandoned').sum())}",
             ha="center", fontsize=9, color="#555")
    plt.tight_layout()
    save(fig, out_dir, "abandoned_vs_completed_comparison")

    return {
        "completed_tasks": int((sub["bucket"] == "completed").sum()),
        "abandoned_tasks": int((sub["bucket"] == "abandoned").sum()),
        "completed_median_cost": round(float(sub.loc[sub["bucket"] == "completed", "cost_usd"].median()), 4),
        "abandoned_median_cost": round(float(sub.loc[sub["bucket"] == "abandoned", "cost_usd"].median()), 4),
    }


def chart_pr_cycle_time_over_time(d: dict, out_dir: Path) -> dict:
    prs = d["prs"]
    sub = prs[(prs["merged"]) & (prs["cycle_time_hours"].notna())].copy()
    sub = sub.sort_values("created_at")
    fig, ax = plt.subplots(figsize=(13, 6))
    if not sub.empty:
        ax.plot(sub["created_at"], sub["cycle_time_hours"], marker="o", linewidth=0.8,
                color="#4c72b0", markersize=4)
        rolling = sub["cycle_time_hours"].rolling(window=20, min_periods=5).median()
        ax.plot(sub["created_at"], rolling, color="#c44e52", linewidth=2,
                label="20-PR rolling median")
        ax.legend(loc="upper right")
    ax.set_xlabel("PR creation date")
    ax.set_ylabel("cycle time (hours)")
    ax.set_yscale("symlog", linthresh=0.1)
    titled(
        ax, "PR cycle time over time",
        f"source: prs.csv where merged; N={len(sub)} merged PRs",
    )
    plt.setp(ax.get_xticklabels(), rotation=45, ha="right", fontsize=8)
    save(fig, out_dir, "generation_pr_cycle_time_over_time")
    return {"n_prs_plotted": int(len(sub))}


def chart_pr_volume_per_run(d: dict, out_dir: Path) -> dict:
    prs = d["prs"]
    runs = d["runs"]
    # match each PR to a run by created_at falling within window
    prs = prs.copy()
    prs["run_id"] = None
    prs["run_end_state"] = None
    for _, run in runs.iterrows():
        end = run["end_at"] if pd.notna(run["end_at"]) else prs["created_at"].max() + pd.Timedelta(seconds=1)
        if pd.isna(run["start_at"]):
            continue
        mask = (prs["created_at"] >= run["start_at"]) & (prs["created_at"] < end) & prs["run_id"].isna()
        prs.loc[mask, "run_id"] = run["run_id"]
        prs.loc[mask, "run_end_state"] = run["end_state"]
    grouped = (
        prs.dropna(subset=["run_id"])
        .groupby(["run_id", "run_end_state"]).size().reset_index(name="pr_count")
    )
    short = runs.set_index("run_id")["short_id"]
    grouped["short"] = grouped["run_id"].map(short)

    fig, ax = plt.subplots(figsize=(13, 6))
    if grouped.empty:
        ax.text(0.5, 0.5, "no PR-to-run matches", ha="center", va="center",
                transform=ax.transAxes, fontsize=14)
    else:
        states = grouped["run_end_state"].unique()
        palette = {"merged": "#55a868", "finished": "#4c72b0",
                   "abandoned": "#c44e52", "in_progress": "#dd8452"}
        for state in states:
            sub = grouped[grouped["run_end_state"] == state]
            ax.bar(sub["short"], sub["pr_count"], color=palette.get(state, "#888"),
                   label=state)
        ax.legend(title="run end_state", loc="upper left", fontsize=9)
        plt.setp(ax.get_xticklabels(), rotation=60, ha="right", fontsize=8)
    ax.set_ylabel("PR count")
    titled(
        ax, "PR count per run, by run end_state",
        f"source: prs.csv created_at matched to runs.csv windows; N={int(grouped['pr_count'].sum())} PRs matched",
    )
    save(fig, out_dir, "pr_volume_per_run")
    return {
        "n_prs_matched_to_run": int(grouped["pr_count"].sum()),
        "n_runs_with_prs": int(grouped["run_id"].nunique()),
    }


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    out_dir = repo_root / "analysis" / "charts" / "github"
    summary_path = repo_root / "analysis" / "datasets" / "github_summary.yaml"
    setup_style()
    d = load(repo_root)

    summary = {
        "total_issues": int(len(d["issues"])),
        "total_prs": int(len(d["prs"])),
        "task_issues": int(d["issues"]["is_task_issue"].sum()),
        "problem_reports": int(d["issues"]["is_problem_report"].sum()),
        "task_issue_close_reasons": chart_task_issue_close_reason(d, out_dir),
        "problem_report_per_run": chart_problem_report_rate_per_run(d, out_dir),
        "pr_cycle_time": chart_pr_cycle_time_distribution(d, out_dir),
        "pr_size": chart_pr_size_distribution(d, out_dir),
        "abandoned_vs_completed": chart_abandoned_vs_completed_comparison(d, out_dir),
        "pr_cycle_over_time": chart_pr_cycle_time_over_time(d, out_dir),
        "pr_volume_per_run": chart_pr_volume_per_run(d, out_dir),
    }
    with summary_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    print("=== GitHub workflow summary ===")
    for k, v in summary.items():
        print(f"  {k}: {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
