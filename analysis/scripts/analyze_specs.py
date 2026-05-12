#!/usr/bin/env python3
"""Spec-aware difficulty charts for #5027.

Joins per-task cost (tasks.csv) with per-requirement metadata (specs.csv)
to ask: are weights predictive of cost? do longer or code-block-bearing
requirements cost more? which SRDs are hardest? which releases?

The `requirements` column in tasks.csv is a comma-separated mix of single
ids and ranges (e.g., `R1.1-R1.4, R3.1`). Expansion to individual req_ids
uses per-SRD section_max from specs.csv so ranges like `R1.1-R5.4` resolve
to the real count rather than a 99/section fallback.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns
import yaml

_RANGE_RE = re.compile(r"R(\d+)\.(\d+)(?:-R?(\d+)\.(\d+))?")


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


def _section_max_index(specs: pd.DataFrame) -> dict[tuple[str, int], int]:
    out: dict[tuple[str, int], int] = {}
    for _, row in specs.iterrows():
        m = re.match(r"R(\d+)\.(\d+)", str(row["req_id"]))
        if not m:
            continue
        sec = int(m.group(1))
        idx = int(m.group(2))
        key = (row["srd_full_id"], sec)
        if idx > out.get(key, 0):
            out[key] = idx
    return out


def _expand_requirements(req_field: str, srd_full_id: str | None,
                         section_max: dict[tuple[str, int], int]) -> list[str]:
    if not isinstance(req_field, str) or not req_field:
        return []
    tokens: set[str] = set()
    for chunk in req_field.split(","):
        chunk = chunk.strip()
        m = _RANGE_RE.match(chunk)
        if not m:
            continue
        s_sec, s_idx = int(m.group(1)), int(m.group(2))
        if m.group(3):
            e_sec, e_idx = int(m.group(3)), int(m.group(4))
            for sec in range(s_sec, e_sec + 1):
                start = s_idx if sec == s_sec else 1
                if sec == e_sec:
                    end = e_idx
                else:
                    end = section_max.get((srd_full_id, sec), 6) if srd_full_id else 6
                for i in range(start, end + 1):
                    tokens.add(f"R{sec}.{i}")
        else:
            tokens.add(f"R{s_sec}.{s_idx}")
    return sorted(tokens)


def load(repo_root: Path) -> dict:
    d = repo_root / "analysis" / "datasets"
    tasks = pd.read_csv(d / "tasks.csv")
    specs = pd.read_csv(d / "specs.csv")
    return {"tasks": tasks, "specs": specs}


def build_task_req_join(tasks: pd.DataFrame, specs: pd.DataFrame) -> pd.DataFrame:
    """One row per (task, req_id) for tasks whose requirements can be expanded
    and joined to specs.csv. Cost and duration are split equally across the
    task's requirements."""
    section_max = _section_max_index(specs)
    short_to_long = (
        specs.drop_duplicates("srd_id").set_index("srd_id")["srd_full_id"].to_dict()
    )
    tasks = tasks[tasks["task_subtype"] == "stitch"].copy()
    tasks["srd_full_id"] = tasks["srd_id"].map(short_to_long)
    tasks["req_ids"] = tasks.apply(
        lambda r: _expand_requirements(r["requirements"], r["srd_full_id"], section_max),
        axis=1,
    )
    tasks["req_count"] = tasks["req_ids"].apply(len)

    rows: list[dict] = []
    for _, t in tasks.iterrows():
        n = max(int(t["req_count"]), 1) if t["req_count"] else 0
        if n == 0:
            continue
        share = float(t["cost_usd"]) / n
        dur_share = float(t["duration_seconds"]) / n
        for rid in t["req_ids"]:
            rows.append({
                "task_id": t["task_id"],
                "commit_sha": t["commit_sha"],
                "target": t["target"],
                "srd_id": t["srd_id"],
                "srd_full_id": t["srd_full_id"],
                "req_id": rid,
                "task_cost_usd": t["cost_usd"],
                "task_duration_s": t["duration_seconds"],
                "share_cost_usd": share,
                "share_duration_s": dur_share,
                "task_req_count": t["req_count"],
            })
    expanded = pd.DataFrame(rows)
    if expanded.empty:
        return expanded
    joined = expanded.merge(
        specs[["srd_full_id", "req_id", "req_section", "text_length",
               "has_code_block", "weight", "state", "release_id",
               "acceptance_criteria_count"]],
        on=["srd_full_id", "req_id"], how="left",
    )
    return joined


def _per_task_features(joined: pd.DataFrame, tasks: pd.DataFrame) -> pd.DataFrame:
    """Aggregate per-req metadata back to per-task: total text length, total
    weight, any-code-block, etc. Used by the scatter plots."""
    agg = (
        joined.groupby("commit_sha")
        .agg(
            total_text_length=("text_length", "sum"),
            total_weight=("weight", "sum"),
            mean_weight=("weight", "mean"),
            any_code_block=("has_code_block", "any"),
            req_count=("req_id", "count"),
        )
        .reset_index()
    )
    t = tasks[["commit_sha", "cost_usd", "duration_seconds", "target",
               "srd_id"]].copy()
    return agg.merge(t, on="commit_sha", how="inner")


def _ols_fit(x: pd.Series, y: pd.Series) -> tuple[float, float, float, int]:
    """Return slope, intercept, r-squared, n."""
    mask = x.notna() & y.notna() & (x != 0) & (y != 0)
    x = x[mask].astype(float)
    y = y[mask].astype(float)
    n = len(x)
    if n < 5:
        return float("nan"), float("nan"), float("nan"), n
    slope, intercept = np.polyfit(x, y, 1)
    y_pred = slope * x + intercept
    ss_res = float(np.sum((y - y_pred) ** 2))
    ss_tot = float(np.sum((y - y.mean()) ** 2))
    r2 = 1 - ss_res / ss_tot if ss_tot > 0 else float("nan")
    return float(slope), float(intercept), float(r2), n


def _scatter_fit(ax: plt.Axes, x: pd.Series, y: pd.Series,
                 color: str, point_alpha: float = 0.35) -> dict:
    ax.scatter(x, y, s=12, alpha=point_alpha, color=color, edgecolor="white", linewidth=0.2)
    slope, intercept, r2, n = _ols_fit(x, y)
    if not np.isnan(slope):
        xs = np.linspace(float(x.min()), float(x.max()), 50)
        ax.plot(xs, slope * xs + intercept, color="#c44e52", linewidth=1.5,
                linestyle="--", label=f"fit slope={slope:.4g}, r²={r2:.3f}, n={n}")
        ax.legend(loc="upper left", fontsize=9)
    return {"slope": slope, "intercept": intercept, "r2": r2, "n": n}


def chart_req_count_vs_cost(features: pd.DataFrame, out_dir: Path) -> dict:
    fig, ax = plt.subplots(figsize=(10, 7))
    stats = _scatter_fit(ax, features["req_count"], features["cost_usd"], "#4c72b0")
    ax.set_xlabel("requirements implemented in this task")
    ax.set_ylabel("task cost (USD)")
    titled(
        ax, "Task cost vs requirement count",
        f"source: tasks.csv expanded via specs.csv; N={len(features)} stitch tasks with matched reqs",
    )
    save(fig, out_dir, "req_count_vs_cost")
    return stats


def chart_text_length_vs_cost(features: pd.DataFrame, out_dir: Path) -> dict:
    fig, ax = plt.subplots(figsize=(10, 7))
    stats = _scatter_fit(ax, features["total_text_length"], features["cost_usd"], "#55a868")
    ax.set_xlabel("sum of requirement text length (chars)")
    ax.set_ylabel("task cost (USD)")
    titled(
        ax, "Task cost vs requirement text length",
        f"source: tasks.csv joined to specs.csv text_length; N={len(features)}",
    )
    save(fig, out_dir, "text_length_vs_cost")
    return stats


def chart_weight_vs_cost(features: pd.DataFrame, out_dir: Path) -> dict:
    sub = features[features["total_weight"].notna() & (features["total_weight"] > 0)]
    fig, ax = plt.subplots(figsize=(10, 7))
    stats = _scatter_fit(ax, sub["total_weight"], sub["cost_usd"], "#dd8452")
    ax.set_xlabel("sum of requirement weights")
    ax.set_ylabel("task cost (USD)")
    titled(
        ax, "Task cost vs explicit weight",
        f"source: specs.csv weight column (only ~297 reqs carry weight); N={len(sub)} tasks",
    )
    save(fig, out_dir, "weight_vs_cost")
    return stats


def chart_weight_vs_duration(features: pd.DataFrame, out_dir: Path) -> dict:
    sub = features[features["total_weight"].notna() & (features["total_weight"] > 0)]
    fig, ax = plt.subplots(figsize=(10, 7))
    stats = _scatter_fit(ax, sub["total_weight"], sub["duration_seconds"], "#8172b3")
    ax.set_xlabel("sum of requirement weights")
    ax.set_ylabel("task duration (s)")
    titled(
        ax, "Task duration vs explicit weight",
        f"source: specs.csv weight column; N={len(sub)} tasks",
    )
    save(fig, out_dir, "weight_vs_duration")
    return stats


def chart_per_srd_difficulty(features: pd.DataFrame, out_dir: Path) -> dict:
    counts = features.groupby("srd_id").size()
    keep = counts[counts >= 3].index
    sub = features[features["srd_id"].isin(keep)].copy()
    means = (
        sub.groupby("srd_id")["cost_usd"].mean()
        .sort_values(ascending=False).head(30)
    )

    fig, ax = plt.subplots(figsize=(13, 9))
    ax.barh(means.index, means.values, color="#c44e52")
    ax.invert_yaxis()
    ax.set_xlabel("mean cost per task (USD)")
    titled(
        ax, "Top 30 SRDs by mean per-task cost",
        f"source: tasks.csv; SRDs with >=3 stitch tasks; N={len(means)} SRDs shown",
    )
    save(fig, out_dir, "per_srd_difficulty")
    return {
        "hardest_srds": [
            {"srd_id": str(srd), "mean_cost_usd": round(float(c), 4)}
            for srd, c in means.head(10).items()
        ],
    }


def chart_code_block_effect(joined: pd.DataFrame, out_dir: Path) -> dict:
    sub = joined[joined["share_cost_usd"].notna() & joined["has_code_block"].notna()].copy()
    if sub.empty:
        return {}
    sub["has_code_block"] = sub["has_code_block"].astype(bool)
    with_block = sub.loc[sub["has_code_block"], "share_cost_usd"]
    without_block = sub.loc[~sub["has_code_block"], "share_cost_usd"]
    ratio = float(with_block.mean() / without_block.mean()) if without_block.mean() else float("nan")

    fig, ax = plt.subplots(figsize=(8, 6))
    sns.boxplot(data=sub, x="has_code_block", y="share_cost_usd", ax=ax,
                order=[False, True], palette=["#4c72b0", "#dd8452"], fliersize=2)
    ax.set_xlabel("requirement text contains code block")
    ax.set_ylabel("per-requirement share of task cost (USD)")
    ax.set_ylim(-0.05, sub["share_cost_usd"].quantile(0.99) * 1.05)
    titled(
        ax, "Cost per requirement: with vs without code-block examples",
        f"source: specs.csv has_code_block; ratio with/without = {ratio:.3f}; "
        f"N={int(with_block.size)} with, N={int(without_block.size)} without",
    )
    save(fig, out_dir, "code_block_effect")
    return {
        "mean_cost_with_code_block": round(float(with_block.mean()), 4),
        "mean_cost_without_code_block": round(float(without_block.mean()), 4),
        "ratio_with_over_without": round(ratio, 4) if not np.isnan(ratio) else None,
        "n_with": int(with_block.size),
        "n_without": int(without_block.size),
    }


def chart_per_release_difficulty(features: pd.DataFrame, joined: pd.DataFrame,
                                 out_dir: Path) -> dict:
    rel_per_task = (
        joined.dropna(subset=["release_id"])
        .groupby("commit_sha")["release_id"].agg(lambda s: s.mode().iloc[0])
        .reset_index()
    )
    sub = features.merge(rel_per_task, on="commit_sha", how="inner")
    sub = sub[sub["cost_usd"] > 0]
    order = sorted(sub["release_id"].dropna().unique())
    if not order:
        return {}

    fig, ax = plt.subplots(figsize=(16, 7))
    sns.boxplot(data=sub, x="release_id", y="cost_usd", ax=ax,
                order=order, color="#55a868", fliersize=2)
    ax.set_xlabel("")
    ax.set_ylabel("task cost (USD)")
    ax.set_ylim(0, sub["cost_usd"].quantile(0.99) * 1.05)
    plt.setp(ax.get_xticklabels(), rotation=60, ha="right", fontsize=8)
    titled(
        ax, "Per-task cost by release",
        f"source: tasks.csv joined to specs.csv release_id (modal release per task); N={len(sub)} tasks",
    )
    save(fig, out_dir, "per_release_difficulty")
    medians = sub.groupby("release_id")["cost_usd"].median()
    return {
        "n_tasks_with_release": int(len(sub)),
        "median_cost_by_release": {str(k): round(float(v), 4) for k, v in medians.items()},
    }


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    out_dir = repo_root / "analysis" / "charts" / "specs"
    summary_path = repo_root / "analysis" / "datasets" / "specs_summary.yaml"
    setup_style()
    d = load(repo_root)

    joined = build_task_req_join(d["tasks"], d["specs"])
    features = _per_task_features(joined, d["tasks"])

    summary = {
        "n_stitch_tasks_with_reqs": int(len(features)),
        "n_per_req_rows": int(len(joined)),
        "req_count_to_cost": chart_req_count_vs_cost(features, out_dir),
        "text_length_to_cost": chart_text_length_vs_cost(features, out_dir),
        "weight_to_cost": chart_weight_vs_cost(features, out_dir),
        "weight_to_duration": chart_weight_vs_duration(features, out_dir),
        "per_srd_difficulty": chart_per_srd_difficulty(features, out_dir),
        "code_block_effect": chart_code_block_effect(joined, out_dir),
        "per_release_difficulty": chart_per_release_difficulty(features, joined, out_dir),
    }
    with summary_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    print("=== Spec-difficulty summary ===")
    for k, v in summary.items():
        print(f"  {k}: {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
