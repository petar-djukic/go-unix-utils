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
    invocations = pd.read_csv(d / "stitch_invocations.csv")
    invocations["started_at"] = pd.to_datetime(invocations["started_at"], utc=True)
    invocations = invocations.sort_values(["task_id", "started_at"])
    invocations["attempt_idx"] = invocations.groupby("task_id").cumcount() + 1
    invocations["productive"] = (invocations["loc_prod_delta"] > 0) | (invocations["loc_test_delta"] > 0)
    invocations["effective_success"] = (invocations["status"] == "success") & invocations["productive"]
    outcomes_path = d / "bash_outcomes.csv"
    outcomes = pd.read_csv(outcomes_path) if outcomes_path.exists() else pd.DataFrame()
    return {
        "tools": tools, "turns": turns, "retries": retries,
        "tasks": tasks, "invocations": invocations, "outcomes": outcomes,
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


def _pass_at_k_table(invocations: pd.DataFrame) -> pd.DataFrame:
    n = invocations["task_id"].nunique()
    max_k = int(invocations["attempt_idx"].max())
    rows = []
    for k in range(1, max_k + 1):
        sub = invocations[invocations["attempt_idx"] <= k]
        strict = sub.groupby("task_id")["effective_success"].any().sum() / n
        status = sub.groupby("task_id")["status"].apply(lambda s: (s == "success").any()).sum() / n
        rows.append({"k": k, "pass_at_k_strict": strict, "pass_at_k_status": status})
    return pd.DataFrame(rows)


def chart_pass_at_k(d: dict, out_dir: Path) -> None:
    """pass@k = fraction of task_ids with at least one productive success
    in the first k orchestrator-level invocations. Two definitions:
    - strict: status='success' AND loc_delta > 0
    - status-only: status='success' (productive or not)
    """
    inv = d["invocations"]
    df = _pass_at_k_table(inv)
    n = inv["task_id"].nunique()
    failure_rate = (1 - df.iloc[0]["pass_at_k_strict"]) * 100

    fig, ax = plt.subplots(figsize=(10, 6))
    x = df["k"].to_numpy(dtype=float)
    width = 0.36
    bars1 = ax.bar(x - width / 2, df["pass_at_k_strict"] * 100, width=width,
                   label="pass@k (strict: success + productive)", color="#4c72b0")
    bars2 = ax.bar(x + width / 2, df["pass_at_k_status"] * 100, width=width,
                   label="pass@k (status=success only)", color="#dd8452")
    for k_, s_, t_ in zip(df["k"], df["pass_at_k_strict"], df["pass_at_k_status"]):
        ax.text(k_ - width / 2, s_ * 100, f"{s_*100:.1f}%", ha="center", va="bottom", fontsize=9)
        ax.text(k_ + width / 2, t_ * 100, f"{t_*100:.1f}%", ha="center", va="bottom", fontsize=9)
    ax.set_xticks(df["k"])
    ax.set_xlabel("k (orchestrator invocations allowed)")
    ax.set_ylabel("pass@k (%)")
    ax.set_ylim(0, 105)
    ax.legend(loc="lower right")
    titled(
        ax, "pass@k — fraction of task_ids solved within k orchestrator invocations",
        f"source: stitch_invocations.csv, N={n} task_ids with transcript coverage; "
        f"failure-to-one-shot (strict) = {failure_rate:.1f}%",
    )
    save(fig, out_dir, "pass_at_k")


def chart_per_attempt_success_rate(d: dict, out_dir: Path) -> None:
    """P(success+productive | reached attempt K). Drops fast — once Claude
    fails attempt 1, attempt 2 only succeeds 36% of the time, and lower
    after that."""
    inv = d["invocations"]
    rows = []
    for k, group in inv.groupby("attempt_idx"):
        n = len(group)
        strict = group["effective_success"].sum()
        status = (group["status"] == "success").sum()
        rows.append({
            "k": int(k), "reached": n,
            "strict_rate": strict / n if n else 0,
            "status_rate": status / n if n else 0,
        })
    df = pd.DataFrame(rows)

    fig, ax = plt.subplots(figsize=(10, 6))
    ax.plot(df["k"], df["strict_rate"] * 100, marker="o", color="#4c72b0",
            label="P(success+productive | reached attempt K)")
    ax.plot(df["k"], df["status_rate"] * 100, marker="s", color="#dd8452",
            label="P(status=success | reached attempt K)")
    for k_, s_, t_, n_ in zip(df["k"], df["strict_rate"], df["status_rate"], df["reached"]):
        ax.annotate(f"n={n_}", (k_, max(s_, t_) * 100), textcoords="offset points",
                    xytext=(0, 8), ha="center", fontsize=8, color="#555")
    ax.set_xticks(df["k"])
    ax.set_xlabel("attempt index K")
    ax.set_ylabel("success rate at this attempt (%)")
    ax.set_ylim(0, 105)
    ax.legend(loc="lower left")
    titled(
        ax, "Conditional success rate at attempt K",
        f"source: stitch_invocations.csv; n=number of invocations that reached this attempt index",
    )
    save(fig, out_dir, "per_attempt_success_rate")


def _session_attempt_index(tools: pd.DataFrame) -> pd.DataFrame:
    """Rank session_uuids per task_id by earliest timestamp.

    Returns a DataFrame with columns task_id, session_uuid, session_attempt
    where session_attempt=1 is the first stitch invocation for that task.
    """
    t = tools.copy()
    t["timestamp"] = pd.to_datetime(t["timestamp"], utc=True, errors="coerce")
    sess = t.groupby(["task_id", "session_uuid"], as_index=False)["timestamp"].min()
    sess = sess.sort_values(["task_id", "timestamp"])
    sess["session_attempt"] = sess.groupby("task_id").cumcount() + 1
    return sess[["task_id", "session_uuid", "session_attempt"]]


def chart_first_pass_build_test_lint(d: dict, out_dir: Path) -> None:
    """First-pass success rate within each task's first stitch invocation,
    grouped by phase (build / test / lint). For each (task, phase), we look
    at the earliest call of that phase in attempt 1; success means the
    output did not match a phase-specific failure pattern (see
    extract_bash_outcomes.py for the patterns)."""
    outcomes = d["outcomes"]
    if outcomes.empty:
        return
    sess = _session_attempt_index(d["tools"])
    o = outcomes.merge(sess, on=["task_id", "session_uuid"], how="inner")
    a1 = o[o["session_attempt"] == 1]
    a1 = a1.sort_values(["task_id", "phase", "turn_index", "tool_call_index"])
    first = a1.drop_duplicates(["task_id", "phase"], keep="first")

    phases = ["build", "test", "lint"]
    rows = []
    for phase in phases:
        sub = first[first["phase"] == phase]
        n = len(sub)
        passed = int((~sub["failed"]).sum())
        rate = passed / n * 100 if n else 0.0
        rows.append({"phase": phase, "n": n, "passed": passed, "rate": rate})
    df = pd.DataFrame(rows)

    fig, ax = plt.subplots(figsize=(9, 6))
    colors = {"build": "#4c72b0", "test": "#dd8452", "lint": "#55a868"}
    bars = ax.bar(df["phase"], df["rate"], color=[colors[p] for p in df["phase"]])
    for bar, n, rate, passed in zip(bars, df["n"], df["rate"], df["passed"]):
        ax.text(
            bar.get_x() + bar.get_width() / 2, rate + 1.5,
            f"{rate:.1f}%\nn={n} ({passed}/{n})",
            ha="center", va="bottom", fontsize=10, color="#333",
        )
    ax.set_ylabel("first-call success rate (%)")
    ax.set_ylim(0, 110)
    ax.set_yticks(range(0, 101, 20))
    titled(
        ax, "First-pass success rate by phase (attempt 1 only)",
        "source: bash_outcomes.csv; first call of each phase per task in its first stitch invocation",
    )
    save(fig, out_dir, "first_pass_build_test_lint")


def chart_cost_by_phase(d: dict, out_dir: Path) -> None:
    """Total estimated cost attributable to {build, test, lint} bash calls,
    split into bottom = attempt-1 cost, top = attempt-2+ cost.

    Cost attribution model:
      - Per-turn cost from token counts via OPUS_RATES (input/output/cache).
      - Each turn's cost is divided equally among the Bash tool calls in that
        turn, so the per-call share is turn_cost / tool_count_in_turn for
        Bash calls (most turns have a single Bash call).
      - A bash call's share is summed into its phase bucket; non-Bash and
        non-{build,test,lint} bash calls are dropped from this view.
      - Attempt bucket: attempt_1 if the call's session_uuid is the earliest
        session for that task_id, else attempt_2_plus.
    """
    outcomes = d["outcomes"]
    tools = d["tools"]
    turns = d["turns"]
    if outcomes.empty or turns.empty:
        return

    turns = turns.copy()
    turns["turn_cost"] = turns.apply(estimated_cost, axis=1)
    bash_per_turn = (
        tools[tools["tool_name"] == "Bash"]
        .groupby(["task_id", "session_uuid", "turn_index"])
        .size()
        .reset_index(name="bash_in_turn")
    )
    cost_per_turn = turns[
        ["task_id", "session_uuid", "turn_index", "turn_cost"]
    ].merge(bash_per_turn, on=["task_id", "session_uuid", "turn_index"], how="left")
    cost_per_turn["bash_in_turn"] = cost_per_turn["bash_in_turn"].fillna(0)
    cost_per_turn = cost_per_turn[cost_per_turn["bash_in_turn"] > 0].copy()
    cost_per_turn["per_call_cost"] = (
        cost_per_turn["turn_cost"] / cost_per_turn["bash_in_turn"]
    )

    sess = _session_attempt_index(tools)
    o = outcomes.merge(
        cost_per_turn[
            ["task_id", "session_uuid", "turn_index", "per_call_cost"]
        ],
        on=["task_id", "session_uuid", "turn_index"],
        how="inner",
    ).merge(sess, on=["task_id", "session_uuid"], how="left")
    o["attempt_bucket"] = o["session_attempt"].apply(
        lambda x: "attempt_1" if x == 1 else "attempt_2_plus"
    )

    pivot = o.pivot_table(
        index="phase", columns="attempt_bucket",
        values="per_call_cost", aggfunc="sum", fill_value=0,
    ).reindex(["build", "test", "lint"]).fillna(0)
    for col in ("attempt_1", "attempt_2_plus"):
        if col not in pivot.columns:
            pivot[col] = 0.0

    fig, ax = plt.subplots(figsize=(9, 6))
    x = range(len(pivot.index))
    bottom = pivot["attempt_1"].values
    top = pivot["attempt_2_plus"].values
    ax.bar(x, bottom, color="#4c72b0", label="attempt 1")
    ax.bar(x, top, bottom=bottom, color="#c44e52", label="attempts 2+")
    for i, (b, t) in enumerate(zip(bottom, top)):
        total = b + t
        if b > 0:
            pct = b / total * 100 if total else 0.0
            ax.text(i, b / 2, f"${b:.2f} ({pct:.1f}%)",
                    ha="center", va="center", color="white", fontsize=9)
        if t > 0:
            pct = t / total * 100 if total else 0.0
            ax.text(i, b + t / 2, f"${t:.2f} ({pct:.1f}%)",
                    ha="center", va="center", color="white", fontsize=9)
        ax.text(i, total + 0.02 * max(bottom + top, default=1),
                f"${total:.2f}", ha="center", va="bottom", fontsize=10,
                color="#333")
    ax.set_xticks(list(x))
    ax.set_xticklabels(pivot.index)
    ax.set_ylabel("estimated cost (USD)")
    ax.legend(loc="upper right")
    titled(
        ax, "Cost by phase, split by attempt",
        "per-turn cost from tokens x Opus rates; turn cost split equally across Bash calls in the turn",
    )
    save(fig, out_dir, "cost_by_phase")


RUN44_INV_ID = "gh-5059-run44"
RUN44_TRANSCRIPT_ID = "run44"


def _cost_by_category_and_outcome(d: dict, transcript_run_id: str, inv_run_id: str) -> pd.DataFrame:
    """Full cost decomposition for a run, split by category and task outcome."""
    turns = d["turns"][d["turns"]["run_id"] == transcript_run_id].copy()
    tools = d["tools"][d["tools"]["run_id"] == transcript_run_id].copy()
    inv = d["invocations"][d["invocations"]["run_id"] == inv_run_id].copy()
    if turns.empty or inv.empty:
        return pd.DataFrame()

    turns["task_id_str"] = turns["task_id"].astype(str)
    tools["task_id_str"] = tools["task_id"].astype(str)
    inv["task_id_str"] = inv["task_id"].astype(str)
    turns["est_cost"] = turns.apply(estimated_cost, axis=1)

    recorded_per_task = inv.groupby("task_id_str")["cost_usd"].sum()
    estimated_per_task = turns.groupby("task_id_str")["est_cost"].sum()
    common = recorded_per_task.index.intersection(estimated_per_task.index)
    scale = (recorded_per_task.loc[common] / estimated_per_task.loc[common].replace(0, 1)).fillna(0)

    # Determine outcome per task: success if any invocation succeeded with LOC > 0
    task_outcome = inv.groupby("task_id_str").apply(
        lambda g: "success" if ((g["status"] == "success") & ((g["loc_prod_delta"] > 0) | (g["loc_test_delta"] > 0))).any() else "failed"
    )

    turns = turns[turns["task_id_str"].isin(common)].copy()
    turns["cal_cost"] = turns["est_cost"] * turns["task_id_str"].map(scale).fillna(0)
    turns["outcome"] = turns["task_id_str"].map(task_outcome).fillna("failed")

    # Reasoning turns (no tools)
    reasoning = turns[~turns["had_tool_use"]].copy()
    reasoning["category"] = "reasoning"
    reasoning_rows = reasoning.groupby(["category", "outcome"])["cal_cost"].sum().reset_index(name="cost")

    # Tool turns: split cost across tools in the turn
    tool_turns = turns[turns["had_tool_use"]].copy()
    tool_turns["per_call"] = tool_turns["cal_cost"] / tool_turns["tool_count_in_turn"].replace(0, 1)
    tools_merged = tools.merge(
        tool_turns[["task_id", "session_uuid", "turn_index", "per_call", "outcome"]],
        on=["task_id", "session_uuid", "turn_index"],
        how="inner",
    )
    tools_merged["category"] = tools_merged.apply(
        lambda r: _categorize_tool(r["tool_name"], r["bash_command_class"]), axis=1
    )
    tool_rows = tools_merged.groupby(["category", "outcome"])["per_call"].sum().reset_index(name="cost")

    return pd.concat([reasoning_rows, tool_rows], ignore_index=True)


def chart_cost_decomposition(d: dict, out_dir: Path) -> None:
    """Where the cost goes, split by task outcome (success vs failure)."""
    cost_df = _cost_by_category_and_outcome(d, RUN44_TRANSCRIPT_ID, RUN44_INV_ID)
    if cost_df.empty:
        return

    inv = d["invocations"]
    r44_inv = inv[inv["run_id"] == RUN44_INV_ID]
    n_tasks = r44_inv["task_id"].nunique()

    cat_order = ["reasoning", "read_code", "write_code", "build", "test", "lint", "other_bash", "other_tool"]
    cat_labels = ["“reasoning”", "read code", "write code", "build", "test", "lint", "other bash", "other"]

    pivot = cost_df.pivot_table(index="category", columns="outcome", values="cost", aggfunc="sum", fill_value=0)
    for col in ["success", "failed"]:
        if col not in pivot.columns:
            pivot[col] = 0.0
    pivot = pivot.reindex([c for c in cat_order if c in pivot.index]).fillna(0)
    labels_used = [cat_labels[cat_order.index(c)] for c in pivot.index]

    grand_total = pivot["success"].sum() + pivot["failed"].sum()
    total_success = pivot["success"].sum()
    total_failed = pivot["failed"].sum()

    fig, ax = plt.subplots(figsize=(13, 7))
    x = range(len(pivot))
    s_vals = pivot["success"].values
    f_vals = pivot["failed"].values

    ax.bar(x, s_vals, color="#4c72b0", label="successful tasks")
    ax.bar(x, f_vals, bottom=s_vals, color="#c44e52", label="failed tasks")

    for i, (s, f) in enumerate(zip(s_vals, f_vals)):
        total = s + f
        if total > 0:
            pct = total / grand_total * 100
            ax.text(i, total + grand_total * 0.01, f"${total:.1f} ({pct:.0f}%)",
                    ha="center", va="bottom", fontsize=10, color="#333")
    ax.set_xticks(list(x))
    ax.set_xticklabels(labels_used, rotation=30, ha="right")
    ax.set_ylabel("estimated cost (USD)")
    ax.legend(loc="upper right", fontsize=10)

    ax.text(
        0.97, 0.82,
        f"success cost: ${total_success:.0f} ({total_success/grand_total*100:.0f}%)\n"
        f"failure cost: ${total_failed:.0f} ({total_failed/grand_total*100:.0f}%)",
        transform=ax.transAxes, fontsize=11, va="top", ha="right",
        bbox={"boxstyle": "round,pad=0.4", "facecolor": "#f8f8f8", "edgecolor": "#ccc"},
    )

    titled(
        ax, "Run 44: where the cost goes",
        f"N={n_tasks} tasks with transcripts | total ${grand_total:.0f} | "
        f"split by task outcome (success = status success + productive LOC)",
    )
    save(fig, out_dir, "cost_decomposition")


def _time_by_category_and_outcome(d: dict, transcript_run_id: str, inv_run_id: str) -> pd.DataFrame:
    """Wall-clock time decomposition for a run, split by category and task outcome.

    Time attribution: gap from each tool call to the next (within the same
    session) is attributed to the earlier call's category. Time unaccounted
    for by inter-tool gaps (session start to first tool, last tool to session
    end) is attributed to 'reasoning'.
    """
    tools = d["tools"][d["tools"]["run_id"] == transcript_run_id].copy()
    inv = d["invocations"][d["invocations"]["run_id"] == inv_run_id].copy()
    if tools.empty or inv.empty:
        return pd.DataFrame()

    tools["task_id_str"] = tools["task_id"].astype(str)
    inv["task_id_str"] = inv["task_id"].astype(str)
    tools["ts"] = pd.to_datetime(tools["timestamp"], utc=True)
    tools = tools.sort_values(["task_id", "session_uuid", "ts"])
    tools["next_ts"] = tools.groupby(["task_id", "session_uuid"])["ts"].shift(-1)
    tools["gap_s"] = (tools["next_ts"] - tools["ts"]).dt.total_seconds().fillna(0)

    task_outcome = inv.groupby("task_id_str").apply(
        lambda g: "success" if ((g["status"] == "success") & ((g["loc_prod_delta"] > 0) | (g["loc_test_delta"] > 0))).any() else "failed"
    )

    common = set(tools["task_id_str"].unique()) & set(inv["task_id_str"].unique())
    tools = tools[tools["task_id_str"].isin(common)].copy()
    tools["outcome"] = tools["task_id_str"].map(task_outcome).fillna("failed")
    tools["category"] = tools.apply(
        lambda r: _categorize_tool(r["tool_name"], r["bash_command_class"]), axis=1
    )

    tool_rows = tools.groupby(["category", "outcome"])["gap_s"].sum().reset_index(name="time_s")

    total_duration = float(inv[inv["task_id_str"].isin(common)]["duration_s"].sum())
    tool_time = tools["gap_s"].sum()
    reasoning_time = max(total_duration - tool_time, 0)

    success_dur = float(inv[inv["task_id_str"].isin(
        set(task_outcome[task_outcome == "success"].index) & common
    )]["duration_s"].sum())
    fail_dur = float(inv[inv["task_id_str"].isin(
        set(task_outcome[task_outcome == "failed"].index) & common
    )]["duration_s"].sum())
    success_tool = float(tools[tools["outcome"] == "success"]["gap_s"].sum())
    fail_tool = float(tools[tools["outcome"] == "failed"]["gap_s"].sum())
    success_reasoning = max(success_dur - success_tool, 0)
    fail_reasoning = max(fail_dur - fail_tool, 0)

    reasoning_rows = pd.DataFrame([
        {"category": "reasoning", "outcome": "success", "time_s": success_reasoning},
        {"category": "reasoning", "outcome": "failed", "time_s": fail_reasoning},
    ])
    result = pd.concat([reasoning_rows, tool_rows], ignore_index=True)
    result["total_duration"] = total_duration
    return result


def chart_time_decomposition(d: dict, out_dir: Path) -> None:
    """Where the wall-clock time goes, split by task outcome."""
    time_df = _time_by_category_and_outcome(d, RUN44_TRANSCRIPT_ID, RUN44_INV_ID)
    if time_df.empty:
        return

    inv = d["invocations"]
    r44_inv = inv[inv["run_id"] == RUN44_INV_ID]
    n_tasks = r44_inv["task_id"].nunique()
    total_duration = time_df["total_duration"].iloc[0]

    cat_order = CATEGORY_ORDER
    cat_labels = ["“reasoning”", "read code", "write code", "build", "test", "lint", "other bash", "other"]

    pivot = time_df.pivot_table(index="category", columns="outcome", values="time_s", aggfunc="sum", fill_value=0)
    for col in ["success", "failed"]:
        if col not in pivot.columns:
            pivot[col] = 0.0
    pivot = pivot.reindex([c for c in cat_order if c in pivot.index]).fillna(0)
    labels_used = [cat_labels[cat_order.index(c)] for c in pivot.index]

    total_s = pivot["success"].sum() + pivot["failed"].sum()
    total_success = pivot["success"].sum()
    total_failed = pivot["failed"].sum()

    fig, ax = plt.subplots(figsize=(13, 7))
    x = range(len(pivot))
    s_vals = pivot["success"].values / 60
    f_vals = pivot["failed"].values / 60

    ax.bar(x, s_vals, color="#4c72b0", label="successful tasks")
    ax.bar(x, f_vals, bottom=s_vals, color="#c44e52", label="failed tasks")

    for i, (s, f) in enumerate(zip(s_vals, f_vals)):
        total_min = s + f
        if total_min > 0:
            pct = (s + f) * 60 / total_s * 100
            ax.text(i, total_min + total_s / 60 * 0.01, f"{total_min:.0f}m ({pct:.0f}%)",
                    ha="center", va="bottom", fontsize=10, color="#333")
    ax.set_xticks(list(x))
    ax.set_xticklabels(labels_used, rotation=30, ha="right")
    ax.set_ylabel("wall-clock time (minutes)")
    ax.legend(loc="upper right", fontsize=10)

    ax.text(
        0.97, 0.82,
        f"success time: {total_success/60:.0f}m ({total_success/total_s*100:.0f}%)\n"
        f"failure time: {total_failed/60:.0f}m ({total_failed/total_s*100:.0f}%)",
        transform=ax.transAxes, fontsize=11, va="top", ha="right",
        bbox={"boxstyle": "round,pad=0.4", "facecolor": "#f8f8f8", "edgecolor": "#ccc"},
    )

    titled(
        ax, "Run 44: where the time goes",
        f"N={n_tasks} tasks with transcripts | total {total_duration/3600:.1f}h | "
        f"time from inter-tool-call gaps; unattributed time → reasoning",
    )
    save(fig, out_dir, "time_decomposition")


def chart_unit_economics(d: dict, out_dir: Path) -> None:
    """What the cost produces: cost per KLOC, cost per requirement, cost per utility."""
    inv = d["invocations"]
    tasks = d["tasks"]
    r44_inv = inv[inv["run_id"] == RUN44_INV_ID].copy()
    if r44_inv.empty:
        return

    r44_inv["task_id_str"] = r44_inv["task_id"].astype(str)
    total_cost = float(r44_inv["cost_usd"].sum())

    success = r44_inv[
        (r44_inv["status"] == "success")
        & ((r44_inv["loc_prod_delta"] > 0) | (r44_inv["loc_test_delta"] > 0))
    ]
    last_success = success.sort_values("started_at").drop_duplicates("task_id", keep="last")
    loc_prod = int(last_success[last_success["loc_prod_delta"] > 0]["loc_prod_delta"].sum())
    loc_test = int(last_success[last_success["loc_test_delta"] > 0]["loc_test_delta"].sum())

    tasks = tasks.copy()
    tasks["task_id_str"] = tasks["task_id"].astype(str)
    success_ids = set(last_success["task_id_str"].unique())
    r44_stitch = tasks[(tasks["task_id_str"].isin(success_ids)) & (tasks["task_subtype"] == "stitch")]
    distinct_targets = r44_stitch["target"].nunique()

    def _count_reqs(r):
        if pd.isna(r):
            return 0
        return len([x.strip() for x in str(r).split(",") if x.strip()])

    reqs_completed = int(r44_stitch["requirements"].apply(_count_reqs).sum())

    metrics = {
        "prod code": total_cost / (loc_prod / 1000) if loc_prod > 0 else 0,
        "test code": total_cost / (loc_test / 1000) if loc_test > 0 else 0,
        "requirement": total_cost / reqs_completed if reqs_completed > 0 else 0,
        "utility": total_cost / distinct_targets if distinct_targets > 0 else 0,
    }
    raw_values = {
        "prod code": f"{loc_prod/1000:.1f} KLOC",
        "test code": f"{loc_test/1000:.1f} KLOC",
        "requirement": f"{reqs_completed} reqs",
        "utility": f"{distinct_targets} utilities",
    }

    fig, ax = plt.subplots(figsize=(11, 7))
    names = list(metrics.keys())
    values = [metrics[n] for n in names]
    colors = ["#4c72b0", "#55a868", "#dd8452", "#8172b3"]
    bars = ax.bar(names, values, color=colors, edgecolor="white", linewidth=1.5)

    for bar, val, name in zip(bars, values, names):
        ax.text(
            bar.get_x() + bar.get_width() / 2, val + max(values) * 0.02,
            f"${val:.2f}",
            ha="center", va="bottom", fontsize=13, fontweight="bold", color="#333",
        )
        ax.text(
            bar.get_x() + bar.get_width() / 2, val / 2,
            raw_values[name],
            ha="center", va="center", fontsize=10, color="white",
        )

    labels = [f"$/KLOC\n{n}" for n in names[:2]] + [f"$/req\n{names[2]}", f"$/utility\n{names[3]}"]
    ax.set_xticks(range(len(names)))
    ax.set_xticklabels(labels, fontsize=11)
    ax.set_ylabel("cost (USD)")

    ax.text(
        0.97, 0.95,
        f"total spend: ${total_cost:.0f}",
        transform=ax.transAxes, fontsize=12, va="top", ha="right",
        bbox={"boxstyle": "round,pad=0.4", "facecolor": "#f8f8f8", "edgecolor": "#ccc"},
    )

    titled(
        ax, "Run 44: what the cost produces",
        f"N={r44_inv['task_id'].nunique()} tasks | ${total_cost:.0f} total | "
        f"{loc_prod+loc_test} LOC generated across {distinct_targets} utilities",
    )
    save(fig, out_dir, "unit_economics")


RUN43_RUN_ID = "gh-4994-run43"

CATEGORY_ORDER = [
    "reasoning", "read_code", "write_code",
    "build", "test", "lint",
    "other_bash", "other_tool",
]


def _categorize_tool(tool_name: str, bash_command_class) -> str:
    if tool_name == "Read":
        return "read_code"
    if tool_name in ("Edit", "Write", "NotebookEdit"):
        return "write_code"
    if tool_name == "Bash":
        if bash_command_class == "go_build":
            return "build"
        if bash_command_class == "go_test":
            return "test"
        if bash_command_class in ("go_vet", "lint"):
            return "lint"
        return "other_bash"
    return "other_tool"


def _calibrated_run43_costs(d: dict) -> tuple[pd.DataFrame, float, int, int]:
    """Per-task calibrated cost for run-43, broken out by category.

    For each task_id in run-43:
      1. Sum estimated per-turn cost (across all that task's sessions).
      2. Sum recorded `cost_usd` from stitch_invocations across the task's
         attempts.
      3. Scale every turn's estimated cost by recorded / estimated.

    Per-task scaling, rather than per-session, sidesteps the matching problem
    when an attempt has zero recorded cost (early failure with no real work)
    but a separate transcript covers a successful retry. Reasoning turns
    (no tools) get the whole calibrated turn cost; tool turns split it
    equally across the tool calls in the turn.

    Returns: (rows DataFrame with columns category, cost; recorded total
    summed across calibrated tasks; tasks_matched; tasks_total).
    """
    turns = d["turns"][d["turns"]["run_id"] == RUN43_RUN_ID].copy()
    tools = d["tools"][d["tools"]["run_id"] == RUN43_RUN_ID].copy()
    inv = d["invocations"][d["invocations"]["run_id"] == RUN43_RUN_ID].copy()
    if turns.empty or inv.empty:
        return pd.DataFrame(columns=["category", "cost"]), 0.0, 0, 0

    turns["task_id_str"] = turns["task_id"].astype(str)
    tools["task_id_str"] = tools["task_id"].astype(str)
    inv["task_id_str"] = inv["task_id"].astype(str)
    turns["est_cost"] = turns.apply(estimated_cost, axis=1)

    recorded_per_task = inv.groupby("task_id_str")["cost_usd"].sum()
    estimated_per_task = turns.groupby("task_id_str")["est_cost"].sum()
    common = recorded_per_task.index.intersection(estimated_per_task.index)
    scale = (recorded_per_task.loc[common] / estimated_per_task.loc[common]).fillna(0)
    tasks_total = int(inv["task_id_str"].nunique())
    tasks_matched = int(len(common))

    turns = turns[turns["task_id_str"].isin(common)].copy()
    turns["cal_turn_cost"] = turns["est_cost"] * turns["task_id_str"].map(scale).fillna(0)

    reasoning_cost = float(turns.loc[~turns["had_tool_use"], "cal_turn_cost"].sum())

    tool_turns = turns[turns["had_tool_use"]][[
        "task_id", "session_uuid", "turn_index",
        "cal_turn_cost", "tool_count_in_turn",
    ]]
    tools_cal = tools.merge(
        tool_turns, on=["task_id", "session_uuid", "turn_index"], how="inner"
    )
    tools_cal["per_call"] = (
        tools_cal["cal_turn_cost"] / tools_cal["tool_count_in_turn"].replace(0, pd.NA)
    ).fillna(0)
    tools_cal["category"] = tools_cal.apply(
        lambda r: _categorize_tool(r["tool_name"], r["bash_command_class"]), axis=1
    )
    by_cat = tools_cal.groupby("category")["per_call"].sum()

    rows = [{"category": "reasoning", "cost": reasoning_cost}]
    for cat in CATEGORY_ORDER:
        if cat == "reasoning":
            continue
        rows.append({"category": cat, "cost": float(by_cat.get(cat, 0.0))})
    df = pd.DataFrame(rows)

    recorded_total = float(recorded_per_task.loc[common].sum())
    return df, recorded_total, tasks_matched, tasks_total


def chart_total_cost_pie(d: dict, out_dir: Path) -> None:
    """Pie chart of total cost decomposition for run-43 only.

    Run-43 is the only run with the complete preserved history (105 history
    files, all log+stats+prompt+report quadruplets paired). Per-session
    calibration scales each turn's token-derived cost so the session sums
    match the recorded `cost_usd`. Categories cover reasoning turns plus
    every tool kind, so the slices sum to the recorded total.
    """
    df, recorded_total, tasks_matched, tasks_total = _calibrated_run43_costs(d)
    if df.empty or recorded_total == 0:
        return
    df = df[df["cost"] > 0].sort_values("cost", ascending=False).reset_index(drop=True)
    palette = sns.color_palette("colorblind", n_colors=len(df))

    fig, ax = plt.subplots(figsize=(11, 7))
    total = float(df["cost"].sum())
    wedges, _ = ax.pie(
        df["cost"], labels=None, colors=palette, startangle=90,
        wedgeprops={"linewidth": 1, "edgecolor": "white"},
    )
    legend_labels = [
        f"{c}  ${v:.2f}  ({v/total*100:.1f}%)"
        for c, v in zip(df["category"], df["cost"])
    ]
    ax.legend(
        wedges, legend_labels,
        loc="center left", bbox_to_anchor=(1.02, 0.5),
        fontsize=10, frameon=False, title="category  cost  share",
        title_fontsize=10,
    )
    ax.set_aspect("equal")
    titled(
        ax,
        f"Run-43 cost decomposition (recorded ${recorded_total:.2f})",
        f"per-task calibration to stitch_invocations.cost_usd; "
        f"tasks matched {tasks_matched}/{tasks_total}; "
        f"categories sum to recorded total",
    )
    save(fig, out_dir, "total_cost_pie")


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

    inv = d["invocations"]
    pak = _pass_at_k_table(inv)
    pak_strict = {f"pass_at_{int(r['k'])}_strict": round(float(r["pass_at_k_strict"]), 4) for _, r in pak.iterrows()}
    pak_status = {f"pass_at_{int(r['k'])}_status": round(float(r["pass_at_k_status"]), 4) for _, r in pak.iterrows()}

    pass_at_phase = {}
    cost_by_phase = {}
    outcomes = d["outcomes"]
    if not outcomes.empty:
        sess = _session_attempt_index(tools)
        o = outcomes.merge(sess, on=["task_id", "session_uuid"], how="inner")
        a1 = o[o["session_attempt"] == 1].sort_values(
            ["task_id", "phase", "turn_index", "tool_call_index"]
        )
        first = a1.drop_duplicates(["task_id", "phase"], keep="first")
        for phase in ("build", "test", "lint"):
            sub = first[first["phase"] == phase]
            n = int(len(sub))
            passed = int((~sub["failed"]).sum())
            pass_at_phase[phase] = {
                "n": n, "passed": passed,
                "rate": round(passed / n, 4) if n else 0.0,
            }

        turns_local = turns.copy()
        turns_local["turn_cost"] = turns_local.apply(estimated_cost, axis=1)
        bash_per_turn = (
            tools[tools["tool_name"] == "Bash"]
            .groupby(["task_id", "session_uuid", "turn_index"]).size()
            .reset_index(name="bash_in_turn")
        )
        cpt = turns_local[
            ["task_id", "session_uuid", "turn_index", "turn_cost"]
        ].merge(bash_per_turn, on=["task_id", "session_uuid", "turn_index"], how="left")
        cpt["bash_in_turn"] = cpt["bash_in_turn"].fillna(0)
        cpt = cpt[cpt["bash_in_turn"] > 0].copy()
        cpt["per_call_cost"] = cpt["turn_cost"] / cpt["bash_in_turn"]
        cb = outcomes.merge(
            cpt[["task_id", "session_uuid", "turn_index", "per_call_cost"]],
            on=["task_id", "session_uuid", "turn_index"], how="inner",
        ).merge(sess, on=["task_id", "session_uuid"], how="left")
        cb["attempt_bucket"] = cb["session_attempt"].apply(
            lambda x: "attempt_1" if x == 1 else "attempt_2_plus"
        )
        for phase in ("build", "test", "lint"):
            sub = cb[cb["phase"] == phase]
            cost_by_phase[phase] = {
                "attempt_1": round(float(sub.loc[sub["attempt_bucket"] == "attempt_1", "per_call_cost"].sum()), 4),
                "attempt_2_plus": round(float(sub.loc[sub["attempt_bucket"] == "attempt_2_plus", "per_call_cost"].sum()), 4),
            }
            cost_by_phase[phase]["total"] = round(
                cost_by_phase[phase]["attempt_1"] + cost_by_phase[phase]["attempt_2_plus"], 4
            )

    summary = {
        "task_ids_with_transcripts": int(tools["task_id"].nunique()),
        "task_ids_with_invocations": int(inv["task_id"].nunique()),
        "max_observed_attempts": int(inv["attempt_idx"].max()),
        "failure_to_one_shot_rate_strict": round(float(1 - pak.iloc[0]["pass_at_k_strict"]), 4),
        "failure_to_one_shot_rate_status": round(float(1 - pak.iloc[0]["pass_at_k_status"]), 4),
        **pak_strict,
        **pak_status,
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
        "pass_at_phase": pass_at_phase,
        "cost_by_phase": cost_by_phase,
    }

    cost_df, recorded_total, tasks_matched, tasks_total = _calibrated_run43_costs(d)
    if not cost_df.empty:
        summary["run43_cost_decomposition"] = {
            "recorded_total_usd": round(recorded_total, 4),
            "tasks_matched": tasks_matched,
            "tasks_total": tasks_total,
            "by_category": {
                row["category"]: round(float(row["cost"]), 4)
                for _, row in cost_df.iterrows()
            },
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
    chart_pass_at_k(d, out_dir)
    chart_per_attempt_success_rate(d, out_dir)
    chart_first_pass_build_test_lint(d, out_dir)
    chart_cost_by_phase(d, out_dir)
    chart_total_cost_pie(d, out_dir)
    chart_cost_decomposition(d, out_dir)
    chart_time_decomposition(d, out_dir)
    chart_unit_economics(d, out_dir)

    summary = write_summary(d, summary_path)
    print("=== Tool-call summary ===")
    for k, v in summary.items():
        print(f"  {k:42s}  {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
