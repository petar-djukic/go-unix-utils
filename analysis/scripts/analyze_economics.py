#!/usr/bin/env python3
"""Economics charts for #5023.

Reads tasks.csv, runs.csv, runs.yaml, and utilities.csv. Produces 7 PNG/SVG
chart pairs under analysis/charts/economics/ plus an economics_summary.yaml.
Uses chronological run order on the x-axis where applicable so the platform
trend over time is visible.
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


def load(repo_root: Path) -> dict:
    d = repo_root / "analysis" / "datasets"
    runs = pd.read_csv(d / "runs.csv")
    runs["start_at"] = pd.to_datetime(runs["start_at"], utc=True)
    runs["end_at"] = pd.to_datetime(runs["end_at"], utc=True, errors="coerce")
    runs = runs.sort_values("start_at").reset_index(drop=True)
    runs["run_order"] = runs.index + 1

    tasks = pd.read_csv(d / "tasks.csv")
    tasks["commit_date"] = pd.to_datetime(tasks["commit_date"], utc=True)

    utilities = pd.read_csv(d / "utilities.csv")
    specs_path = d / "specs.csv"
    specs = pd.read_csv(specs_path) if specs_path.exists() else pd.DataFrame()

    runs_yaml: list[dict] = []
    rp = d / "runs.yaml"
    if rp.exists():
        with rp.open() as f:
            runs_yaml = yaml.safe_load(f) or []

    return {
        "runs": runs, "tasks": tasks, "utilities": utilities,
        "runs_yaml": runs_yaml, "specs": specs,
    }


def _scaffold_version_per_run(runs: pd.DataFrame, runs_yaml: list[dict]) -> pd.Series:
    """Match runs.csv rows to scaffold_version from runs.yaml report by
    matching `generation_branch` (which mirrors run_id for many runs)."""
    by_branch = {}
    for r in runs_yaml:
        if r.get("generation_branch") and r.get("scaffold_version"):
            by_branch[r["generation_branch"]] = r["scaffold_version"]
    return runs["run_id"].apply(
        lambda rid: by_branch.get(f"generation-{rid}") or by_branch.get(rid)
    )


def _requirements_completed_per_run(runs: pd.DataFrame, runs_yaml: list[dict]) -> pd.Series:
    by_branch: dict[str, int] = {}
    for r in runs_yaml:
        rc = r.get("requirements_completed")
        if rc is None:
            continue
        keys = [r.get("generation_branch"), r.get("run_id")]
        for k in keys:
            if k:
                by_branch[k] = rc
    return runs["run_id"].apply(
        lambda rid: by_branch.get(rid) or by_branch.get(f"generation-{rid}")
    )


def chart_cost_per_requirement_trend(d: dict, out_dir: Path) -> dict:
    runs = d["runs"].copy()
    runs["requirements_completed"] = _requirements_completed_per_run(runs, d["runs_yaml"])
    runs["scaffold_version"] = _scaffold_version_per_run(runs, d["runs_yaml"])
    runs = runs[runs["requirements_completed"].fillna(0) > 0].copy()
    runs["cost_per_req"] = runs["total_cost_usd"] / runs["requirements_completed"]

    fig, ax = plt.subplots(figsize=(13, 6))
    ax.plot(runs["start_at"], runs["cost_per_req"], marker="o", color="#4c72b0")
    seen_versions: set[str] = set()
    for _, row in runs.iterrows():
        v = row["scaffold_version"]
        if v and v not in seen_versions:
            ax.axvline(row["start_at"], color="#dd8452", alpha=0.4, linestyle="--", linewidth=1)
            ax.text(row["start_at"], runs["cost_per_req"].max() * 1.02,
                    str(v).split("-")[-1][:8], fontsize=7, color="#dd8452",
                    rotation=75, va="bottom")
            seen_versions.add(v)
    ax.set_xlabel("date")
    fig.autofmt_xdate()
    ax.set_ylabel("cost per requirement completed (USD)")
    titled(
        ax, "Cost per requirement over time",
        f"source: runs.csv joined to runs.yaml; N={len(runs)} runs with reported requirements_completed",
    )
    save(fig, out_dir, "cost_per_requirement_trend")
    return {
        "n_runs_with_requirements": int(len(runs)),
        "mean_cost_per_req_usd": round(float(runs["cost_per_req"].mean()), 4),
        "median_cost_per_req_usd": round(float(runs["cost_per_req"].median()), 4),
    }


def chart_cost_per_loc_trend(d: dict, out_dir: Path) -> dict:
    runs = d["runs"].copy()
    runs = runs[(runs["total_loc_prod_delta"] > 0) & (runs["total_cost_usd"] > 0)].copy()
    runs["cost_per_kloc"] = runs["total_cost_usd"] / (runs["total_loc_prod_delta"] / 1000)

    fig, ax = plt.subplots(figsize=(13, 6))
    ax.plot(runs["start_at"], runs["cost_per_kloc"], marker="o", color="#55a868")
    ax.set_xlabel("date")
    fig.autofmt_xdate()
    ax.set_ylabel("cost per KLOC produced (USD)")
    titled(
        ax, "Cost per production KLOC over time",
        f"source: runs.csv; N={len(runs)} runs with positive LOC delta and cost",
    )
    save(fig, out_dir, "cost_per_loc_trend")
    return {
        "n_runs_with_loc": int(len(runs)),
        "mean_cost_per_kloc_usd": round(float(runs["cost_per_kloc"].mean()), 4),
        "median_cost_per_kloc_usd": round(float(runs["cost_per_kloc"].median()), 4),
    }


def _largest_run_id(runs: pd.DataFrame) -> str:
    completed = runs[runs["task_count"] > 0]
    return str(completed.sort_values("task_count", ascending=False).iloc[0]["run_id"])


def chart_cache_hit_rate_within_run(d: dict, out_dir: Path) -> dict:
    tasks = d["tasks"].copy()
    runs = d["runs"]
    target_run = _largest_run_id(runs)
    target_window = runs[runs["run_id"] == target_run].iloc[0]
    sub = tasks[
        (tasks["commit_date"] >= target_window["start_at"])
        & ((target_window["end_at"] != target_window["end_at"]) |  # NaT check
           (tasks["commit_date"] <= target_window["end_at"]))
    ].copy()
    sub = sub.sort_values("commit_date").reset_index(drop=True)
    sub["task_index"] = sub.index + 1
    sub["non_cached_input"] = sub["tokens_input"] + sub["tokens_cache_creation"]
    sub["cache_hit_rate"] = sub["tokens_cache_read"] / (
        sub["tokens_cache_read"] + sub["non_cached_input"]
    ).replace(0, np.nan)

    fig, ax = plt.subplots(figsize=(13, 6))
    ax.scatter(sub["task_index"], sub["cache_hit_rate"] * 100, s=14, alpha=0.5, color="#4c72b0")
    rolling = sub["cache_hit_rate"].rolling(window=20, min_periods=5).median() * 100
    ax.plot(sub["task_index"], rolling, color="#c44e52", linewidth=2,
            label="20-task rolling median")
    ax.set_xlabel("task index within run (chronological)")
    ax.set_ylabel("cache hit rate (%)")
    ax.set_ylim(0, 105)
    ax.legend(loc="lower right")
    titled(
        ax, f"Cache hit rate within {target_run}",
        f"source: tasks.csv; N={len(sub)} tasks within {target_run} window; "
        f"hit rate = cache_read / (cache_read + input + cache_creation)",
    )
    save(fig, out_dir, "cache_hit_rate_within_run42")
    return {
        "target_run_id": target_run,
        "n_tasks_in_target_run": int(len(sub)),
        "median_within_run_hit_rate": round(float(sub["cache_hit_rate"].median() * 100), 2),
    }


def chart_cache_hit_rate_across_runs(d: dict, out_dir: Path) -> dict:
    tasks = d["tasks"].copy()
    runs = d["runs"]
    rows = []
    for _, run in runs.iterrows():
        end = run["end_at"] if pd.notna(run["end_at"]) else tasks["commit_date"].max()
        sub = tasks[(tasks["commit_date"] >= run["start_at"]) & (tasks["commit_date"] <= end)]
        if len(sub) == 0:
            continue
        non_cached = sub["tokens_input"] + sub["tokens_cache_creation"]
        denom = sub["tokens_cache_read"] + non_cached
        rate = sub["tokens_cache_read"] / denom.replace(0, np.nan)
        rows.append({
            "start_at": run["start_at"],
            "run_id": str(run["run_id"]),
            "n_tasks": int(len(sub)),
            "median_hit_rate": float(rate.median()) if rate.notna().any() else float("nan"),
        })
    df = pd.DataFrame(rows).dropna(subset=["median_hit_rate"])

    fig, ax = plt.subplots(figsize=(13, 6))
    ax.scatter(df["start_at"], df["median_hit_rate"] * 100,
               s=df["n_tasks"] * 1.2 + 12, alpha=0.6, color="#8172b3", edgecolor="black", linewidth=0.4)
    ax.set_xlabel("date")
    fig.autofmt_xdate()
    ax.set_ylabel("median cache hit rate per run (%)")
    ax.set_ylim(0, 105)
    titled(
        ax, "Median cache hit rate across runs",
        f"source: tasks.csv grouped by run window; N={len(df)} runs; bubble size proportional to task_count",
    )
    save(fig, out_dir, "cache_hit_rate_across_runs")
    return {
        "n_runs": int(len(df)),
        "median_across_runs": round(float(df["median_hit_rate"].median() * 100), 2),
    }


def chart_per_utility_avg_cost(d: dict, out_dir: Path) -> dict:
    util = d["utilities"].copy()
    util["avg_cost_usd"] = util["total_cost_usd"] / util["total_runs_touched"].replace(0, np.nan)
    util = util.dropna(subset=["avg_cost_usd"])
    util = util.sort_values("avg_cost_usd", ascending=False).head(30)
    fig, ax = plt.subplots(figsize=(13, 8))
    colors = ["#c44e52" if k == "cmd" else "#4c72b0" for k in util["target_kind"]]
    ax.barh(util["target"], util["avg_cost_usd"], color=colors)
    ax.invert_yaxis()
    ax.set_xlabel("average cost per completed utility (USD)")
    titled(
        ax, "Top 30 utilities by average completion cost",
        f"source: utilities.csv; cmd in red, pkg in blue; total tasks annotated",
    )
    for i, (_, row) in enumerate(util.iterrows()):
        ax.text(row["avg_cost_usd"] + 0.1, i,
                f" {int(row['total_tasks'])}t", fontsize=8, va="center", color="#555")
    save(fig, out_dir, "per_utility_avg_cost")
    return {
        "top_utility": str(util.iloc[0]["target"]),
        "top_utility_avg_cost": round(float(util.iloc[0]["avg_cost_usd"]), 4),
    }


def chart_same_utility_variance(d: dict, out_dir: Path) -> dict:
    util = d["utilities"].copy()
    util = util[util["total_runs_touched"] >= 3].copy()
    if util.empty:
        return {"n_utilities_3plus_runs": 0}
    util["mean_cost"] = util["total_cost_usd"] / util["total_attempts"].replace(0, np.nan)
    util["cv"] = util["cost_variance_per_run"].pow(0.5) / util["mean_cost"].replace(0, np.nan)
    util = util.sort_values("cv", ascending=False).head(30)

    fig, ax = plt.subplots(figsize=(13, 8))
    ax.barh(util["target"], util["cv"], color="#dd8452")
    ax.invert_yaxis()
    ax.set_xlabel("coefficient of variation: stdev(cost per attempt) / mean(cost per attempt)")
    titled(
        ax, "Same-utility cost variance across runs (higher = less consistent)",
        f"source: utilities.csv; N={len(util)} utilities with >=3 runs; top 30 by CV",
    )
    save(fig, out_dir, "same_utility_variance")
    return {
        "n_utilities_3plus_runs": int(len(util)),
        "max_cv": round(float(util["cv"].max()), 4),
        "least_consistent_utility": str(util.iloc[0]["target"]),
    }


def _section_max_index(specs: pd.DataFrame) -> dict[tuple[str, int], int]:
    """For each (srd_full_id, section_number), the max requirement index that
    exists in specs.csv. E.g., srd001 R3 has indexes 1-6 → ('srd001-testutils', 3) -> 6."""
    out: dict[tuple[str, int], int] = {}
    if specs.empty:
        return out
    import re
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


def _count_requirements(req_field: str, srd_id: str | None,
                        section_max: dict[tuple[str, int], int]) -> int:
    if not isinstance(req_field, str) or not req_field:
        return 0
    import re
    tokens = set()
    for chunk in req_field.split(","):
        chunk = chunk.strip()
        if not chunk:
            continue
        m = re.match(r"R(\d+)\.(\d+)(?:-R?(\d+)\.(\d+))?", chunk)
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
                    end = section_max.get((srd_id, sec)) if srd_id else None
                    if end is None:
                        end = 6  # fallback typical section size
                for i in range(start, end + 1):
                    tokens.add(f"R{sec}.{i}")
        else:
            tokens.add(f"R{s_sec}.{s_idx}")
    return len(tokens)


def chart_cost_vs_requirement_count(d: dict, out_dir: Path) -> dict:
    tasks = d["tasks"].copy()
    tasks = tasks[tasks["task_subtype"] == "stitch"].copy()
    section_max = _section_max_index(d.get("specs", pd.DataFrame()))
    # tasks.csv srd_id is short form (srd001); specs.csv is long form (srd001-testutils)
    short_to_long = {}
    if not d.get("specs", pd.DataFrame()).empty:
        for _, row in d["specs"].drop_duplicates("srd_id").iterrows():
            short_to_long[row["srd_id"]] = row["srd_full_id"]
    tasks["srd_full_id"] = tasks["srd_id"].map(short_to_long)
    tasks["req_count"] = tasks.apply(
        lambda r: _count_requirements(r["requirements"], r["srd_full_id"], section_max),
        axis=1,
    )
    sub = tasks[(tasks["req_count"] > 0) & (tasks["cost_usd"] > 0)].copy()

    fig, ax = plt.subplots(figsize=(11, 7))
    sc = ax.scatter(
        sub["req_count"], sub["cost_usd"],
        c=sub["duration_seconds"], cmap="viridis",
        s=18, alpha=0.55, edgecolor="white", linewidth=0.2,
    )
    cb = plt.colorbar(sc, ax=ax)
    cb.set_label("duration (s)")
    if len(sub) >= 5:
        coef = np.polyfit(sub["req_count"], sub["cost_usd"], 1)
        x_fit = np.linspace(sub["req_count"].min(), sub["req_count"].max(), 50)
        ax.plot(x_fit, np.polyval(coef, x_fit), color="#c44e52", linewidth=1.5,
                linestyle="--", label=f"fit: ${coef[0]:.4f}/req + ${coef[1]:.3f}")
        ax.legend(loc="upper left")
    ax.set_xlabel("requirements covered by task")
    ax.set_ylabel("cost (USD)")
    titled(
        ax, "Cost vs requirement count per stitch task",
        f"source: tasks.csv (stitch only); N={len(sub)} tasks with positive cost and parsed reqs",
    )
    save(fig, out_dir, "cost_vs_requirement_count")
    coef = np.polyfit(sub["req_count"], sub["cost_usd"], 1) if len(sub) >= 5 else (None, None)
    return {
        "n_tasks_with_reqs": int(len(sub)),
        "fit_slope_usd_per_req": round(float(coef[0]), 4) if coef[0] is not None else None,
        "fit_intercept_usd": round(float(coef[1]), 4) if coef[1] is not None else None,
    }


def chart_requirement_cost_pdf(d: dict, out_dir: Path) -> dict:
    tasks = d["tasks"].copy()
    stitch = tasks[tasks["task_subtype"] == "stitch"].copy()

    def count_reqs(r):
        if pd.isna(r):
            return 0
        return len([x.strip() for x in str(r).split(",") if x.strip()])

    stitch["req_count"] = stitch["requirements"].apply(count_reqs)
    stitch = stitch[stitch["req_count"] > 0].copy()
    stitch["cost_per_req"] = stitch["cost_usd"] / stitch["req_count"]

    by_srd = stitch.groupby("srd_id").agg(
        target=("target", "first"),
        total_cost=("cost_usd", "sum"),
        total_reqs=("req_count", "sum"),
    ).reset_index()
    by_srd["cost_per_req"] = by_srd["total_cost"] / by_srd["total_reqs"]

    fig, ax = plt.subplots(figsize=(13, 6))
    ax.hist(by_srd["cost_per_req"], bins=30, density=True, color="#4c72b0",
            edgecolor="white", alpha=0.7, label="histogram")
    from scipy.stats import gaussian_kde
    x = np.linspace(by_srd["cost_per_req"].min() * 0.8, by_srd["cost_per_req"].max() * 1.1, 200)
    kde = gaussian_kde(by_srd["cost_per_req"])
    ax.plot(x, kde(x), color="#c44e52", linewidth=2.5, label="KDE")
    med = by_srd["cost_per_req"].median()
    mean = by_srd["cost_per_req"].mean()
    ax.axvline(med, color="#55a868", linestyle="--", linewidth=2,
               label=f"median ${med:.3f}")
    ax.axvline(mean, color="#dd8452", linestyle="--", linewidth=2,
               label=f"mean ${mean:.3f}")
    ax.set_xlabel("cost per requirement (USD)")
    ax.set_ylabel("density")
    ax.legend(loc="upper right", fontsize=10)
    titled(
        ax, "Distribution of cost per requirement across SRDs",
        f"source: tasks.csv grouped by srd_id; N={len(by_srd)} SRDs, {len(stitch)} tasks",
    )
    save(fig, out_dir, "requirement_cost_pdf")
    return {
        "n_srds": int(len(by_srd)),
        "mean_cost_per_req": round(float(mean), 4),
        "median_cost_per_req": round(float(med), 4),
    }


TOTAL_REQUIREMENTS = 1475


def _run_label(run_id: str) -> str:
    import re
    m = re.search(r"run(\d+\w*)", run_id)
    return m.group(0) if m else run_id.split("-")[-1]


def chart_generation_progress(d: dict, out_dir: Path) -> dict:
    """LOC produced and requirements covered per major generation run."""
    runs = d["runs"].copy()
    tasks = d["tasks"].copy()
    stitch = tasks[tasks["task_subtype"] == "stitch"].copy()
    stitch["commit_date"] = pd.to_datetime(stitch["commit_date"], utc=True)

    def count_reqs(r):
        if pd.isna(r):
            return 0
        return len([x.strip() for x in str(r).split(",") if x.strip()])

    stitch["req_count"] = stitch["requirements"].apply(count_reqs)

    active = runs[runs["task_count"] >= 30].sort_values("start_at").copy()
    active["end_at"] = pd.to_datetime(active["end_at"], utc=True, errors="coerce")

    rows = []
    for _, run in active.iterrows():
        start = run["start_at"]
        end = run["end_at"] if pd.notna(run["end_at"]) else start + pd.Timedelta(days=7)
        sub = stitch[(stitch["commit_date"] >= start) & (stitch["commit_date"] <= end)]
        reqs = int(sub["req_count"].sum())
        rows.append({
            "run_id": run["run_id"],
            "label": _run_label(run["run_id"]),
            "date": run["start_at"],
            "tasks": int(run["task_count"]),
            "loc_prod": int(max(run["total_loc_prod_delta"], 0)),
            "loc_test": int(max(run.get("total_loc_test_delta", 0), 0)),
            "reqs": reqs,
            "req_pct": reqs / TOTAL_REQUIREMENTS * 100,
            "cost": float(run["total_cost_usd"]),
        })
    df = pd.DataFrame(rows)

    fig, ax1 = plt.subplots(figsize=(14, 7))
    x = np.arange(len(df))
    w = 0.35

    ax1.bar(x - w / 2, df["loc_prod"] / 1000, w, color="#4c72b0", label="production KLOC")
    ax1.bar(x + w / 2, df["loc_test"] / 1000, w, color="#55a868", label="test KLOC")
    ax1.set_ylabel("lines of code (KLOC)")
    ax1.set_xticks(x)
    ax1.set_xticklabels(df["label"], rotation=40, ha="right")

    ax2 = ax1.twinx()
    ax2.plot(x, df["req_pct"], marker="D", color="#c44e52", linewidth=2.5,
             markersize=7, label="% requirements", zorder=5)
    ax2.set_ylabel("requirements covered (%)")
    ax2.set_ylim(0, 110)

    for i, row in df.iterrows():
        if row["req_pct"] > 0:
            ax2.annotate(
                f"{row['req_pct']:.0f}%",
                (i, row["req_pct"]), textcoords="offset points",
                xytext=(0, 10), ha="center", fontsize=9, color="#c44e52",
            )
        total_kloc = (row["loc_prod"] + row["loc_test"]) / 1000
        if total_kloc > 0:
            ax1.text(i, total_kloc + 1, f"${row['cost']:.0f}",
                     ha="center", fontsize=8, color="#555")

    lines1, labels1 = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines1 + lines2, labels1 + labels2, loc="upper left", fontsize=10)

    titled(
        ax1, "Generation progress across runs",
        f"major runs (≥30 tasks) | {TOTAL_REQUIREMENTS} total requirements in spec | "
        f"cost annotated above bars",
    )
    save(fig, out_dir, "generation_progress")
    return {
        "n_major_runs": int(len(df)),
        "latest_req_pct": round(float(df.iloc[-1]["req_pct"]), 1),
        "latest_loc_prod": int(df.iloc[-1]["loc_prod"]),
        "peak_loc_prod": int(df["loc_prod"].max()),
    }


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    out_dir = repo_root / "analysis" / "charts" / "economics"
    summary_path = repo_root / "analysis" / "datasets" / "economics_summary.yaml"
    setup_style()
    d = load(repo_root)

    summary = {
        "n_runs_total": int(len(d["runs"])),
        "n_tasks_total": int(len(d["tasks"])),
        "n_utilities_total": int(len(d["utilities"])),
        "cost_per_requirement": chart_cost_per_requirement_trend(d, out_dir),
        "cost_per_loc": chart_cost_per_loc_trend(d, out_dir),
        "cache_hit_within_run": chart_cache_hit_rate_within_run(d, out_dir),
        "cache_hit_across_runs": chart_cache_hit_rate_across_runs(d, out_dir),
        "per_utility_avg": chart_per_utility_avg_cost(d, out_dir),
        "same_utility_variance": chart_same_utility_variance(d, out_dir),
        "cost_vs_req_count": chart_cost_vs_requirement_count(d, out_dir),
        "requirement_cost_pdf": chart_requirement_cost_pdf(d, out_dir),
        "generation_progress": chart_generation_progress(d, out_dir),
    }
    with summary_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    print("=== Economics summary ===")
    for k, v in summary.items():
        print(f"  {k}: {v}")
    print(f"\nCharts written to {out_dir.relative_to(repo_root)}")
    print(f"Summary written to {summary_path.relative_to(repo_root)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
