#!/usr/bin/env python3
"""Train a tool-count proxy model and back-estimate trailer-only tasks (#5029).

Only ~143 of ~2,200 task commits have transcript-level tool counts. The
remaining ~2,070 carry only commit trailers (tokens, cost, duration, LOC
delta). We train a regression on the labeled subset that maps trailer
features to tool counts, then apply it to all tasks so cross-run tool-call
charts can span the full history.

Outputs:
  analysis/datasets/tasks_with_estimated_tools.csv
  analysis/datasets/proxy_model_metrics.yaml
  analysis/charts/proxy/*.{png,svg}
"""
from __future__ import annotations

import sys
import warnings
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns
import yaml
from sklearn.ensemble import GradientBoostingRegressor, RandomForestRegressor
from sklearn.linear_model import LinearRegression
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score
from sklearn.model_selection import GroupKFold

warnings.filterwarnings("ignore", category=UserWarning)

FEATURES_NUM = [
    "tokens_input",
    "tokens_output",
    "tokens_cache_creation",
    "tokens_cache_read",
    "duration_seconds",
    "loc_prod_delta",
    "loc_test_delta",
    "insertions",
    "deletions",
]
FEATURES_BOOL = ["is_contract"]
FEATURES_CAT = ["target_kind"]

TARGETS = [
    "total_tool_calls",
    "bash_calls",
    "read_calls",
    "edit_calls",
    "write_calls",
    "go_build_calls",
    "go_test_calls",
    "go_vet_calls",
    "go_build_failed_calls",
]


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


def load(repo_root: Path) -> dict[str, pd.DataFrame]:
    ds = repo_root / "analysis" / "datasets"
    tasks = pd.read_csv(ds / "tasks.csv")
    tools = pd.read_csv(ds / "tools.csv")
    runs = pd.read_csv(ds / "runs.csv")
    return {"tasks": tasks, "tools": tools, "runs": runs}


def _aggregate_tools_per_task(tools: pd.DataFrame) -> pd.DataFrame:
    sub = tools[tools["task_id_kind"] == "numeric"].copy()
    sub["task_id"] = sub["task_id"].astype(int)

    by_task = sub.groupby("task_id")
    agg = pd.DataFrame({"total_tool_calls": by_task.size()})

    for tname, col in [
        ("Bash", "bash_calls"),
        ("Read", "read_calls"),
        ("Edit", "edit_calls"),
        ("Write", "write_calls"),
    ]:
        agg[col] = by_task.apply(lambda g, t=tname: int((g["tool_name"] == t).sum()))

    for klass, col in [
        ("go_build", "go_build_calls"),
        ("go_test", "go_test_calls"),
        ("go_vet", "go_vet_calls"),
    ]:
        agg[col] = by_task.apply(lambda g, k=klass: int((g["bash_command_class"] == k).sum()))

    agg["go_build_failed_calls"] = by_task.apply(
        lambda g: int(((g["bash_command_class"] == "go_build") & (g["tool_result_is_error"])).sum())
    )

    return agg.reset_index()


def _encode(df: pd.DataFrame) -> tuple[pd.DataFrame, list[str]]:
    out = df[FEATURES_NUM + FEATURES_BOOL].copy()
    for c in FEATURES_NUM + FEATURES_BOOL:
        out[c] = pd.to_numeric(out[c], errors="coerce").fillna(0)
    dummies = pd.get_dummies(df["target_kind"].fillna("other"), prefix="kind").astype(float)
    out = pd.concat([out, dummies], axis=1)
    return out, list(out.columns)


def _fit_eval(X_train, y_train, X_test, y_test, model):
    model.fit(X_train, y_train)
    pred = model.predict(X_test)
    return {
        "mae": float(mean_absolute_error(y_test, pred)),
        "rmse": float(np.sqrt(mean_squared_error(y_test, pred))),
        "r2": float(r2_score(y_test, pred)),
        "pred": pred,
        "model": model,
    }


def _train_for_target(features_df: pd.DataFrame, X: pd.DataFrame, y: pd.Series, groups: pd.Series):
    n = len(X)
    n_groups = groups.nunique()
    n_splits = min(5, n_groups) if n_groups >= 2 else 2

    if n_groups >= 2:
        gkf = GroupKFold(n_splits=n_splits)
        splits = list(gkf.split(X, y, groups))
    else:
        idx = np.arange(n)
        rng = np.random.default_rng(0)
        rng.shuffle(idx)
        cut = int(n * 0.8)
        splits = [(idx[:cut], idx[cut:])]

    candidates = {
        "linear": LinearRegression(),
        "random_forest": RandomForestRegressor(n_estimators=200, max_depth=8, random_state=0),
        "gradient_boosting": GradientBoostingRegressor(n_estimators=200, max_depth=3, random_state=0),
    }

    results = {}
    oof_by_name: dict[str, np.ndarray] = {}
    for name, base_model in candidates.items():
        from sklearn.base import clone
        oof = np.full(len(X), np.nan)
        for tr, te in splits:
            m = clone(base_model)
            m.fit(X.iloc[tr], y.iloc[tr])
            oof[te] = m.predict(X.iloc[te])
        oof_by_name[name] = oof
        mask = ~np.isnan(oof)
        results[name] = {
            "mae": float(mean_absolute_error(y.values[mask], oof[mask])),
            "rmse": float(np.sqrt(mean_squared_error(y.values[mask], oof[mask]))),
            "r2": float(r2_score(y.values[mask], oof[mask])),
        }

    best = max(results.items(), key=lambda kv: kv[1]["r2"])
    best_name, best_metrics = best
    final_model = candidates[best_name]
    final_model.fit(X, y)
    return best_name, best_metrics, results, final_model, oof_by_name[best_name]


def _bootstrap_intervals(model_cls_args, X_train, y_train, X_apply, n_boot: int = 30, seed: int = 0):
    rng = np.random.default_rng(seed)
    cls, kwargs = model_cls_args
    preds = np.zeros((n_boot, len(X_apply)))
    n = len(X_train)
    for i in range(n_boot):
        idx = rng.integers(0, n, size=n)
        m = cls(**kwargs)
        m.fit(X_train.iloc[idx], y_train.iloc[idx])
        preds[i] = m.predict(X_apply)
    lo = np.quantile(preds, 0.05, axis=0)
    hi = np.quantile(preds, 0.95, axis=0)
    return lo, hi


_MODEL_FACTORY = {
    "linear": (LinearRegression, {}),
    "random_forest": (RandomForestRegressor, {"n_estimators": 200, "max_depth": 8, "random_state": 0}),
    "gradient_boosting": (GradientBoostingRegressor, {"n_estimators": 200, "max_depth": 3, "random_state": 0}),
}


def chart_predicted_vs_actual(
    y_actual: pd.Series,
    y_pred: np.ndarray,
    r2: float,
    mae: float,
    out_dir: Path,
) -> None:
    fig, ax = plt.subplots(figsize=(8, 7))
    ax.scatter(y_actual, y_pred, alpha=0.6, s=50, color="#1f77b4", edgecolor="white")
    lim = max(float(y_actual.max()), float(y_pred.max())) * 1.05
    ax.plot([0, lim], [0, lim], "--", color="#888", linewidth=1.2)
    ax.set_xlabel("actual tool calls")
    ax.set_ylabel("predicted tool calls")
    ax.set_xlim(0, lim)
    ax.set_ylim(0, lim)
    titled(
        ax,
        "Predicted vs actual total tool calls",
        f"cross-validated by run; test R²={r2:.2f}, MAE={mae:.1f}; n={len(y_actual)} labeled tasks",
    )
    save(fig, out_dir, "predicted_vs_actual_scatter")


def chart_feature_importance(model, feature_names: list[str], out_dir: Path) -> None:
    if not hasattr(model, "feature_importances_"):
        return
    imp = pd.Series(model.feature_importances_, index=feature_names).sort_values()
    fig, ax = plt.subplots(figsize=(9, 0.4 * len(imp) + 2))
    bars = ax.barh(imp.index, imp.values, color="#2ca02c", edgecolor="white")
    for bar, val in zip(bars, imp.values):
        ax.text(val + 0.005, bar.get_y() + bar.get_height() / 2, f"{val:.2f}",
                va="center", fontsize=9)
    ax.set_xlabel("feature importance")
    titled(
        ax,
        "Feature importance for total_tool_calls model",
        "source: random-forest on labeled task-trailer features",
    )
    save(fig, out_dir, "feature_importance")


def _run_order(tasks: pd.DataFrame) -> pd.Series:
    """Map each task to a chronological run bucket by commit_date (day)."""
    return pd.to_datetime(tasks["commit_date"], utc=True, errors="coerce").dt.tz_convert("UTC").dt.date


def chart_trend(
    tasks_pred: pd.DataFrame,
    target: str,
    out_dir: Path,
    file_name: str,
    title: str,
    labeled_dates: set,
) -> dict:
    sub = tasks_pred.copy()
    sub["run_day"] = _run_order(sub)
    sub = sub.dropna(subset=["run_day"])
    grp = sub.groupby("run_day")
    mean_val = grp[target].mean()
    lo_val = grp[f"{target}_lo"].mean()
    hi_val = grp[f"{target}_hi"].mean()
    counts = grp.size()
    days = mean_val.index.tolist()

    fig, ax = plt.subplots(figsize=(13, 6))
    x = np.arange(len(days))
    ax.fill_between(x, lo_val.values, hi_val.values, alpha=0.2, color="#1f77b4", label="90% interval")
    ax.plot(x, mean_val.values, marker="o", color="#1f77b4", linewidth=1.6, label="mean predicted")
    for i, d in enumerate(days):
        if d in labeled_dates:
            ax.axvline(i, color="#ff7f0e", alpha=0.18, linewidth=2)
    ax.set_xticks(x)
    ax.set_xticklabels([str(d) for d in days], rotation=60, ha="right", fontsize=7)
    ax.set_xlabel("run day (chronological)")
    ax.set_ylabel(f"mean {target} per task")
    ax.legend(loc="upper left", fontsize=9, framealpha=0.9)
    titled(
        ax, title,
        f"orange bars mark days with transcript-labeled tasks; {len(days)} day buckets across {int(counts.sum())} tasks",
    )
    save(fig, out_dir, file_name)
    return {
        "n_day_buckets": int(len(days)),
        "n_tasks": int(counts.sum()),
        "global_mean": round(float(mean_val.mean()), 2),
    }


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    out_dir = repo_root / "analysis" / "charts" / "proxy"
    out_csv = repo_root / "analysis" / "datasets" / "tasks_with_estimated_tools.csv"
    summary_path = repo_root / "analysis" / "datasets" / "proxy_model_metrics.yaml"
    setup_style()

    d = load(repo_root)
    tasks = d["tasks"].copy()
    tasks = tasks[tasks["task_subtype"] == "stitch"].copy()
    tasks = tasks.drop_duplicates(subset=["task_id"], keep="last")

    tool_agg = _aggregate_tools_per_task(d["tools"])
    labeled = tasks.merge(tool_agg, on="task_id", how="inner")
    print(f"labeled tasks: {len(labeled)} of {len(tasks)} total")

    X_lab, feat_names = _encode(labeled)
    groups = _run_order(labeled).astype(str)

    summary: dict = {
        "training_set_size": int(len(labeled)),
        "total_tasks": int(len(tasks)),
        "feature_names": feat_names,
    }

    target_results: dict[str, dict] = {}
    chosen_models: dict[str, tuple] = {}
    for target in TARGETS:
        if target not in labeled.columns:
            continue
        y = labeled[target].astype(float)
        if y.sum() == 0:
            continue
        best_name, best_metrics, all_metrics, fitted, oof_pred = _train_for_target(labeled, X_lab, y, groups)
        target_results[target] = {
            "best_model": best_name,
            "test_r2": round(best_metrics["r2"], 4),
            "test_mae": round(best_metrics["mae"], 4),
            "test_rmse": round(best_metrics["rmse"], 4),
            "candidates": {k: {kk: round(vv, 4) for kk, vv in v.items()} for k, v in all_metrics.items()},
        }
        chosen_models[target] = (best_name, fitted, y)

        if target == "total_tool_calls":
            mask = ~np.isnan(oof_pred)
            chart_predicted_vs_actual(y[mask], oof_pred[mask], best_metrics["r2"], best_metrics["mae"], out_dir)
            if hasattr(fitted, "feature_importances_"):
                chart_feature_importance(fitted, feat_names, out_dir)
            elif "random_forest" in all_metrics:
                rf = RandomForestRegressor(n_estimators=200, max_depth=8, random_state=0)
                rf.fit(X_lab, y)
                chart_feature_importance(rf, feat_names, out_dir)

    summary["models"] = target_results

    X_all, _ = _encode(tasks)
    tasks_out = tasks.copy()
    labeled_ids = set(labeled["task_id"])
    labeled_agg_by_id = tool_agg.set_index("task_id")

    for target, (best_name, fitted, y_train) in chosen_models.items():
        preds = fitted.predict(X_all)
        cls, kwargs = _MODEL_FACTORY[best_name]
        lo, hi = _bootstrap_intervals((cls, kwargs), X_lab, y_train, X_all)
        preds = np.clip(preds, 0, None)
        lo = np.clip(lo, 0, None)
        hi = np.clip(hi, 0, None)
        tasks_out[f"est_{target}"] = preds
        tasks_out[f"est_{target}_lo"] = lo
        tasks_out[f"est_{target}_hi"] = hi

    is_predicted = ~tasks_out["task_id"].isin(labeled_ids)
    tasks_out["is_predicted"] = is_predicted
    for target in chosen_models:
        if target in labeled_agg_by_id.columns:
            mapped = tasks_out["task_id"].map(labeled_agg_by_id[target])
            tasks_out.loc[~is_predicted, f"est_{target}"] = mapped[~is_predicted].values
            tasks_out.loc[~is_predicted, f"est_{target}_lo"] = mapped[~is_predicted].values
            tasks_out.loc[~is_predicted, f"est_{target}_hi"] = mapped[~is_predicted].values

    tasks_out.to_csv(out_csv, index=False)
    print(f"wrote {out_csv.relative_to(repo_root)} with {len(tasks_out)} rows")

    labeled_days = set(_run_order(labeled).dropna().unique().tolist())
    trend_summary: dict[str, dict] = {}
    if "total_tool_calls" in chosen_models:
        trend_summary["total"] = chart_trend(
            tasks_out, "est_total_tool_calls", out_dir,
            "estimated_tool_calls_trend",
            "Mean predicted total tool calls per task, by run day",
            labeled_days,
        )
    if "go_build_failed_calls" in chosen_models:
        trend_summary["go_build_failed"] = chart_trend(
            tasks_out, "est_go_build_failed_calls", out_dir,
            "estimated_go_build_failures_trend",
            "Mean predicted failed go-build calls per task, by run day",
            labeled_days,
        )
    summary["trend"] = trend_summary
    summary["n_predicted_rows"] = int(is_predicted.sum())
    summary["n_labeled_rows"] = int((~is_predicted).sum())

    with summary_path.open("w") as f:
        yaml.safe_dump(summary, f, sort_keys=False)
    print(f"wrote {summary_path.relative_to(repo_root)}")
    print(f"charts -> {out_dir.relative_to(repo_root)}")
    print()
    print("=== Model R² (cross-validated by run day) ===")
    for tgt, res in target_results.items():
        print(f"  {tgt:30s} best={res['best_model']:18s} R²={res['test_r2']:+.3f}  MAE={res['test_mae']:.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
