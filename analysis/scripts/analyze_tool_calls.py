#!/usr/bin/env python3
"""Deep tool-call charts for #5026 from tools.csv + task_turns.csv + task_retries.csv.

Coverage caveat (called out on every chart): only the ~185 task_ids that have
preserved transcripts (run-43 + Feb-Mar recovery via #5020) appear in tools.csv
and task_turns.csv. The other ~2,025 task_ids have only commit trailers and are
invisible to deep analysis.

Per-turn cost is computed from token counts using public Anthropic rates for
claude-opus-4-6 (USD per million tokens):

  input          15.00
  output         75.00
  cache_creation 30.00   (1-hour TTL approximation; transcripts mix 5m and 1h)
  cache_read      1.50

Sum over a task should approximate that task's stitch_invocations.cost_usd.
"""
from __future__ import annotations

import sys
from pathlib import Path

import matplotlib.pyplot as plt
import pandas as pd
import seaborn as sns
import yaml

OPUS_RATES = {
    "input": 15.00,
    "output": 75.00,
    "cache_creation": 30.00,
    "cache_read": 1.50,
}


def estimated_cost(row: pd.Series) -> float:
    return (
        row["tokens_input"] * OPUS_RATES["input"]
        + row["tokens_output"] * OPUS_RATES["output"]
        + row["cache_creation"] * OPUS_RATES["cache_creation"]
        + row["cache_read"] * OPUS_RATES["cache_read"]
    ) / 1_000_000


def load(repo_root: Path) -> dict:
    d = repo_root / "analysis" / "datasets"
    tools = pd.read_csv(d / "tools.csv")
    turns = pd.read_csv(d / "task_turns.csv")
    retries = pd.read_csv(d / "task_retries.csv")
    tasks = pd.read_csv(d / "tasks.csv")
    return {"tools": tools, "turns": turns, "retries": retries, "tasks": tasks}


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


def chart_tool_call_mix_per_task(d: dict, out_dir: Path) -> None:
    tools = d["tools"]
    pivot = tools.pivot_table(
        index="task_id", columns="tool_name", aggfunc="size", fill_value=0
    )
    pivot["__total__"] = pivot.sum(axis=1)
    top = pivot.sort_values("__total__", ascending=False).head(50).drop(columns="__total__")
    # Order tool columns by overall frequency
    col_order = pivot.drop(columns="__total__").sum().sort_values(ascending=False).index.tolist()
    top = top[col_order]

    fig, ax = plt.subplots(figsize=(14, 12))
    top.plot(kind="barh", stacked=True, ax=ax, colormap="tab20", edgecolor="white", linewidth=0.2)
    ax.invert_yaxis()
    ax.set_xlabel("tool calls")
    ax.set_ylabel("task_id")
    ax.legend(loc="lower right", ncol=2, fontsize=8)
    titled(
        ax, "Tool-call mix per task (top 50)",
        f"source: tools.csv, N={len(tools):,} tool calls across "
        f"{tools['task_id'].nunique()} task_ids with transcript coverage "
        f"(~8% of all task_ids).",
    )
    save(fig, out_dir, "tool_call_mix_per_task")


def chart_build_failure_loop_histogram(d: dict, out_dir: Path) -> None:
    bash = d["tools"][d["tools"]["tool_name"] == "Bash"]
    build_classes = ("go_build", "go_vet")
    builds = bash[bash["bash_command_class"].isin(build_classes)]
    failed = builds[builds["tool_result_is_error"] == True]  # noqa: E712
    per_task_failed = failed.groupby("task_id").size()
    per_task_total = builds.groupby("task_id").size()
    # Include zero-failure tasks that DID run a build
    per_task_failed = per_task_failed.reindex(per_task_total.index, fill_value=0)

    fig, ax = plt.subplots(figsize=(10, 6))
    counts = per_task_failed.value_counts().sort_index()
    bars = ax.bar(counts.index.astype(str), counts.values, color="#c44e52")
    for b, v in zip(bars, counts.values):
        ax.text(b.get_x() + b.get_width() / 2, v, f"{int(v)}", ha="center", va="bottom", fontsize=10)
    ax.set_xlabel("failed go_build / go_vet calls per task")
    ax.set_ylabel("number of task_ids (log)")
    ax.set_yscale("log")
    titled(
        ax, "Build-failure loops per task",
        f"source: tools.csv where bash_command_class in {build_classes} and is_error=True; "
        f"N={len(builds)} build calls across {len(per_task_total)} task_ids; "
        f"{int(per_task_failed.sum())} failures total.",
    )
    save(fig, out_dir, "build_failure_loop_histogram")


def chart_test_failure_loop_histogram(d: dict, out_dir: Path) -> None:
    bash = d["tools"][d["tools"]["tool_name"] == "Bash"]
    tests = bash[bash["bash_command_class"] == "go_test"]
    failed = tests[tests["tool_result_is_error"] == True]  # noqa: E712
    per_task_failed = failed.groupby("task_id").size()
    per_task_total = tests.groupby("task_id").size()
    per_task_failed = per_task_failed.reindex(per_task_total.index, fill_value=0)

    fig, ax = plt.subplots(figsize=(10, 6))
    counts = per_task_failed.value_counts().sort_index()
    bars = ax.bar(counts.index.astype(str), counts.values, color="#dd8452")
    for b, v in zip(bars, counts.values):
        ax.text(b.get_x() + b.get_width() / 2, v, f"{int(v)}", ha="center", va="bottom", fontsize=10)
    ax.set_xlabel("failed go_test calls per task")
    ax.set_ylabel("number of task_ids (log)")
    ax.set_yscale("log")
    titled(
        ax, "Test-failure loops per task",
        f"source: tools.csv where bash_command_class='go_test' and is_error=True; "
        f"N={len(tests)} test calls across {len(per_task_total)} task_ids; "
        f"{int(per_task_failed.sum())} failures total.",
    )
    save(fig, out_dir, "test_failure_loop_histogram")


def chart_file_reread_count(d: dict, out_dir: Path) -> None:
    """For each task, count file paths Read more than once."""
    reads = d["tools"][d["tools"]["tool_name"] == "Read"]
    per_task_files = (
        reads.groupby(["task_id", "tool_input_summary"]).size().reset_index(name="read_count")
    )
    rereads = per_task_files[per_task_files["read_count"] > 1]
    per_task_reread_files = rereads.groupby("task_id").size()
    # Include zero-reread tasks that did some Read
    all_read_tasks = reads["task_id"].unique()
    per_task_reread_files = per_task_reread_files.reindex(all_read_tasks, fill_value=0)

    fig, ax = plt.subplots(figsize=(10, 6))
    counts = per_task_reread_files.value_counts().sort_index()
    bars = ax.bar(counts.index.astype(str), counts.values, color="#4c72b0")
    for b, v in zip(bars, counts.values):
        ax.text(b.get_x() + b.get_width() / 2, v, f"{int(v)}", ha="center", va="bottom", fontsize=10)
    ax.set_xlabel("number of file paths Read more than once in a single task")
    ax.set_ylabel("number of task_ids (log)")
    ax.set_yscale("log")
    titled(
        ax, "File re-read counts per task",
        f"source: tools.csv where tool_name='Read'; N={len(reads):,} Read calls "
        f"across {len(all_read_tasks)} task_ids.",
    )
    save(fig, out_dir, "file_reread_count")


def chart_edit_write_churn_per_file(d: dict, out_dir: Path) -> None:
    """Top 30 (task_id, file_path) pairs by Edit/Write call count."""
    mods = d["tools"][d["tools"]["tool_name"].isin(["Edit", "Write"])]
    per_pair = (
        mods.groupby(["task_id", "tool_input_summary"]).size()
        .reset_index(name="edit_write_count")
        .sort_values("edit_write_count", ascending=False)
        .head(30)
    )
    per_pair["label"] = per_pair["task_id"].astype(str) + " :: " + per_pair["tool_input_summary"].apply(lambda p: Path(str(p)).name)
    fig, ax = plt.subplots(figsize=(12, 10))
    ax.barh(per_pair["label"][::-1], per_pair["edit_write_count"][::-1], color="#dd8452")
    ax.set_xlabel("Edit + Write calls on this file in this task")
    titled(
        ax, "Top 30 file churn (task_id :: filename)",
        f"source: tools.csv tool_name in (Edit, Write); N={len(mods)} modification calls.",
    )
    save(fig, out_dir, "edit_write_churn_per_file")


def chart_per_turn_cost_within_task(d: dict, out_dir: Path) -> None:
    """Small-multiples plot of per-turn estimated cost for top-10 most expensive tasks."""
    turns = d["turns"].copy()
    turns["est_cost_usd"] = turns.apply(estimated_cost, axis=1)
    per_task_cost = turns.groupby("task_id")["est_cost_usd"].sum().sort_values(ascending=False)
    top_ids = per_task_cost.head(10).index.tolist()

    # Mark turns where the previous turn had a tool error (proxy for "after a failure")
    tools = d["tools"]
    err_turns = (
        tools[tools["tool_result_is_error"] == True]  # noqa: E712
        .groupby(["task_id", "turn_index"]).size().reset_index(name="errs")
    )

    fig, axes = plt.subplots(2, 5, figsize=(18, 8), sharey=False)
    for ax, tid in zip(axes.flat, top_ids):
        sub = turns[turns["task_id"].astype(str) == str(tid)].sort_values("turn_index")
        ax.plot(sub["turn_index"], sub["est_cost_usd"], color="#4c72b0", marker="o", markersize=3, linewidth=1)
        # Highlight turns where the same turn had an error result
        err_for_task = err_turns[err_turns["task_id"].astype(str) == str(tid)]
        for _, e in err_for_task.iterrows():
            t = sub[sub["turn_index"] == e["turn_index"]]
            if len(t) > 0:
                ax.scatter(t["turn_index"], t["est_cost_usd"], color="#c44e52", s=30, zorder=3)
        ax.set_title(f"task {tid} (${per_task_cost[tid]:.2f})", fontsize=10)
        ax.set_xlabel("turn_index", fontsize=9)
        ax.set_ylabel("est cost (USD)", fontsize=9)
        ax.tick_params(axis="both", labelsize=8)
    fig.suptitle(
        "Per-turn estimated cost for top-10 most expensive tasks "
        "(red dots = turn had tool error)",
        y=1.02, fontsize=14,
    )
    fig.tight_layout()
    save(fig, out_dir, "per_turn_cost_within_task")


def chart_tool_efficiency_scatter(d: dict, out_dir: Path) -> None:
    """For each task: total tool calls vs total task cost, colored by productivity."""
    tools = d["tools"]
    retries = d["retries"]

    per_task_tools = tools.groupby("task_id").size().reset_index(name="tool_call_count")
    per_task_tools["task_id"] = per_task_tools["task_id"].astype(str)
    retries["task_id"] = retries["task_id"].astype(str)
    df = per_task_tools.merge(
        retries[["task_id", "total_cost_usd", "productive_invocations", "invocation_count"]],
        on="task_id", how="inner",
    )
    df["productive"] = df["productive_invocations"] > 0

    fig, ax = plt.subplots(figsize=(12, 8))
    for is_prod, color, label in [(True, "#4c9a4c", "productive (loc>0 in some attempt)"),
                                   (False, "#c44e52", "all attempts zero-LOC")]:
        sub = df[df["productive"] == is_prod]
        ax.scatter(sub["tool_call_count"], sub["total_cost_usd"], color=color, alpha=0.6,
                   s=40 + sub["invocation_count"] * 20, label=label, edgecolor="black", linewidth=0.3)
    ax.set_xlabel("total tool calls in this task")
    ax.set_ylabel("total cost (USD) across all invocations")
    ax.legend(loc="upper left")
    titled(
        ax, "Tool-call count vs task cost",
        f"source: tools.csv groupby task_id join task_retries.csv; N={len(df)} tasks; "
        f"bubble size proportional to invocation_count.",
    )
    save(fig, out_dir, "tool_efficiency_scatter")


def write_summary(d: dict, out_path: Path) -> dict:
    tools = d["tools"]
    turns = d["turns"]
    bash = tools[tools["tool_name"] == "Bash"]
    builds = bash[bash["bash_command_class"].isin(["go_build", "go_vet"])]
    tests = bash[bash["bash_command_class"] == "go_test"]
    builds_failed = builds[builds["tool_result_is_error"] == True]  # noqa: E712
    tests_failed = tests[tests["tool_result_is_error"] == True]  # noqa: E712

    per_task_tool_count = tools.groupby("task_id").size()
    n_tasks_with_build_loops = (builds_failed.groupby("task_id").size() >= 2).sum()
    n_tasks_with_test_loops = (tests_failed.groupby("task_id").size() >= 2).sum()

    summary = {
        "task_ids_with_transcripts": int(tools["task_id"].nunique()),
        "total_tool_calls": int(len(tools)),
        "total_bash_calls": int(len(bash)),
        "go_build_or_vet_calls": int(len(builds)),
        "go_build_or_vet_failed": int(len(builds_failed)),
        "go_test_calls": int(len(tests)),
        "go_test_failed": int(len(tests_failed)),
        "mean_tool_calls_per_task": round(float(per_task_tool_count.mean()), 1),
        "median_tool_calls_per_task": int(per_task_tool_count.median()),
        "p95_tool_calls_per_task": int(per_task_tool_count.quantile(0.95)),
        "max_tool_calls_per_task": int(per_task_tool_count.max()),
        "tasks_with_build_loops_2plus": int(n_tasks_with_build_loops),
        "tasks_with_test_loops_2plus": int(n_tasks_with_test_loops),
        "total_turns_recorded": int(len(turns)),
    }
    with out_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    return summary


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent.parent
    out_dir = repo_root / "analysis" / "charts" / "tool_calls"
    summary_path = repo_root / "analysis" / "datasets" / "tool_call_summary.yaml"
    setup_style()

    d = load(repo_root)
    chart_tool_call_mix_per_task(d, out_dir)
    chart_build_failure_loop_histogram(d, out_dir)
    chart_test_failure_loop_histogram(d, out_dir)
    chart_file_reread_count(d, out_dir)
    chart_edit_write_churn_per_file(d, out_dir)
    chart_per_turn_cost_within_task(d, out_dir)
    chart_tool_efficiency_scatter(d, out_dir)

    summary = write_summary(d, summary_path)
    print("=== Tool-call summary ===")
    for k, v in summary.items():
        print(f"  {k:42s}  {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
