#!/usr/bin/env python3
"""Waste charts for #5024: zero-LOC tasks, retries, timeouts, abandonment, rate-limit.

Joins tasks.csv, runs.csv, task_attempts.csv, utilities.csv, runs.yaml.

Each chart is saved as both PNG (300dpi) and SVG under analysis/charts/waste/.
A waste_summary.yaml with headline numbers is written to analysis/datasets/.
"""
from __future__ import annotations

import sys
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.ticker as mticker
import pandas as pd
import seaborn as sns
import yaml

MAX_TIME_SEC = 1500  # current generator timeout, per CLAUDE.md / configuration.yaml


def load(repo_root: Path) -> dict:
    d = repo_root / "analysis" / "datasets"
    tasks = pd.read_csv(d / "tasks.csv")
    tasks["commit_date"] = pd.to_datetime(tasks["commit_date"], utc=True)
    runs = pd.read_csv(d / "runs.csv")
    runs["start_at"] = pd.to_datetime(runs["start_at"], utc=True)
    runs["end_at"] = pd.to_datetime(runs["end_at"], utc=True)
    attempts = pd.read_csv(d / "task_attempts.csv")
    attempts["first_commit_at"] = pd.to_datetime(attempts["first_commit_at"], utc=True)
    utilities = pd.read_csv(d / "utilities.csv")
    retries = pd.read_csv(d / "task_retries.csv")
    retries["first_started_at"] = pd.to_datetime(retries["first_started_at"], utc=True)
    with (d / "runs.yaml").open() as f:
        run_reports = yaml.safe_load(f)
    return {
        "tasks": tasks,
        "runs": runs,
        "attempts": attempts,
        "utilities": utilities,
        "retries": retries,
        "reports": run_reports,
    }


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
    """Set a title with a smaller subtitle below it, avoiding overlap."""
    ax.set_title(title, pad=24, loc="center", fontsize=14)
    ax.text(
        0, 1.01, subtitle,
        transform=ax.transAxes, fontsize=9, color="#555", va="bottom",
    )


def chart_zero_loc_per_run(d: dict, out_dir: Path) -> None:
    runs = d["runs"].copy()
    runs = runs.sort_values("start_at").reset_index(drop=True)
    runs["productive_count"] = runs["task_count"] - runs["zero_loc_count"]
    runs["short_id"] = runs["run_id"].str.slice(0, 24)

    fig, ax = plt.subplots(figsize=(14, 6))
    x = range(len(runs))
    ax.bar(x, runs["productive_count"], label="productive", color="#4c9a4c")
    ax.bar(x, runs["zero_loc_count"], bottom=runs["productive_count"], label="zero-LOC", color="#c44e52")
    ax.set_xlabel("run (chronological)")
    ax.set_ylabel("task count")
    ax.set_xticks(list(x))
    ax.set_xticklabels(runs["short_id"], rotation=90, fontsize=7)
    ax.legend(loc="upper left")
    titled(
        ax, "Zero-LOC vs productive tasks per run",
        f"source: runs.csv, N={len(runs)} runs; total zero-LOC tasks: "
        f"{int(runs['zero_loc_count'].sum())} / {int(runs['task_count'].sum())}",
    )
    save(fig, out_dir, "zero_loc_per_run")


def chart_retry_attempt_distribution(d: dict, out_dir: Path) -> None:
    """Real retry distribution from task_retries.csv (one row per task_id with
    transcript coverage). Coverage is limited to ~185 of 2,210 task_ids — runs
    whose .cobbler/history/ was either preserved (run-43) or recovered from git
    (#5020). The other ~2,025 task_ids have no log-side retry visibility.
    """
    retries = d["retries"]
    fig, ax = plt.subplots(figsize=(10, 6))
    counts = retries["invocation_count"].value_counts().sort_index()
    bars = ax.bar(counts.index.astype(str), counts.values, color="#4c72b0")
    ax.set_xlabel("invocation_count (real retries from *-stitch-stats.yaml)")
    ax.set_ylabel("number of task_ids (log)")
    ax.set_yscale("log")
    for bar, val in zip(bars, counts.values):
        ax.text(bar.get_x() + bar.get_width() / 2, val, f"{int(val):,}",
                ha="center", va="bottom", fontsize=10)
    total_wasted = retries["wasted_cost_usd"].sum()
    n_retried = int((retries["invocation_count"] > 1).sum())
    titled(
        ax, "Distribution of Claude Code invocations per task_id",
        f"source: task_retries.csv, N={len(retries)} task_ids with transcript coverage "
        f"(~8% of all task_ids); {n_retried} retried; log-based wasted cost ${total_wasted:.2f}",
    )
    save(fig, out_dir, "retry_attempt_distribution")


def chart_wasted_cost_pareto(d: dict, out_dir: Path) -> None:
    """Log-based wasted cost grouped by target.

    task_retries.csv is keyed by task_id (string). We join with tasks.csv to
    pick up `target` (only resolves for numeric task_ids; slug-kind tasks have
    no commit and stay unjoined → bucketed as `<unmapped>`).
    """
    retries = d["retries"][d["retries"]["wasted_cost_usd"] > 0].copy()
    tasks = d["tasks"]
    # tasks.csv task_id is int; cast to string for join
    tasks_targets = tasks[["task_id", "target"]].copy()
    tasks_targets["task_id"] = tasks_targets["task_id"].astype(str)
    tasks_targets = tasks_targets.drop_duplicates(subset=["task_id"], keep="last")
    retries = retries.merge(tasks_targets, on="task_id", how="left")
    retries["target"] = retries["target"].fillna("<unmapped>")

    per_target = (
        retries.groupby("target")["wasted_cost_usd"]
        .sum()
        .sort_values(ascending=False)
        .head(30)
    )
    fig, ax = plt.subplots(figsize=(12, 8))
    if len(per_target) == 0:
        ax.text(0.5, 0.5, "(no log-based wasted cost > 0)", transform=ax.transAxes, ha="center")
    else:
        ax.barh(per_target.index[::-1], per_target.values[::-1], color="#c44e52")
    ax.set_xlabel("wasted cost (USD)")
    total = float(d["retries"]["wasted_cost_usd"].sum())
    titled(
        ax, "Log-based wasted cost by target (top 30)",
        f"source: task_retries.csv joined to tasks.csv; total log-based wasted "
        f"${total:.2f} across {len(d['retries'])} task_ids with transcript coverage",
    )
    save(fig, out_dir, "wasted_cost_pareto_by_utility")


def chart_task_duration_pdf(d: dict, out_dir: Path) -> None:
    """KDE of task durations from tasks.csv — answers "how long does Claude
    take to finish a task" rather than "when did long durations occur"."""
    tasks = d["tasks"]
    durations = tasks["duration_seconds"].astype(float)
    p50 = float(durations.quantile(0.50))
    p95 = float(durations.quantile(0.95))
    p99 = float(durations.quantile(0.99))
    mean = float(durations.mean())

    fig, ax = plt.subplots(figsize=(12, 6))
    sns.kdeplot(x=durations, ax=ax, fill=True, color="#4c72b0", bw_adjust=0.6, clip=(0, durations.max()))
    ax.axvline(p50, color="#666", linestyle="--", alpha=0.7, label=f"p50={p50:.0f}s")
    ax.axvline(p95, color="#dd8452", linestyle="--", alpha=0.7, label=f"p95={p95:.0f}s")
    ax.axvline(p99, color="#c44e52", linestyle="--", alpha=0.7, label=f"p99={p99:.0f}s")
    ax.axvline(MAX_TIME_SEC, color="#c44e52", linestyle="-", alpha=0.4, label=f"max_time_sec={MAX_TIME_SEC}")
    ax.set_xlim(left=0, right=min(MAX_TIME_SEC * 1.05, durations.max() * 1.05))
    ax.set_xlabel("task duration (seconds)")
    ax.set_ylabel("density")
    ax.legend(loc="upper right")
    titled(
        ax, "Distribution of task duration (time to finish a task)",
        f"source: tasks.csv, N={len(durations):,}; mean={mean:.0f}s, p50={p50:.0f}s, "
        f"p95={p95:.0f}s; tasks killed at {MAX_TIME_SEC}s do not appear here.",
    )
    save(fig, out_dir, "task_duration_pdf")


def chart_timeout_map(d: dict, out_dir: Path) -> None:
    tasks = d["tasks"].sort_values("commit_date").reset_index(drop=True)
    near_threshold = 1200  # 80% of timeout; visualizes the long-tail
    fig, ax = plt.subplots(figsize=(14, 7))
    colors = tasks["target_kind"].map({"cmd": "#4c72b0", "pkg": "#dd8452", "other": "#aaaaaa"}).fillna("#aaaaaa")
    ax.scatter(tasks.index, tasks["duration_seconds"], c=colors, alpha=0.4, s=10)

    near = tasks[tasks["duration_seconds"] >= near_threshold]
    if len(near) > 0:
        ax.scatter(near.index, near["duration_seconds"], color="#c44e52", s=40,
                   edgecolor="black", linewidth=0.5,
                   label=f"≥ {near_threshold}s near-timeout ({len(near)} tasks)")
        for _, row in near.nlargest(8, "duration_seconds").iterrows():
            ax.annotate(row["target"], (row.name, row["duration_seconds"]),
                        fontsize=8, alpha=0.9, xytext=(3, 3), textcoords="offset points")

    ax.axhline(MAX_TIME_SEC, color="#c44e52", linestyle="--", alpha=0.6,
               label=f"max_time_sec={MAX_TIME_SEC} (kills are not captured here)")
    ax.set_xlabel("commit index (chronological)")
    ax.set_ylabel("duration (seconds)")
    ax.legend(loc="upper left")
    titled(
        ax, "Task duration over chronological commit order",
        f"source: tasks.csv, N={len(tasks):,}; near-timeout (≥{near_threshold}s): {len(near)}. "
        f"Tasks killed at the {MAX_TIME_SEC}s timeout produce no commit.",
    )
    save(fig, out_dir, "timeout_map")


def chart_abandoned_run_cost(d: dict, out_dir: Path) -> None:
    runs = d["runs"]
    abandoned = runs[runs["end_state"] == "abandoned"].sort_values("start_at")
    fig, ax = plt.subplots(figsize=(12, 6))
    if len(abandoned) == 0:
        ax.text(0.5, 0.5, "(no abandoned runs)", transform=ax.transAxes, ha="center")
    else:
        ax.bar(range(len(abandoned)), abandoned["task_count"], color="#c44e52")
        ax.set_xticks(range(len(abandoned)))
        ax.set_xticklabels(abandoned["run_id"].str.slice(0, 28), rotation=45, ha="right", fontsize=9)
        for i, v in enumerate(abandoned["task_count"]):
            ax.text(i, v, f"{int(v)}", ha="center", va="bottom", fontsize=9)
    ax.set_xlabel("run_id")
    ax.set_ylabel("committed task_count")
    total_cost = abandoned["total_cost_usd"].sum()
    titled(
        ax, "Tasks committed in abandoned runs",
        f"source: runs.csv where end_state='abandoned', N={len(abandoned)}; "
        f"committed cost in trailers: ${total_cost:.2f}. Most abandoned runs aborted before "
        f"any stitch task committed; trailer-side cost understates real burn.",
    )
    save(fig, out_dir, "abandoned_run_cost")


def chart_rate_limit_per_run(d: dict, out_dir: Path) -> None:
    reports = [r for r in d["reports"] if r.get("rate_limited_seconds") is not None]
    fig, ax = plt.subplots(figsize=(12, 6))
    if not reports:
        ax.text(0.5, 0.5, "(no rate-limit data parsed)", transform=ax.transAxes, ha="center")
    else:
        reports_sorted = sorted(reports, key=lambda r: r["report_id"])
        ids = [r["report_id"] for r in reports_sorted]
        secs = [r["rate_limited_seconds"] / 60 for r in reports_sorted]  # minutes for readability
        ax.bar(ids, secs, color="#dd8452")
        for x, v in zip(ids, secs):
            ax.text(x, v, f"{v:.0f}m", ha="center", va="bottom", fontsize=9)
    ax.set_xlabel("report_id")
    ax.set_ylabel("rate-limited time (minutes)")
    titled(
        ax, "Rate-limited time per run (where reported)",
        f"source: runs.yaml, N={len(reports)} of {len(d['reports'])} reports parse a rate-limit field",
    )
    save(fig, out_dir, "rate_limit_time_per_run")


def chart_retry_rate_by_hour(d: dict, out_dir: Path) -> None:
    tasks = d["tasks"].copy()
    tasks["hour_utc"] = tasks["commit_date"].dt.hour
    tasks["zero_loc"] = (tasks["loc_prod_delta"] == 0) & (tasks["loc_test_delta"] == 0)
    by_hour = tasks.groupby("hour_utc").agg(
        n=("commit_sha", "count"), zero=("zero_loc", "sum")
    )
    by_hour["zero_loc_rate"] = by_hour["zero"] / by_hour["n"]

    fig, ax = plt.subplots(figsize=(12, 6))
    ax.scatter(by_hour.index, by_hour["zero_loc_rate"], s=by_hour["n"] / 5, color="#4c72b0", alpha=0.7)
    for h, row in by_hour.iterrows():
        ax.annotate(f"n={int(row['n'])}", (h, row["zero_loc_rate"]),
                    fontsize=8, alpha=0.8, xytext=(0, 8), textcoords="offset points", ha="center")
    ax.set_xlabel("hour (UTC)")
    ax.set_ylabel("zero-LOC tasks / total tasks at this hour")
    ax.set_xticks(range(24))
    ax.yaxis.set_major_formatter(mticker.PercentFormatter(1.0))
    titled(
        ax, "Zero-LOC rate vs hour of day (UTC)",
        f"source: tasks.csv, N={len(tasks):,}; bubble size proportional to task count",
    )
    save(fig, out_dir, "retry_rate_vs_time_of_day")


def write_summary(d: dict, out_path: Path) -> dict:
    runs = d["runs"]
    attempts = d["attempts"]
    retries = d["retries"]
    abandoned = runs[runs["end_state"] == "abandoned"]
    rate_limited_total = sum(
        (r.get("rate_limited_seconds") or 0) for r in d["reports"]
    )
    durations = d["tasks"]["duration_seconds"].astype(float)
    summary = {
        "total_tasks": int(d["tasks"].shape[0]),
        "total_unique_task_ids": int(attempts.shape[0]),
        "task_duration_p50_s": int(durations.quantile(0.50)),
        "task_duration_p95_s": int(durations.quantile(0.95)),
        "task_duration_p99_s": int(durations.quantile(0.99)),
        "task_duration_max_s": int(durations.max()),
        "zero_loc_task_count": int(((d["tasks"]["loc_prod_delta"] == 0) & (d["tasks"]["loc_test_delta"] == 0)).sum()),
        "git_commit_count_distribution": {int(k): int(v) for k, v in attempts["git_commit_count"].value_counts().sort_index().to_dict().items()},
        "git_wasted_cost_proxy_usd": round(float(attempts["git_wasted_cost_proxy"].sum()), 2),
        "log_based_retry_coverage_task_ids": int(len(retries)),
        "log_based_invocation_count_distribution": {int(k): int(v) for k, v in retries["invocation_count"].value_counts().sort_index().to_dict().items()},
        "log_based_retry_count_task_ids": int((retries["invocation_count"] > 1).sum()),
        "log_based_max_invocations_for_a_single_task": int(retries["invocation_count"].max()),
        "log_based_total_wasted_cost_usd": round(float(retries["wasted_cost_usd"].sum()), 2),
        "near_timeout_task_count": int((d["tasks"]["duration_seconds"] >= 1200).sum()),
        "abandoned_run_count": int(len(abandoned)),
        "abandoned_run_committed_cost_usd": round(float(abandoned["total_cost_usd"].sum()), 2),
        "total_rate_limited_seconds_reported": int(rate_limited_total),
        "total_rate_limited_minutes_reported": round(rate_limited_total / 60, 1),
    }
    with out_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    return summary


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent.parent
    out_dir = repo_root / "analysis" / "charts" / "waste"
    summary_path = repo_root / "analysis" / "datasets" / "waste_summary.yaml"
    setup_style()

    d = load(repo_root)
    chart_zero_loc_per_run(d, out_dir)
    chart_retry_attempt_distribution(d, out_dir)
    chart_wasted_cost_pareto(d, out_dir)
    chart_task_duration_pdf(d, out_dir)
    chart_timeout_map(d, out_dir)
    chart_abandoned_run_cost(d, out_dir)
    chart_rate_limit_per_run(d, out_dir)
    chart_retry_rate_by_hour(d, out_dir)

    summary = write_summary(d, summary_path)

    print("=== Waste summary ===")
    for k, v in summary.items():
        print(f"  {k:42s}  {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
