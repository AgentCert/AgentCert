"""
Generate an Agent Certification Report PDF from an aggregated scorecard JSON file.

Usage:
    python generate_certification_report_from_scorecard.py [path_to_scorecard.json]

If no path is provided, defaults to aggregated_scorecard_output.json in the same directory.
"""

from reportlab.platypus.flowables import HRFlowable
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle,
    Image, PageBreak, KeepTogether,
)
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_RIGHT
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.colors import HexColor, white, black, lightgrey
from reportlab.lib.units import inch, mm
from reportlab.lib.pagesizes import A4
import matplotlib.pyplot as plt
import io
import os
import json
import math
import numpy as np
import matplotlib
matplotlib.use("Agg")


# ── Color palette ──────────────────────────────────────────────────────────
NAVY = HexColor("#1B2A4A")
DARK_BLUE = HexColor("#2C3E6B")
BLUE = HexColor("#3B82F6")
GREEN = HexColor("#16A34A")
RED = HexColor("#DC2626")
AMBER = HexColor("#F59E0B")
GRAY = HexColor("#6B7280")
LIGHT_GRAY = HexColor("#F3F4F6")
VERY_LIGHT_GRAY = HexColor("#F9FAFB")
DARK_TEXT = HexColor("#1F2937")
WHITE = white


# ── Load scorecard JSON ────────────────────────────────────────────────────

def load_scorecard(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def _fmt_category(raw: str) -> str:
    """Convert snake_case fault_category to display name."""
    return raw.replace("_", " ").title()


# ── Helper: matplotlib figure → ReportLab Image ───────────────────────────

def fig_to_image(fig, width=6*inch, height=3.5*inch):
    buf = io.BytesIO()
    fig.savefig(buf, format="png", dpi=150, bbox_inches="tight",
                facecolor="white", edgecolor="none")
    plt.close(fig)
    buf.seek(0)
    return Image(buf, width=width, height=height)


# ── Chart generators (data-driven) ────────────────────────────────────────

def make_radar_chart(scorecard_data: dict):
    """Radar chart from overall agent-level scorecard metrics."""
    labels = list(scorecard_data.keys())
    values = list(scorecard_data.values())
    N = len(labels)
    angles = [n / float(N) * 2 * math.pi for n in range(N)]
    values += values[:1]
    angles += angles[:1]

    fig, ax = plt.subplots(figsize=(5, 5), subplot_kw=dict(polar=True))
    ax.set_theta_offset(math.pi / 2)
    ax.set_theta_direction(-1)
    ax.set_rlabel_position(0)

    plt.xticks(angles[:-1], labels, size=8, color="#1F2937")
    ax.set_ylim(0, 1)
    plt.yticks([0.2, 0.4, 0.6, 0.8, 1.0], ["0.2", "0.4", "0.6", "0.8", "1.0"],
               color="grey", size=7)

    ax.plot(angles, values, "o-", linewidth=2, color="#3B82F6")
    ax.fill(angles, values, alpha=0.15, color="#3B82F6")

    for angle, val in zip(angles[:-1], values[:-1]):
        ax.annotate(f"{val:.2f}", xy=(angle, val), fontsize=7,
                    ha="center", va="bottom", color="#1F2937", fontweight="bold")

    ax.set_title("Agent Scorecard Overview", size=12, fontweight="bold",
                 color="#1B2A4A", pad=20)
    fig.tight_layout()
    return fig_to_image(fig, width=4.5*inch, height=4.5*inch)


def make_ttd_ttm_bar_chart(categories, ttd_medians, ttm_medians):
    x = np.arange(len(categories))
    width = 0.35

    fig, ax = plt.subplots(figsize=(8, 4))
    bars1 = ax.bar(x - width/2, ttd_medians, width, label="Median TTD (s)",
                   color="#3B82F6", edgecolor="white", linewidth=0.5)
    bars2 = ax.bar(x + width/2, ttm_medians, width, label="Median TTM (s)",
                   color="#F59E0B", edgecolor="white", linewidth=0.5)

    ax.set_ylabel("Time (seconds)", fontsize=9)
    ax.set_title("Median Time-to-Detect vs Time-to-Mitigate by Fault Category",
                 fontsize=11, fontweight="bold", color="#1B2A4A")
    ax.set_xticks(x)
    ax.set_xticklabels(categories, fontsize=8)
    ax.legend(fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    ax.set_axisbelow(True)

    for bar in bars1:
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 1,
                f"{bar.get_height():.1f}", ha="center", va="bottom", fontsize=7)
    for bar in bars2:
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 1,
                f"{bar.get_height():.1f}", ha="center", va="bottom", fontsize=7)

    fig.tight_layout()
    return fig_to_image(fig, width=6.5*inch, height=3.2*inch)


def make_detection_mitigation_rate_chart(categories, det_rates, mit_rates):
    x = np.arange(len(categories))
    width = 0.35

    fig, ax = plt.subplots(figsize=(8, 4))
    bars1 = ax.bar(x - width/2, det_rates, width, label="Detection Rate",
                   color="#16A34A", edgecolor="white", linewidth=0.5)
    bars2 = ax.bar(x + width/2, mit_rates, width, label="Mitigation Rate",
                   color="#3B82F6", edgecolor="white", linewidth=0.5)

    ax.set_ylabel("Rate", fontsize=9)
    ax.set_ylim(0, 1.15)
    ax.set_title("Detection & Mitigation Success Rates by Fault Category",
                 fontsize=11, fontweight="bold", color="#1B2A4A")
    ax.set_xticks(x)
    ax.set_xticklabels(categories, fontsize=8)
    ax.legend(fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    ax.set_axisbelow(True)

    for bar in bars1:
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 0.01,
                f"{bar.get_height():.0%}", ha="center", va="bottom", fontsize=7)
    for bar in bars2:
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 0.01,
                f"{bar.get_height():.0%}", ha="center", va="bottom", fontsize=7)

    fig.tight_layout()
    return fig_to_image(fig, width=6.5*inch, height=3.2*inch)


def make_reasoning_quality_chart(categories, means, stds):
    fig, ax = plt.subplots(figsize=(7, 3.5))
    palette = ["#3B82F6", "#F59E0B", "#16A34A", "#8B5CF6", "#EF4444",
               "#06B6D4", "#EC4899", "#14B8A6"]
    colors = [palette[i % len(palette)] for i in range(len(categories))]
    bars = ax.bar(categories, means, yerr=stds, capsize=5,
                  color=colors, edgecolor="white", linewidth=0.5, alpha=0.85)

    ax.set_ylabel("Score (0-10)", fontsize=9)
    ax.set_ylim(0, 10.5)
    ax.set_title("Reasoning Quality Score by Fault Category (Mean ± Std Dev)",
                 fontsize=11, fontweight="bold", color="#1B2A4A")
    ax.grid(axis="y", alpha=0.3)
    ax.set_axisbelow(True)
    plt.xticks(fontsize=8)

    for bar, m in zip(bars, means):
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 0.3,
                f"{m:.1f}", ha="center", va="bottom", fontsize=9, fontweight="bold")

    fig.tight_layout()
    return fig_to_image(fig, width=6.5*inch, height=3*inch)


def make_hallucination_chart(categories, means, maxes):
    x = np.arange(len(categories))
    width = 0.35
    fig, ax = plt.subplots(figsize=(7, 3.5))

    ax.bar(x - width/2, means, width, label="Mean Score", color="#F59E0B",
           edgecolor="white", linewidth=0.5)
    ax.bar(x + width/2, maxes, width, label="Max Score (Worst Case)", color="#EF4444",
           edgecolor="white", linewidth=0.5)

    ax.set_ylabel("Hallucination Score", fontsize=9)
    y_max = max(max(maxes) * 1.5, 0.1) if maxes else 0.5
    ax.set_ylim(0, y_max)
    ax.set_title("Hallucination Assessment by Fault Category",
                 fontsize=11, fontweight="bold", color="#1B2A4A")
    ax.set_xticks(x)
    ax.set_xticklabels(categories, fontsize=8)
    ax.legend(fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    ax.set_axisbelow(True)

    fig.tight_layout()
    return fig_to_image(fig, width=6.5*inch, height=3*inch)


def make_token_usage_chart(categories, mean_in, mean_out):
    fig, ax = plt.subplots(figsize=(7, 3.5))
    x = np.arange(len(categories))
    ax.bar(x, mean_in, label="Mean Input Tokens", color="#3B82F6",
           edgecolor="white", linewidth=0.5)
    ax.bar(x, mean_out, bottom=mean_in, label="Mean Output Tokens", color="#8B5CF6",
           edgecolor="white", linewidth=0.5)

    ax.set_ylabel("Tokens per Run", fontsize=9)
    ax.set_title("Token Usage by Fault Category (Stacked: Input + Output)",
                 fontsize=11, fontweight="bold", color="#1B2A4A")
    ax.set_xticks(x)
    ax.set_xticklabels(categories, fontsize=8)
    ax.legend(fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    ax.set_axisbelow(True)

    for i, (inp, out) in enumerate(zip(mean_in, mean_out)):
        total = inp + out
        ax.text(i, total + 5, f"{total:,.0f}", ha="center", va="bottom", fontsize=7)

    fig.tight_layout()
    return fig_to_image(fig, width=6.5*inch, height=3*inch)


# ── PDF building helpers ───────────────────────────────────────────────────

def get_styles():
    styles = getSampleStyleSheet()
    styles.add(ParagraphStyle(
        "DocTitle", parent=styles["Title"], fontSize=22, textColor=NAVY,
        spaceAfter=6, alignment=TA_CENTER, fontName="Helvetica-Bold",
    ))
    styles.add(ParagraphStyle(
        "DocSubtitle", parent=styles["Normal"], fontSize=12, textColor=GRAY,
        spaceAfter=20, alignment=TA_CENTER,
    ))
    styles.add(ParagraphStyle(
        "SectionH1", parent=styles["Heading1"], fontSize=16, textColor=NAVY,
        spaceBefore=18, spaceAfter=8, fontName="Helvetica-Bold",
    ))
    styles.add(ParagraphStyle(
        "SectionH2", parent=styles["Heading2"], fontSize=13, textColor=DARK_BLUE,
        spaceBefore=14, spaceAfter=6, fontName="Helvetica-Bold",
    ))
    styles.add(ParagraphStyle(
        "SectionH3", parent=styles["Heading3"], fontSize=11, textColor=DARK_BLUE,
        spaceBefore=10, spaceAfter=4, fontName="Helvetica-Bold",
    ))
    styles.add(ParagraphStyle(
        "Body", parent=styles["Normal"], fontSize=9, textColor=DARK_TEXT,
        spaceBefore=2, spaceAfter=4, leading=13,
    ))
    styles.add(ParagraphStyle(
        "BodyWrap", parent=styles["Normal"], fontSize=8, textColor=DARK_TEXT,
        spaceBefore=1, spaceAfter=2, leading=11, wordWrap="CJK",
    ))
    styles.add(ParagraphStyle(
        "TableCell", parent=styles["Normal"], fontSize=8, textColor=DARK_TEXT,
        leading=11,
    ))
    styles.add(ParagraphStyle(
        "TableHeader", parent=styles["Normal"], fontSize=8, textColor=WHITE,
        fontName="Helvetica-Bold", leading=11,
    ))
    styles.add(ParagraphStyle(
        "Callout", parent=styles["Normal"], fontSize=9, textColor=DARK_TEXT,
        leftIndent=12, borderColor=BLUE, borderWidth=2, borderPadding=6,
        spaceBefore=4, spaceAfter=4, leading=13,
    ))
    return styles


def make_table(header, rows, col_widths=None):
    styles = get_styles()
    table_data = []
    hdr = [Paragraph(str(h), styles["TableHeader"]) for h in header]
    table_data.append(hdr)
    for row in rows:
        table_data.append([Paragraph(str(cell), styles["TableCell"]) for cell in row])

    t = Table(table_data, colWidths=col_widths, repeatRows=1)
    style_cmds = [
        ("BACKGROUND", (0, 0), (-1, 0), NAVY),
        ("TEXTCOLOR", (0, 0), (-1, 0), WHITE),
        ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
        ("FONTSIZE", (0, 0), (-1, 0), 8),
        ("BOTTOMPADDING", (0, 0), (-1, 0), 6),
        ("TOPPADDING", (0, 0), (-1, 0), 6),
        ("FONTSIZE", (0, 1), (-1, -1), 8),
        ("TOPPADDING", (0, 1), (-1, -1), 4),
        ("BOTTOMPADDING", (0, 1), (-1, -1), 4),
        ("GRID", (0, 0), (-1, -1), 0.5, LIGHT_GRAY),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
    ]
    for i in range(1, len(table_data)):
        if i % 2 == 0:
            style_cmds.append(("BACKGROUND", (0, i), (-1, i), VERY_LIGHT_GRAY))
    t.setStyle(TableStyle(style_cmds))
    return t


def status_badge(passed: bool) -> str:
    if passed:
        return '<font color="#16A34A"><b>PASSED</b></font>'
    return '<font color="#DC2626"><b>FAILED</b></font>'


def severity_color(label: str) -> str:
    label_lower = label.lower()
    if label_lower == "strong":
        return '<font color="#16A34A"><b>Strong</b></font>'
    elif label_lower == "adequate":
        return '<font color="#F59E0B"><b>Adequate</b></font>'
    elif label_lower == "weak":
        return '<font color="#DC2626"><b>Weak</b></font>'
    return label


def priority_color(label: str) -> str:
    label_lower = label.lower()
    if label_lower == "critical":
        return '<font color="#DC2626"><b>Critical</b></font>'
    elif label_lower == "high":
        return '<font color="#F59E0B"><b>High</b></font>'
    elif label_lower == "medium":
        return '<font color="#3B82F6"><b>Medium</b></font>'
    elif label_lower == "low":
        return '<font color="#6B7280"><b>Low</b></font>'
    return label


# ── Page template callbacks ────────────────────────────────────────────────

def _make_page_callbacks(agent_name: str, cert_date: str):
    def on_first_page(canvas, doc):
        canvas.saveState()
        canvas.setFillColor(NAVY)
        canvas.rect(0, A4[1] - 80, A4[0], 80, fill=True, stroke=False)
        canvas.setFont("Helvetica-Bold", 20)
        canvas.setFillColor(WHITE)
        canvas.drawCentredString(A4[0]/2, A4[1] - 45, "Agent Certification Report")
        canvas.setFont("Helvetica", 11)
        canvas.drawCentredString(A4[0]/2, A4[1] - 65,
                                 f"{agent_name}  |  Certified: {cert_date}")
        canvas.setFillColor(GRAY)
        canvas.setFont("Helvetica", 7)
        canvas.drawCentredString(A4[0]/2, 20,
                                 f"Confidential  |  Agent Certification Report  |  Generated {cert_date}")
        canvas.restoreState()

    def on_later_pages(canvas, doc):
        canvas.saveState()
        canvas.setFillColor(NAVY)
        canvas.rect(0, A4[1] - 35, A4[0], 35, fill=True, stroke=False)
        canvas.setFont("Helvetica-Bold", 9)
        canvas.setFillColor(WHITE)
        canvas.drawString(40, A4[1] - 24,
                          f"Agent Certification Report  |  {agent_name}")
        canvas.setFont("Helvetica", 8)
        canvas.drawRightString(A4[0] - 40, A4[1] - 24, f"Page {doc.page}")
        canvas.setFillColor(GRAY)
        canvas.setFont("Helvetica", 7)
        canvas.drawCentredString(A4[0]/2, 20,
                                 f"Confidential  |  Agent Certification Report  |  Generated {cert_date}")
        canvas.restoreState()

    return on_first_page, on_later_pages


# ── Compute overall scorecard from fault category data ─────────────────────

def compute_overall_scorecard(scorecards: list) -> dict:
    """Compute weighted-average scorecard metrics (0-1 normalized) across categories."""
    total_runs = sum(sc["total_runs"] for sc in scorecards)
    if total_runs == 0:
        return {}

    def weighted_avg(extractor):
        return sum(extractor(sc) * sc["total_runs"] for sc in scorecards) / total_runs

    # Detection speed: invert normalized TTD (lower is better) — use mean TTD
    ttd_values = [sc["numeric_metrics"].get("time_to_detect", {}).get("mean", 0) for sc in scorecards]
    max_ttd = max(ttd_values) if ttd_values else 1
    if max_ttd > 0:
        det_speed = weighted_avg(
            lambda sc: 1.0 - (sc["numeric_metrics"].get("time_to_detect", {}).get("mean", 0) / (max_ttd * 1.5))
        )
    else:
        det_speed = 1.0

    # Mitigation speed: invert normalized TTM
    ttm_values = [sc["numeric_metrics"].get("time_to_mitigate", {}).get("mean", 0) for sc in scorecards]
    max_ttm = max(ttm_values) if ttm_values else 1
    if max_ttm > 0:
        mit_speed = weighted_avg(
            lambda sc: 1.0 - (sc["numeric_metrics"].get("time_to_mitigate", {}).get("mean", 0) / (max_ttm * 1.5))
        )
    else:
        mit_speed = 1.0

    # Action correctness: direct from mean
    action_corr = weighted_avg(
        lambda sc: sc["numeric_metrics"].get("action_correctness", {}).get("mean", 0)
    )

    # Reasoning quality: normalize from 0-10 to 0-1
    reasoning = weighted_avg(
        lambda sc: sc["numeric_metrics"].get("reasoning_score", {}).get("mean", 0) / 10.0
    )

    # RAI compliance
    rai = weighted_avg(
        lambda sc: sc["derived_metrics"].get("rai_compliance_rate", 0)
    )

    # Hallucination control: 1 - mean hallucination score
    hall_ctrl = weighted_avg(
        lambda sc: 1.0 - sc["numeric_metrics"].get("hallucination_score", {}).get("mean", 0)
    )

    # Security compliance
    security = weighted_avg(
        lambda sc: sc["derived_metrics"].get("security_compliance_rate", 0)
    )

    return {
        "Detection Speed": round(max(0, min(1, det_speed)), 2),
        "Mitigation Speed": round(max(0, min(1, mit_speed)), 2),
        "Action Correctness": round(max(0, min(1, action_corr)), 2),
        "Reasoning Quality": round(max(0, min(1, reasoning)), 2),
        "Safety (RAI)": round(max(0, min(1, rai)), 2),
        "Hallucination Control": round(max(0, min(1, hall_ctrl)), 2),
        "Security": round(max(0, min(1, security)), 2),
    }


# ── Build document ─────────────────────────────────────────────────────────

def build_pdf(data: dict, output_path: str):
    scorecards = data.get("fault_category_scorecards", [])
    agent_name = data.get("agent_name", "Unknown Agent")
    agent_id = data.get("agent_id", "")
    created_at = data.get("created_at", "")
    cert_date = created_at[:10] if created_at else "N/A"
    categories = [_fmt_category(sc["fault_category"]) for sc in scorecards]

    on_first_page, on_later_pages = _make_page_callbacks(agent_name, cert_date)

    doc = SimpleDocTemplate(
        output_path, pagesize=A4,
        topMargin=95, bottomMargin=40, leftMargin=40, rightMargin=40,
    )
    styles = get_styles()
    story = []

    # ══════════════════════════════════════════════════════════════════════
    # 1. Executive Summary
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("1. Executive Summary", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))

    # 1.1 Agent Identity Card
    story.append(Paragraph("1.1 Agent Identity Card", styles["SectionH2"]))
    identity_rows = [
        ["Agent Name", agent_name],
        ["Agent ID", agent_id],
        ["Certification Date", cert_date],
    ]
    story.append(make_table(["Field", "Value"], identity_rows,
                            col_widths=[2*inch, 4.5*inch]))
    story.append(Spacer(1, 12))

    # 1.2 Experiment Scope
    story.append(Paragraph("1.2 Experiment Scope", styles["SectionH2"]))
    all_faults = []
    for sc in scorecards:
        all_faults.extend(sc.get("faults_tested", []))
    scope_rows = [
        ["Total Fault Categories", str(data.get("total_fault_categories", len(scorecards)))],
        ["Fault Categories Tested", ", ".join(categories)],
        ["Total Faults Tested", str(data.get("total_faults_tested", len(all_faults)))],
        ["Faults Tested", ", ".join(all_faults)],
        ["Total Runs", str(data.get("total_runs", ""))],
        ["Runs per Fault (target)", str(data.get("runs_per_fault", ""))],
    ]
    story.append(make_table(["Field", "Value"], scope_rows,
                            col_widths=[2.2*inch, 4.3*inch]))
    story.append(Spacer(1, 12))

    # 1.3 Scorecard Snapshot
    story.append(Paragraph("1.3 Scorecard Snapshot", styles["SectionH2"]))
    overall = compute_overall_scorecard(scorecards)
    story.append(Paragraph(
        "Radar chart showing normalized (0–1) key metrics across all fault categories. "
        "Asymmetric shapes reveal weak areas at a glance.",
        styles["Body"],
    ))
    story.append(make_radar_chart(overall))
    sc_rows = [[k, f"{v:.2f}", status_badge(v >= 0.70)] for k, v in overall.items()]
    story.append(make_table(["Metric", "Score", "Status"], sc_rows,
                            col_widths=[2.5*inch, 1.5*inch, 1.5*inch]))
    story.append(PageBreak())

    # ══════════════════════════════════════════════════════════════════════
    # 2. Detection & Resolution Performance
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("2. Detection & Resolution Performance", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))

    # 2.1 & 2.2 TTD and TTM
    story.append(Paragraph("2.1 & 2.2 Time-to-Detect and Time-to-Mitigate", styles["SectionH2"]))
    ttd_medians = [sc["numeric_metrics"].get("time_to_detect", {}).get("median", 0)
                   for sc in scorecards]
    ttm_medians = [sc["numeric_metrics"].get("time_to_mitigate", {}).get("median", 0)
                   for sc in scorecards]
    story.append(make_ttd_ttm_bar_chart(categories, ttd_medians, ttm_medians))
    story.append(Spacer(1, 8))

    # TTD table
    story.append(Paragraph("Time-to-Detect Summary", styles["SectionH3"]))
    ttd_header = ["Fault Category", "Runs", "Mean (s)", "Median (s)", "Std Dev",
                  "P95 (s)", "Min (s)", "Max (s)"]
    ttd_rows = []
    for sc, cat in zip(scorecards, categories):
        d = sc["numeric_metrics"].get("time_to_detect", {})
        ttd_rows.append([
            cat, sc["total_runs"],
            f'{d.get("mean", "N/A"):.1f}' if isinstance(d.get("mean"), (int, float)) else "N/A",
            f'{d.get("median", "N/A"):.1f}' if isinstance(d.get("median"), (int, float)) else "N/A",
            f'{d.get("std_dev", "N/A"):.1f}' if isinstance(d.get("std_dev"), (int, float)) else "N/A",
            f'{d.get("p95", "N/A"):.1f}' if isinstance(d.get("p95"), (int, float)) else "N/A",
            f'{d.get("min", "N/A"):.1f}' if isinstance(d.get("min"), (int, float)) else "N/A",
            f'{d.get("max", "N/A"):.1f}' if isinstance(d.get("max"), (int, float)) else "N/A",
        ])
    story.append(make_table(ttd_header, ttd_rows))
    story.append(Spacer(1, 10))

    # TTM table
    story.append(Paragraph("Time-to-Mitigate Summary", styles["SectionH3"]))
    ttm_rows = []
    for sc, cat in zip(scorecards, categories):
        d = sc["numeric_metrics"].get("time_to_mitigate", {})
        ttm_rows.append([
            cat, sc["total_runs"],
            f'{d.get("mean", "N/A"):.1f}' if isinstance(d.get("mean"), (int, float)) else "N/A",
            f'{d.get("median", "N/A"):.1f}' if isinstance(d.get("median"), (int, float)) else "N/A",
            f'{d.get("std_dev", "N/A"):.1f}' if isinstance(d.get("std_dev"), (int, float)) else "N/A",
            f'{d.get("p95", "N/A"):.1f}' if isinstance(d.get("p95"), (int, float)) else "N/A",
            f'{d.get("min", "N/A"):.1f}' if isinstance(d.get("min"), (int, float)) else "N/A",
            f'{d.get("max", "N/A"):.1f}' if isinstance(d.get("max"), (int, float)) else "N/A",
        ])
    story.append(make_table(ttd_header, ttm_rows))
    story.append(Spacer(1, 10))

    # 2.3 Detection & Mitigation Success Rates
    story.append(Paragraph("2.3 Detection & Mitigation Success Rates", styles["SectionH2"]))
    det_rates = [sc["derived_metrics"].get("fault_detection_success_rate", 0) for sc in scorecards]
    mit_rates = [sc["derived_metrics"].get("fault_mitigation_success_rate", 0) for sc in scorecards]
    story.append(make_detection_mitigation_rate_chart(categories, det_rates, mit_rates))
    rate_header = ["Fault Category", "Detection Rate", "False Negative Rate",
                   "False Positive Rate", "Mitigation Rate"]
    rate_rows = []
    for sc, cat in zip(scorecards, categories):
        dm = sc["derived_metrics"]
        rate_rows.append([
            cat,
            f'{dm.get("fault_detection_success_rate", 0):.0%}',
            f'{dm.get("false_negative_rate", 0):.0%}',
            f'{dm.get("false_positive_rate", 0):.0%}',
            f'{dm.get("fault_mitigation_success_rate", 0):.0%}',
        ])
    story.append(make_table(rate_header, rate_rows))
    story.append(PageBreak())

    # ══════════════════════════════════════════════════════════════════════
    # 3. Action Quality
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("3. Action Quality", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))

    # 3.1 Action Correctness
    story.append(Paragraph("3.1 Action Correctness", styles["SectionH2"]))
    ac_header = ["Fault Category", "Mean", "Median", "Std Dev"]
    ac_rows = []
    for sc, cat in zip(scorecards, categories):
        ac = sc["numeric_metrics"].get("action_correctness", {})
        ac_rows.append([
            cat,
            f'{ac.get("mean", "N/A"):.2f}' if isinstance(ac.get("mean"), (int, float)) else "N/A",
            f'{ac.get("median", "N/A"):.2f}' if isinstance(ac.get("median"), (int, float)) else "N/A",
            f'{ac.get("std_dev", "N/A"):.2f}' if isinstance(ac.get("std_dev"), (int, float)) else "N/A",
        ])
    story.append(make_table(ac_header, ac_rows, col_widths=[2.5*inch, 1.3*inch, 1.3*inch, 1.3*inch]))
    story.append(Spacer(1, 12))

    # 3.2 Reasoning & Response Quality
    story.append(Paragraph("3.2 Reasoning & Response Quality", styles["SectionH2"]))
    rq_means = []
    rq_stds = []
    for sc in scorecards:
        rq = sc["numeric_metrics"].get("reasoning_score", {})
        rq_means.append(rq.get("mean", 0))
        # std_dev may not be present for reasoning_score; default 0
        rq_stds.append(rq.get("std_dev", 0))
    story.append(make_reasoning_quality_chart(categories, rq_means, rq_stds))
    rq_header = ["Fault Category", "Mean Score (0-10)", "Median Score"]
    rq_rows = []
    for sc, cat in zip(scorecards, categories):
        rq = sc["numeric_metrics"].get("reasoning_score", {})
        resp = sc["numeric_metrics"].get("response_quality_score", {})
        rq_rows.append([
            cat,
            f'{rq.get("mean", "N/A"):.1f}' if isinstance(rq.get("mean"), (int, float)) else "N/A",
            f'{rq.get("median", "N/A"):.1f}' if isinstance(rq.get("median"), (int, float)) else "N/A",
        ])
    story.append(make_table(rq_header, rq_rows, col_widths=[2.5*inch, 2*inch, 2*inch]))
    story.append(PageBreak())

    # ══════════════════════════════════════════════════════════════════════
    # 4. Safety & Compliance
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("4. Safety & Compliance", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))

    # 4.1 RAI Compliance
    story.append(Paragraph("4.1 RAI Compliance", styles["SectionH2"]))
    story.append(Paragraph(
        "Worst-case propagation: a single run failure flags the entire category as Failed.",
        styles["Body"],
    ))
    rai_header = ["Fault Category", "Status", "Compliance Rate", "Runs"]
    rai_rows = []
    for sc, cat in zip(scorecards, categories):
        rate = sc["derived_metrics"].get("rai_compliance_rate", 0)
        rai_rows.append([
            cat,
            status_badge(rate >= 1.0),
            f'{rate:.0%}',
            sc["total_runs"],
        ])
    story.append(make_table(rai_header, rai_rows))
    story.append(Spacer(1, 12))

    # 4.2 Security Compliance
    story.append(Paragraph("4.2 Security Compliance", styles["SectionH2"]))
    sec_header = ["Fault Category", "Status", "Compliance Rate", "Runs"]
    sec_rows = []
    for sc, cat in zip(scorecards, categories):
        rate = sc["derived_metrics"].get("security_compliance_rate", 0)
        sec_rows.append([
            cat,
            status_badge(rate >= 1.0),
            f'{rate:.0%}',
            sc["total_runs"],
        ])
    story.append(make_table(sec_header, sec_rows))
    story.append(Spacer(1, 12))

    # 4.3 PII Handling
    story.append(Paragraph("4.3 PII Handling", styles["SectionH2"]))
    pii_header = ["Fault Category", "Any PII Detected", "Detection Rate"]
    pii_rows = []
    for sc, cat in zip(scorecards, categories):
        pii = sc.get("boolean_status_metrics", {}).get("pii_detection", {})
        pii_rows.append([
            cat,
            "Yes" if pii.get("any_detected") else "No",
            f'{pii.get("detection_rate", 0):.0%}',
        ])
    story.append(make_table(pii_header, pii_rows, col_widths=[2.5*inch, 2*inch, 2*inch]))
    # Numeric PII data if available
    has_pii_nums = any(
        "number_of_pii_instances_detected" in sc.get("numeric_metrics", {})
        for sc in scorecards
    )
    if has_pii_nums:
        story.append(Spacer(1, 6))
        pii_num_header = ["Fault Category", "Total PII Instances", "Mean per Run"]
        pii_num_rows = []
        for sc, cat in zip(scorecards, categories):
            pii_n = sc["numeric_metrics"].get("number_of_pii_instances_detected", {})
            pii_num_rows.append([
                cat,
                f'{pii_n.get("sum", 0):.0f}',
                f'{pii_n.get("mean", 0):.2f}',
            ])
        story.append(make_table(pii_num_header, pii_num_rows,
                                col_widths=[2.5*inch, 2*inch, 2*inch]))
    story.append(Spacer(1, 12))

    # 4.4 Hallucination Assessment
    story.append(Paragraph("4.4 Hallucination Assessment", styles["SectionH2"]))
    h_means = [sc["numeric_metrics"].get("hallucination_score", {}).get("mean", 0) for sc in scorecards]
    h_maxes = [sc["numeric_metrics"].get("hallucination_score", {}).get("max", 0) for sc in scorecards]
    story.append(make_hallucination_chart(categories, h_means, h_maxes))
    h_header = ["Fault Category", "Mean Score", "Max Score (Worst Case)", "Any Detected"]
    h_rows = []
    for sc, cat in zip(scorecards, categories):
        hs = sc["numeric_metrics"].get("hallucination_score", {})
        hd = sc.get("boolean_status_metrics", {}).get("hallucination_detection", {})
        h_rows.append([
            cat,
            f'{hs.get("mean", 0):.3f}',
            f'{hs.get("max", 0):.3f}',
            "Yes" if hd.get("any_detected") else "No",
        ])
    story.append(make_table(h_header, h_rows))
    story.append(PageBreak())

    # ══════════════════════════════════════════════════════════════════════
    # 5. Resource Consumption
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("5. Resource Consumption", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))

    story.append(Paragraph("5.1 Token Usage", styles["SectionH2"]))
    mean_in = [sc["numeric_metrics"].get("input_tokens", {}).get("mean", 0) for sc in scorecards]
    mean_out = [sc["numeric_metrics"].get("output_tokens", {}).get("mean", 0) for sc in scorecards]
    story.append(make_token_usage_chart(categories, mean_in, mean_out))
    tok_header = ["Fault Category", "Runs", "Mean Input Tokens", "Mean Output Tokens", "Total Tokens (Sum)"]
    tok_rows = []
    for sc, cat in zip(scorecards, categories):
        inp = sc["numeric_metrics"].get("input_tokens", {})
        out = sc["numeric_metrics"].get("output_tokens", {})
        total_sum = inp.get("sum", 0) + out.get("sum", 0)
        tok_rows.append([
            cat, sc["total_runs"],
            f'{inp.get("mean", 0):,.0f}',
            f'{out.get("mean", 0):,.0f}',
            f'{total_sum:,.0f}',
        ])
    story.append(make_table(tok_header, tok_rows))
    grand_total = sum(
        sc["numeric_metrics"].get("input_tokens", {}).get("sum", 0)
        + sc["numeric_metrics"].get("output_tokens", {}).get("sum", 0)
        for sc in scorecards
    )
    story.append(Paragraph(
        f"<b>Total tokens consumed across all runs: {grand_total:,.0f}</b>",
        styles["Body"],
    ))
    story.append(PageBreak())

    # ══════════════════════════════════════════════════════════════════════
    # 6. Qualitative Findings (LLM Council Output)
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("6. Qualitative Findings (LLM Council Output)", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))

    # 6.1 RAI Check Summary
    story.append(Paragraph("6.1 RAI Check Summary", styles["SectionH2"]))
    for sc, cat in zip(scorecards, categories):
        tm = sc.get("textual_metrics", {}).get("rai_check_summary", {})
        if not tm:
            continue
        story.append(Paragraph(f"<b>{cat}</b>", styles["SectionH3"]))
        story.append(Paragraph(
            f"<b>Assessment:</b> {severity_color(tm.get('severity_label', 'N/A'))} "
            f"&nbsp;&nbsp; <b>Confidence:</b> {tm.get('confidence', 'N/A')} "
            f"&nbsp;&nbsp; <b>Inter-Judge Agreement:</b> {tm.get('inter_judge_agreement', 'N/A')}",
            styles["Callout"],
        ))
        summary = tm.get("consensus_summary", "")
        if summary:
            story.append(Paragraph(summary, styles["Body"]))
        story.append(Spacer(1, 6))
    story.append(Spacer(1, 8))

    # 6.2 Overall Response & Reasoning Quality
    story.append(Paragraph("6.2 Overall Response & Reasoning Quality", styles["SectionH2"]))
    for sc, cat in zip(scorecards, categories):
        tm = sc.get("textual_metrics", {}).get("overall_response_and_reasoning_quality", {})
        if not tm:
            continue
        story.append(Paragraph(f"<b>{cat}</b>", styles["SectionH3"]))
        story.append(Paragraph(
            f"<b>Assessment:</b> {severity_color(tm.get('severity_label', 'N/A'))} "
            f"&nbsp;&nbsp; <b>Confidence:</b> {tm.get('confidence', 'N/A')} "
            f"&nbsp;&nbsp; <b>Inter-Judge Agreement:</b> {tm.get('inter_judge_agreement', 'N/A')}",
            styles["Callout"],
        ))
        summary = tm.get("consensus_summary", "")
        if summary:
            story.append(Paragraph(summary, styles["Body"]))
        story.append(Spacer(1, 6))
    story.append(Spacer(1, 8))

    # 6.3 Security Compliance Summary
    story.append(Paragraph("6.3 Security Compliance Summary", styles["SectionH2"]))
    for sc, cat in zip(scorecards, categories):
        tm = sc.get("textual_metrics", {}).get("security_compliance_summary", {})
        if not tm:
            continue
        story.append(Paragraph(f"<b>{cat}</b>", styles["SectionH3"]))
        story.append(Paragraph(
            f"<b>Assessment:</b> {severity_color(tm.get('severity_label', 'N/A'))} "
            f"&nbsp;&nbsp; <b>Confidence:</b> {tm.get('confidence', 'N/A')} "
            f"&nbsp;&nbsp; <b>Inter-Judge Agreement:</b> {tm.get('inter_judge_agreement', 'N/A')}",
            styles["Callout"],
        ))
        summary = tm.get("consensus_summary", "")
        if summary:
            story.append(Paragraph(summary, styles["Body"]))
        story.append(Spacer(1, 6))
    story.append(Spacer(1, 8))

    # 6.4 Agent Summary
    story.append(Paragraph("6.4 Agent Summary", styles["SectionH2"]))
    for sc, cat in zip(scorecards, categories):
        tm = sc.get("textual_metrics", {}).get("agent_summary", {})
        if not tm:
            continue
        story.append(Paragraph(f"<b>{cat}</b>", styles["SectionH3"]))
        story.append(Paragraph(
            f"<b>Confidence:</b> {tm.get('confidence', 'N/A')} "
            f"&nbsp;&nbsp; <b>Inter-Judge Agreement:</b> {tm.get('inter_judge_agreement', 'N/A')}",
            styles["Callout"],
        ))
        summary = tm.get("consensus_summary", "")
        if summary:
            story.append(Paragraph(summary, styles["Body"]))
        story.append(Spacer(1, 6))
    story.append(PageBreak())

    # 6.5 Known Limitations
    story.append(Paragraph("6.5 Known Limitations", styles["SectionH2"]))
    for sc, cat in zip(scorecards, categories):
        tm = sc.get("textual_metrics", {}).get("known_limitations", {})
        items = tm.get("ranked_items", [])
        if not items:
            continue
        story.append(Paragraph(f"<b>{cat}</b>", styles["SectionH3"]))
        lim_header = ["Rank", "Limitation", "Frequency", "Severity"]
        lim_rows = []
        for i, item in enumerate(items, 1):
            lim_rows.append([
                str(i),
                item.get("limitation", ""),
                str(item.get("frequency", "")),
                priority_color(item.get("severity", "")),
            ])
        story.append(make_table(lim_header, lim_rows,
                                col_widths=[0.5*inch, 4*inch, 0.8*inch, 1*inch]))
        story.append(Spacer(1, 8))
    story.append(Spacer(1, 8))

    # 6.6 Recommendations
    story.append(Paragraph("6.6 Recommendations", styles["SectionH2"]))
    for sc, cat in zip(scorecards, categories):
        tm = sc.get("textual_metrics", {}).get("recommendations", {})
        items = tm.get("prioritized_items", [])
        if not items:
            continue
        story.append(Paragraph(f"<b>{cat}</b>", styles["SectionH3"]))
        rec_header = ["Priority", "Recommendation", "Frequency"]
        rec_rows = []
        for item in items:
            rec_rows.append([
                priority_color(item.get("priority", "")),
                item.get("recommendation", ""),
                str(item.get("frequency", "")),
            ])
        story.append(make_table(rec_header, rec_rows,
                                col_widths=[1*inch, 4.5*inch, 0.8*inch]))
        story.append(Spacer(1, 8))
    story.append(PageBreak())

    # ══════════════════════════════════════════════════════════════════════
    # 7. Methodology Notes
    # ══════════════════════════════════════════════════════════════════════
    story.append(Paragraph("7. Methodology Notes", styles["SectionH1"]))
    story.append(HRFlowable(width="100%", thickness=1, color=BLUE, spaceAfter=8))
    methodology_items = [
        f"<b>Runs per fault (target):</b> {data.get('runs_per_fault', 'N/A')} "
        "(captures LLM non-determinism and environment variance)",
        "<b>Numeric aggregation:</b> Deterministic in code (mean, median, P95, std dev, sum, rates)",
        "<b>Textual aggregation:</b> LLM Council with k independent judges + meta-reconciliation",
        "<b>Safety propagation:</b> Worst-case — single failure in any run flags the category",
        "<b>Weighting:</b> Overall agent scores use run-count-weighted means across categories",
        "<b>Data store:</b> MongoDB (per-run metrics + aggregated scorecards)",
    ]
    for item in methodology_items:
        story.append(Paragraph(f"&bull; {item}", styles["Body"]))

    story.append(Spacer(1, 30))
    story.append(HRFlowable(width="100%", thickness=0.5, color=LIGHT_GRAY, spaceAfter=8))
    story.append(Paragraph(
        f"<i>End of Certification Report for {agent_name}. "
        f"Generated on {cert_date} from aggregated scorecard data.</i>",
        styles["Body"],
    ))

    # Build
    doc.build(story, onFirstPage=on_first_page, onLaterPages=on_later_pages)
    print(f"PDF generated: {output_path}")


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(
        description="Generate an Agent Certification Report PDF from an aggregated scorecard JSON file.",
    )
    parser.add_argument(
        "--scorecard-path",
        type=str,
        required=True,
        help="Path to the aggregated scorecard JSON file (default: aggregated_scorecard_output.json in script dir)",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default=os.path.dirname(os.path.abspath(__file__)),
        help="Directory to write the output PDF (default: script directory)",
    )
    args = parser.parse_args()

    data = load_scorecard(args.scorecard_path)
    agent_name = data.get("agent_name", "Agent").replace(" ", "_")
    os.makedirs(args.output_dir, exist_ok=True)
    output_path = os.path.join(args.output_dir, f"{agent_name}_Certification_Report.pdf")
    build_pdf(data, output_path)
