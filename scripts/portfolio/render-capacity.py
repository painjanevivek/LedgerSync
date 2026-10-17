"""Render the archived R09 component study. Requires matplotlib 3.10.7.

Run from the repository root with the local plotting environment. This script
reads only the public archived data. It does not start services or workloads.
"""
import json
from pathlib import Path
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

root = Path(__file__).resolve().parents[2]
assets = root / "docs/portfolio/assets"
data = json.loads((assets / "r09-capacity.json").read_text(encoding="utf-8-sig"))
plt.rcParams.update({"font.family": "DejaVu Sans", "font.size": 10, "svg.fonttype": "none", "svg.hashsalt": "ledgersync-r09-v1"})
fig, axes = plt.subplots(1, 2, figsize=(12, 4.6), gridspec_kw={"width_ratios": [1, 1.35]})
fig.patch.set_facecolor("#fbfaf7")
for ax in axes:
    ax.set_facecolor("#fbfaf7")
    ax.spines[["top", "right"]].set_visible(False)
    ax.grid(axis="y", color="#dedbd3", linewidth=.7)
    ax.set_axisbelow(True)
x = np.arange(3)
axes[0].bar(x, data["p95_ms"], color="#21665c", width=.55)
axes[0].set(xticks=x, xticklabels=["Run 1", "Run 2", "Run 3"], ylabel="Command p95 (ms)", ylim=(0, 36), title="Qualified: 10 offered commands/s")
for i, value in enumerate(data["p95_ms"]):
    axes[0].text(i, value + .7, f"{value:.2f}", ha="center")
for index, (row, color) in enumerate(zip(data["calibrations"], ["#656c72", "#21665c", "#bd692c"])):
    axes[1].bar(x + (index - 1) * .23, row["queue_age_seconds"], width=.22, color=color, label=row["name"])
axes[1].axhline(data["queue_age_budget_seconds"], linestyle="--", color="#7d2525", linewidth=1.3, label="5 s age budget")
axes[1].set(xticks=x, xticklabels=["Repeat 1", "Repeat 2", "Repeat 3"], ylabel="Oldest pending event (s)", ylim=(0, 7), title="Unqualified: 20 commands/s calibrations")
axes[1].legend(loc="upper left", ncol=2, fontsize=8, frameon=False)
fig.suptitle("LedgerSync · historical R09 component evidence", x=.06, ha="left", fontsize=15, weight="bold")
fig.text(.06, .045, "One workstation · synthetic USD · PostgreSQL 1 CPU / 768 MiB · Redis 1 CPU / 256 MiB\n300 s qualified stages; 8.8 new postings/s. Short 20/s trials do not qualify sustained capacity. Source: capacity.md", fontsize=9)
fig.subplots_adjust(left=.07, right=.98, top=.81, bottom=.24, wspace=.33)
fig.savefig(assets / "r09-capacity.svg", metadata={"Date": None, "Creator": "LedgerSync: matplotlib 3.10.7; archived R09 data"})
# Keep generated path formatting compatible with the staged whitespace check.
svg = assets / "r09-capacity.svg"
svg.write_text("\n".join(line.rstrip() for line in svg.read_text(encoding="utf-8").splitlines()) + "\n", encoding="utf-8", newline="\n")
preview = root / "data/portfolio/r14-20261015/r09-capacity-preview.png"
preview.parent.mkdir(parents=True, exist_ok=True)
fig.savefig(preview, dpi=150)
plt.close(fig)
print("Rendered public SVG and private preview from archived R09 data.")
