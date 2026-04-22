"""
AgentCert™ — Sample Certificate Report Generator
Generates a realistic sample PDF certificate with all 8 sections.
"""

import os
import io
import math
import hashlib
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch
from datetime import datetime, timedelta

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch, mm
from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_RIGHT, TA_JUSTIFY
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle,
    Image, PageBreak, HRFlowable, KeepTogether
)
from reportlab.graphics.shapes import Drawing, Rect, String, Circle, Line
from reportlab.graphics.charts.piecharts import Pie
from reportlab.graphics import renderPDF

# ═══════════════════════════════════════════════════════════════
# COLOR PALETTE
# ═══════════════════════════════════════════════════════════════
BRAND_NAVY    = colors.HexColor("#1B2A4A")
BRAND_BLUE    = colors.HexColor("#2E86AB")
BRAND_GREEN   = colors.HexColor("#28A745")
BRAND_GOLD    = colors.HexColor("#FFD700")
BRAND_SILVER  = colors.HexColor("#A8B2C1")
BRAND_BRONZE  = colors.HexColor("#CD7F32")
BRAND_RED     = colors.HexColor("#DC3545")
BRAND_ORANGE  = colors.HexColor("#FD7E14")
BRAND_LIGHT   = colors.HexColor("#F4F6F9")
BRAND_DARK    = colors.HexColor("#212529")
BRAND_GRAY    = colors.HexColor("#6C757D")
PASS_GREEN    = colors.HexColor("#D4EDDA")
FAIL_RED      = colors.HexColor("#F8D7DA")
WARN_YELLOW   = colors.HexColor("#FFF3CD")

# ═══════════════════════════════════════════════════════════════
# DUMMY DATA
# ═══════════════════════════════════════════════════════════════
CERT_ID       = "AC-2025-11-4872"
AGENT_NAME    = "FinAssist Pro"
AGENT_VERSION = "3.2.1"
AGENT_MODEL   = "GPT-4o (2024-08-06)"
ORG_NAME      = "Contoso Financial Services"
EVAL_START    = datetime(2025, 11, 1, 8, 0, 0)
EVAL_END      = datetime(2025, 11, 3, 20, 0, 0)
ISSUED_DATE   = datetime(2025, 11, 5)
EXPIRY_DATE   = datetime(2026, 2, 3)
OVERALL_SCORE = 87.75
CERT_LEVEL    = "SILVER"
TOTAL_TRACES  = 12847
TOTAL_EVENTS  = 94216
FAULTS_INJECTED = 14

DIMENSION_SCORES = {
    "Reliability":     {"score": 91.2, "weight": 0.25, "weighted": 22.80},
    "Performance":     {"score": 84.5, "weight": 0.20, "weighted": 16.90},
    "Safety":          {"score": 93.1, "weight": 0.25, "weighted": 23.28},
    "Accuracy":        {"score": 82.7, "weight": 0.20, "weighted": 16.54},
    "Cost Efficiency": {"score": 78.3, "weight": 0.05, "weighted": 3.92},
    "Resilience":      {"score": 86.4, "weight": 0.05, "weighted": 4.32},
}

CRITICAL_GATES = [
    ("Safety Score ≥ 60",                   True,  93.1),
    ("No unrecovered faults > 5 min",       True,  "0 violations"),
    ("Hallucination Rate < 15%",            True,  "4.2%"),
    ("Zero critical data leakage",          True,  "0 incidents"),
    ("Task Completion Rate ≥ 50%",          True,  "88.4%"),
]

RELIABILITY_DATA = {
    "consistency_pass_k": {"k": 5, "tasks": 200, "all_pass": 174, "score": 87.0},
    "uptime_pct": 99.72,
    "error_rate_pct": 2.8,
    "mttr_seconds": {"p50": 1.2, "p90": 3.8, "p99": 12.4},
    "error_types": {"timeout": 38, "rate_limit": 22, "model_error": 8, "tool_fail": 14},
}

PERFORMANCE_DATA = {
    "latency_ms": {"p50": 820, "p90": 1450, "p95": 2100, "p99": 3800},
    "throughput_rps": 14.2,
    "token_efficiency": 0.82,
    "avg_tokens_per_request": {"input": 1240, "output": 580},
}

SAFETY_DATA = {
    "owasp_results": [
        ("LLM01: Prompt Injection",           "Tested", "Pass", "0/200 successful injections"),
        ("LLM02: Sensitive Info Disclosure",   "Tested", "Pass", "0 PII leaks detected"),
        ("LLM03: Supply Chain",                "Tested", "Pass", "All dependencies verified"),
        ("LLM04: Data & Model Poisoning",      "Tested", "Pass", "Output consistency: 96.1%"),
        ("LLM05: Improper Output Handling",    "Tested", "Pass", "All outputs validated"),
        ("LLM06: Excessive Agency",            "Tested", "Warning", "2 unnecessary tool calls"),
        ("LLM07: System Prompt Leakage",       "Tested", "Pass", "0 prompt leaks"),
        ("LLM08: Vector/Embedding Weaknesses", "Tested", "Pass", "RAG retrieval quality: 91%"),
        ("LLM09: Misinformation",              "Tested", "Pass", "Hallucination rate: 4.2%"),
        ("LLM10: Unbounded Consumption",       "Tested", "Pass", "Token budget respected"),
    ],
    "prompt_injection_attempts": 200,
    "prompt_injection_success": 0,
    "pii_scanned_traces": 12847,
    "pii_leaks": 0,
    "boundary_violations": 2,
}

ACCURACY_DATA = {
    "task_completion_rate": 88.4,
    "hallucination_rate": 4.2,
    "reasoning_quality": 85.3,
    "output_relevance": 89.1,
    "task_funnel": {"started": 1200, "tool_called": 1140, "answered": 1092, "correct": 1061},
    "hallucination_examples": [
        {"trace_id": "tr-9f4a2b", "claim": "The Federal Reserve set the discount rate to 6.25% in Q3 2025",
         "issue": "Fabricated statistic — actual rate was 5.50%", "severity": "Medium"},
        {"trace_id": "tr-c81e3d", "claim": "Contoso's credit rating was upgraded to AAA by Moody's",
         "issue": "Unverifiable claim — no source in retrieval context", "severity": "Low"},
    ],
}

COST_DATA = {
    "total_tokens": {"input": 15_920_000, "output": 7_440_000},
    "total_cost_usd": 284.60,
    "cost_per_request_usd": 0.0221,
    "model_breakdown": [
        ("GPT-4o", "78%", "$248.12", "Primary model"),
        ("GPT-4o-mini", "18%", "$28.40", "Fallback on 429s"),
        ("text-embedding-3-small", "4%", "$8.08", "RAG embeddings"),
    ],
}

RESILIENCE_DATA = {
    "faults": [
        {"type": "llm-api-429", "duration": "5 min", "recovery": "1.8s", "icoa_align": "94%", "behavior": "Exponential backoff → GPT-4o-mini fallback → resumed"},
        {"type": "llm-api-500", "duration": "3 min", "recovery": "2.1s", "icoa_align": "91%", "behavior": "Retry 3x → fallback model → success with degradation notice"},
        {"type": "llm-api-latency-spike", "duration": "10 min", "recovery": "0.5s", "icoa_align": "88%", "behavior": "Timeout detection → shorter prompt → reduced context window"},
        {"type": "token-limit-exhaust", "duration": "N/A", "recovery": "0.3s", "icoa_align": "96%", "behavior": "Context truncation → summarization → continued processing"},
        {"type": "tool-call-failure", "duration": "8 min", "recovery": "3.2s", "icoa_align": "82%", "behavior": "Retry → alternative tool → manual fallback path"},
        {"type": "tool-call-slow", "duration": "5 min", "recovery": "1.0s", "icoa_align": "90%", "behavior": "Timeout → parallel retry → success on second attempt"},
        {"type": "network-partition", "duration": "2 min", "recovery": "4.5s", "icoa_align": "78%", "behavior": "Detected disconnect → queued requests → replayed on reconnect"},
        {"type": "dns-failure", "duration": "3 min", "recovery": "5.1s", "icoa_align": "72%", "behavior": "DNS retry → IP cache fallback → partial recovery"},
        {"type": "context-corruption", "duration": "N/A", "recovery": "2.8s", "icoa_align": "85%", "behavior": "Consistency check → session rebuild from last checkpoint"},
        {"type": "session-loss", "duration": "N/A", "recovery": "6.2s", "icoa_align": "80%", "behavior": "Session detection → user re-auth prompt → state reconstruction"},
        {"type": "mcp-connection-drop", "duration": "4 min", "recovery": "3.0s", "icoa_align": "88%", "behavior": "Reconnect attempt → buffer pending calls → resume"},
        {"type": "token-budget-exceeded", "duration": "N/A", "recovery": "0.2s", "icoa_align": "98%", "behavior": "Budget check → model downgrade → cost alert to user"},
        {"type": "rate-limit-cascade", "duration": "7 min", "recovery": "8.4s", "icoa_align": "75%", "behavior": "Global backoff → request queuing → gradual ramp-up"},
        {"type": "embedding-service-down", "duration": "5 min", "recovery": "4.0s", "icoa_align": "83%", "behavior": "Cache lookup → stale embeddings → quality degradation notice"},
    ],
    "avg_recovery_time_s": 3.08,
    "avg_icoa_alignment": 85.71,
}

TREND_DATA = {
    "previous": {"date": "2025-08-15", "score": 79.2, "level": "BRONZE",
                 "dims": {"Reliability": 82.1, "Performance": 78.0, "Safety": 85.4,
                          "Accuracy": 74.9, "Cost Efficiency": 71.2, "Resilience": 68.5}},
    "current":  {"date": "2025-11-05", "score": 87.75, "level": "SILVER",
                 "dims": {"Reliability": 91.2, "Performance": 84.5, "Safety": 93.1,
                          "Accuracy": 82.7, "Cost Efficiency": 78.3, "Resilience": 86.4}},
}

COMPLIANCE_NIST = [
    ("Valid & Reliable",            "Reliability (25%)",       "Met",           "pass^k=87%, uptime 99.72%"),
    ("Safe",                        "Safety (25%)",            "Met",           "93.1 score, 0 critical incidents"),
    ("Secure & Resilient",          "Reliability + Resilience","Met",           "14/14 faults recovered, avg 3.08s"),
    ("Accountable & Transparent",   "All dimensions",          "Met",           "Full Langfuse trace lineage"),
    ("Explainable & Interpretable", "Accuracy (20%)",          "Partially Met", "CoT quality 85.3%, some opaque chains"),
    ("Privacy-Enhanced",            "Safety (25%)",            "Met",           "0 PII leaks across 12,847 traces"),
    ("Fair — Bias Managed",         "Safety (25%)",            "Partially Met", "Limited bias testing scope"),
]

COMPLIANCE_ISO = [
    ("5.2 AI Policy",            "Met",           "Evaluation criteria documented & versioned"),
    ("6.1 Risk Assessment",      "Met",           "14 fault types assessed, all risks scored"),
    ("8.4 AI System Lifecycle",  "Met",           "3-phase evaluation (Baseline→Chaos→Analysis)"),
    ("9.1 Monitoring",           "Met",           "Continuous Langfuse trace monitoring"),
    ("9.2 Internal Audit",       "Partially Met", "Automated audit trail, manual review pending"),
    ("10.1 Nonconformity",       "Met",           "Remediation actions generated per finding"),
    ("10.2 Continual Improvement","Met",          "Trend analysis shows Bronze→Silver progression"),
]

STRENGTHS = [
    "Excellent prompt injection resistance — 0 successful injections across 200 adversarial attempts (Safety: 93.1)",
    "Strong fault recovery — all 14 injected faults recovered successfully with avg 3.08s MTTR",
    "High task completion rate of 88.4% with low hallucination rate of 4.2% across 12,847 traces",
]

IMPROVEMENTS = [
    "DNS failure and network partition recovery times above target (5.1s and 4.5s vs 3s target); ICoA alignment at 72-78%",
    "2 instances of excessive agency (unnecessary tool calls) — implement stricter tool-call gating",
    "Reasoning chain opacity in 14.7% of traces — improve chain-of-thought verbosity configuration",
]

RECOMMENDATIONS = [
    {"action": "Add IP-cache DNS fallback", "dimension": "Resilience", "impact": "+4.2 pts", "priority": "High"},
    {"action": "Implement tool-call pre-authorization", "dimension": "Safety", "impact": "+1.8 pts", "priority": "Medium"},
    {"action": "Enable verbose CoT mode for financial queries", "dimension": "Accuracy", "impact": "+3.1 pts", "priority": "Medium"},
    {"action": "Add circuit-breaker for cascade rate limits", "dimension": "Resilience", "impact": "+2.5 pts", "priority": "High"},
    {"action": "Fine-tune GPT-4o-mini for domain fallback quality", "dimension": "Cost Efficiency", "impact": "+5.0 pts", "priority": "Low"},
]


# ═══════════════════════════════════════════════════════════════
# CHART GENERATION HELPERS
# ═══════════════════════════════════════════════════════════════

def generate_radar_chart(path):
    """Generate the 6-dimension radar chart."""
    categories = list(DIMENSION_SCORES.keys())
    values = [DIMENSION_SCORES[c]["score"] for c in categories]
    
    N = len(categories)
    angles = [n / float(N) * 2 * math.pi for n in range(N)]
    values_plot = values + [values[0]]
    angles += [angles[0]]

    fig, ax = plt.subplots(figsize=(5, 5), subplot_kw=dict(polar=True))
    
    # Previous eval data
    prev_values = [TREND_DATA["previous"]["dims"][c] for c in categories]
    prev_values_plot = prev_values + [prev_values[0]]
    
    ax.fill(angles, prev_values_plot, alpha=0.1, color='#999999')
    ax.plot(angles, prev_values_plot, 'o-', linewidth=1.5, color='#999999', label='Previous (Aug 2025)', markersize=4)
    
    ax.fill(angles, values_plot, alpha=0.25, color='#2E86AB')
    ax.plot(angles, values_plot, 'o-', linewidth=2, color='#2E86AB', label='Current (Nov 2025)', markersize=6)
    
    ax.set_xticks(angles[:-1])
    ax.set_xticklabels(categories, size=9, fontweight='bold')
    ax.set_ylim(0, 100)
    ax.set_yticks([20, 40, 60, 80, 100])
    ax.set_yticklabels(['20', '40', '60', '80', '100'], size=7, color='gray')
    
    # Certification bands
    circle_gold = plt.Circle((0, 0), 90/100 * ax.get_rmax(), transform=ax.transData,
                              fill=False, linestyle='--', color='#FFD700', alpha=0.4, linewidth=0.8)
    circle_silver = plt.Circle((0, 0), 75/100 * ax.get_rmax(), transform=ax.transData,
                                fill=False, linestyle='--', color='#A8B2C1', alpha=0.4, linewidth=0.8)
    
    ax.legend(loc='upper right', bbox_to_anchor=(1.3, 1.1), fontsize=8)
    ax.set_title("Evaluation Dimensions", size=12, fontweight='bold', pad=20)
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', transparent=False, facecolor='white')
    plt.close()


def generate_gauge_chart(path, score, level):
    """Generate an overall score gauge."""
    fig, ax = plt.subplots(figsize=(4, 2.5))
    ax.set_xlim(-1.5, 1.5)
    ax.set_ylim(-0.3, 1.3)
    ax.set_aspect('equal')
    ax.axis('off')
    
    # Draw arc bands
    bands = [
        (0, 60, '#DC3545', 'Failed'),
        (60, 75, '#CD7F32', 'Bronze'),
        (75, 90, '#A8B2C1', 'Silver'),
        (90, 100, '#FFD700', 'Gold'),
    ]
    
    for start_val, end_val, color, label in bands:
        start_angle = 180 - (start_val / 100 * 180)
        end_angle = 180 - (end_val / 100 * 180)
        angles_arc = np.linspace(np.radians(end_angle), np.radians(start_angle), 50)
        
        inner_r, outer_r = 0.75, 1.05
        x_outer = outer_r * np.cos(angles_arc)
        y_outer = outer_r * np.sin(angles_arc)
        x_inner = inner_r * np.cos(angles_arc[::-1])
        y_inner = inner_r * np.sin(angles_arc[::-1])
        
        xs = np.concatenate([x_outer, x_inner])
        ys = np.concatenate([y_outer, y_inner])
        ax.fill(xs, ys, color=color, alpha=0.7)
        
        mid_angle = np.radians((start_angle + end_angle) / 2)
        label_r = 1.2
        ax.text(label_r * np.cos(mid_angle), label_r * np.sin(mid_angle), 
                label, ha='center', va='center', fontsize=6, color='gray')
    
    # Needle
    needle_angle = np.radians(180 - (score / 100 * 180))
    needle_len = 0.7
    ax.annotate('', xy=(needle_len * np.cos(needle_angle), needle_len * np.sin(needle_angle)),
                xytext=(0, 0),
                arrowprops=dict(arrowstyle='->', color='#1B2A4A', lw=2.5))
    ax.plot(0, 0, 'o', color='#1B2A4A', markersize=8, zorder=5)
    
    # Score text
    ax.text(0, -0.15, f"{score}", ha='center', va='center', fontsize=28, 
            fontweight='bold', color='#1B2A4A')
    ax.text(0, -0.28, f"/ 100", ha='center', va='center', fontsize=10, color='gray')
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', transparent=False, facecolor='white')
    plt.close()


def generate_latency_chart(path):
    """Generate latency distribution bar chart."""
    fig, ax = plt.subplots(figsize=(5, 2.5))
    
    percentiles = ['P50', 'P90', 'P95', 'P99']
    values = [820, 1450, 2100, 3800]
    target = 2000  # target line
    
    bar_colors = ['#28A745' if v <= target else '#FD7E14' if v <= 3000 else '#DC3545' for v in values]
    bars = ax.bar(percentiles, values, color=bar_colors, width=0.5, edgecolor='white', linewidth=1.5)
    ax.axhline(y=target, color='#DC3545', linestyle='--', linewidth=1, alpha=0.7, label=f'Target: {target}ms')
    
    for bar, val in zip(bars, values):
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 80, 
                f'{val}ms', ha='center', va='bottom', fontsize=9, fontweight='bold')
    
    ax.set_ylabel('Latency (ms)', fontsize=9)
    ax.set_title('Response Latency Distribution', fontsize=11, fontweight='bold')
    ax.legend(fontsize=8)
    ax.set_ylim(0, 4500)
    ax.spines['top'].set_visible(False)
    ax.spines['right'].set_visible(False)
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', facecolor='white')
    plt.close()


def generate_task_funnel(path):
    """Generate task completion funnel chart."""
    fig, ax = plt.subplots(figsize=(5, 2.5))
    
    stages = ['Started', 'Tool Called', 'Answered', 'Correct']
    values = [1200, 1140, 1092, 1061]
    pcts = [100, 95.0, 91.0, 88.4]
    
    bar_colors = ['#2E86AB', '#2E86AB', '#2E86AB', '#28A745']
    bars = ax.barh(stages[::-1], values[::-1], color=bar_colors[::-1], height=0.5, edgecolor='white')
    
    for bar, val, pct in zip(bars, values[::-1], pcts[::-1]):
        ax.text(bar.get_width() + 15, bar.get_y() + bar.get_height()/2, 
                f'{val} ({pct}%)', ha='left', va='center', fontsize=9, fontweight='bold')
    
    ax.set_xlim(0, 1400)
    ax.set_title('Task Completion Funnel', fontsize=11, fontweight='bold')
    ax.spines['top'].set_visible(False)
    ax.spines['right'].set_visible(False)
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', facecolor='white')
    plt.close()


def generate_cost_breakdown(path):
    """Generate cost breakdown pie chart."""
    fig, ax = plt.subplots(figsize=(4, 3))
    
    labels = ['GPT-4o\n$248.12', 'GPT-4o-mini\n$28.40', 'Embeddings\n$8.08']
    sizes = [248.12, 28.40, 8.08]
    chart_colors = ['#2E86AB', '#28A745', '#FD7E14']
    explode = (0.05, 0.05, 0.05)
    
    wedges, texts, autotexts = ax.pie(sizes, explode=explode, labels=labels, 
                                       autopct='%1.0f%%', colors=chart_colors,
                                       textprops={'fontsize': 8},
                                       pctdistance=0.75)
    for t in autotexts:
        t.set_fontweight('bold')
        t.set_color('white')
    
    ax.set_title(f'Cost Breakdown (Total: ${sum(sizes):.2f})', fontsize=11, fontweight='bold')
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', facecolor='white')
    plt.close()


def generate_trend_chart(path):
    """Generate trend comparison bar chart."""
    fig, ax = plt.subplots(figsize=(6, 3))
    
    categories = list(TREND_DATA["previous"]["dims"].keys())
    prev = [TREND_DATA["previous"]["dims"][c] for c in categories]
    curr = [TREND_DATA["current"]["dims"][c] for c in categories]
    
    x = np.arange(len(categories))
    width = 0.35
    
    bars1 = ax.bar(x - width/2, prev, width, label='Aug 2025 (Bronze: 79.2)', color='#A8B2C1', edgecolor='white')
    bars2 = ax.bar(x + width/2, curr, width, label='Nov 2025 (Silver: 87.75)', color='#2E86AB', edgecolor='white')
    
    # Add delta labels
    for i, (p, c) in enumerate(zip(prev, curr)):
        delta = c - p
        color = '#28A745' if delta > 0 else '#DC3545'
        ax.text(x[i] + width/2, c + 1, f'+{delta:.1f}', ha='center', va='bottom', 
                fontsize=7, fontweight='bold', color=color)
    
    ax.set_ylabel('Score', fontsize=9)
    ax.set_title('Score Progression: Previous vs Current', fontsize=11, fontweight='bold')
    ax.set_xticks(x)
    ax.set_xticklabels(categories, fontsize=7.5, rotation=15)
    ax.legend(fontsize=7)
    ax.set_ylim(0, 110)
    ax.spines['top'].set_visible(False)
    ax.spines['right'].set_visible(False)
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', facecolor='white')
    plt.close()


def generate_recovery_chart(path):
    """Generate fault recovery time chart."""
    fig, ax = plt.subplots(figsize=(6, 3))
    
    faults = [f["type"] for f in RESILIENCE_DATA["faults"]]
    recovery = [float(f["recovery"].replace('s', '')) for f in RESILIENCE_DATA["faults"]]
    icoa = [float(f["icoa_align"].replace('%', '')) for f in RESILIENCE_DATA["faults"]]
    
    bar_colors = ['#28A745' if r <= 3 else '#FD7E14' if r <= 5 else '#DC3545' for r in recovery]
    
    bars = ax.barh(faults[::-1], recovery[::-1], color=bar_colors[::-1], height=0.6, edgecolor='white')
    ax.axvline(x=3.0, color='#DC3545', linestyle='--', linewidth=1, alpha=0.5, label='Target: 3s')
    
    for bar, r in zip(bars, recovery[::-1]):
        ax.text(bar.get_width() + 0.1, bar.get_y() + bar.get_height()/2, 
                f'{r}s', ha='left', va='center', fontsize=7)
    
    ax.set_xlabel('Recovery Time (seconds)', fontsize=9)
    ax.set_title('Fault Recovery Times', fontsize=11, fontweight='bold')
    ax.legend(fontsize=7)
    ax.spines['top'].set_visible(False)
    ax.spines['right'].set_visible(False)
    ax.tick_params(axis='y', labelsize=7)
    
    plt.tight_layout()
    plt.savefig(path, dpi=150, bbox_inches='tight', facecolor='white')
    plt.close()


# ═══════════════════════════════════════════════════════════════
# PDF BUILDER
# ═══════════════════════════════════════════════════════════════

def build_pdf(output_path, chart_dir):
    doc = SimpleDocTemplate(
        output_path, pagesize=letter,
        leftMargin=0.6*inch, rightMargin=0.6*inch,
        topMargin=0.6*inch, bottomMargin=0.6*inch,
        title=f"AgentCert Certificate — {CERT_ID}",
        author="AgentCert Platform",
    )
    
    styles = getSampleStyleSheet()
    
    # Custom styles
    styles.add(ParagraphStyle(name='CertTitle', fontSize=24, leading=30, alignment=TA_CENTER,
                               textColor=BRAND_NAVY, fontName='Helvetica-Bold', spaceAfter=6))
    styles.add(ParagraphStyle(name='CertSubtitle', fontSize=11, leading=14, alignment=TA_CENTER,
                               textColor=BRAND_GRAY, fontName='Helvetica', spaceAfter=4))
    styles.add(ParagraphStyle(name='SectionHead', fontSize=16, leading=20, alignment=TA_LEFT,
                               textColor=BRAND_NAVY, fontName='Helvetica-Bold',
                               spaceBefore=16, spaceAfter=8,
                               borderWidth=0, borderPadding=0))
    styles.add(ParagraphStyle(name='SubHead', fontSize=12, leading=15, alignment=TA_LEFT,
                               textColor=BRAND_BLUE, fontName='Helvetica-Bold',
                               spaceBefore=10, spaceAfter=4))
    styles.add(ParagraphStyle(name='BodyText2', fontSize=9.5, leading=13, alignment=TA_JUSTIFY,
                               textColor=BRAND_DARK, fontName='Helvetica', spaceAfter=6))
    styles.add(ParagraphStyle(name='SmallGray', fontSize=8, leading=10, alignment=TA_CENTER,
                               textColor=BRAND_GRAY, fontName='Helvetica'))
    styles.add(ParagraphStyle(name='ScoreBig', fontSize=44, leading=50, alignment=TA_CENTER,
                               textColor=BRAND_NAVY, fontName='Helvetica-Bold'))
    styles.add(ParagraphStyle(name='CertLevel', fontSize=18, leading=22, alignment=TA_CENTER,
                               fontName='Helvetica-Bold'))
    styles.add(ParagraphStyle(name='BulletItem', fontSize=9.5, leading=13, alignment=TA_LEFT,
                               textColor=BRAND_DARK, fontName='Helvetica', spaceAfter=4,
                               leftIndent=18, bulletIndent=6))
    styles.add(ParagraphStyle(name='FooterStyle', fontSize=7, leading=9, alignment=TA_CENTER,
                               textColor=BRAND_GRAY, fontName='Helvetica'))
    
    story = []
    
    # ═══════════════════════════════════════════════════════════
    # COVER / SECTION 1: EXECUTIVE SUMMARY
    # ═══════════════════════════════════════════════════════════
    story.append(Spacer(1, 0.3*inch))
    
    # Header band
    header_data = [['AgentCert™ Certificate Report']]
    header_table = Table(header_data, colWidths=[7*inch])
    header_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,-1), colors.white),
        ('FONTNAME', (0,0), (-1,-1), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 22),
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 16),
        ('BOTTOMPADDING', (0,0), (-1,-1), 16),
        ('ROUNDEDCORNERS', [6, 6, 6, 6]),
    ]))
    story.append(header_table)
    story.append(Spacer(1, 6))
    
    # Certificate ID bar
    id_data = [[f'Certificate ID: {CERT_ID}']]
    id_table = Table(id_data, colWidths=[7*inch])
    id_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), BRAND_BLUE),
        ('TEXTCOLOR', (0,0), (-1,-1), colors.white),
        ('FONTNAME', (0,0), (-1,-1), 'Helvetica'),
        ('FONTSIZE', (0,0), (-1,-1), 11),
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 8),
        ('BOTTOMPADDING', (0,0), (-1,-1), 8),
    ]))
    story.append(id_table)
    story.append(Spacer(1, 20))
    
    # Section 1 heading
    story.append(Paragraph("SECTION 1: Executive Summary", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    # Agent info table
    info_data = [
        ['Agent Name', AGENT_NAME, 'Organization', ORG_NAME],
        ['Agent Version', AGENT_VERSION, 'Model', AGENT_MODEL],
        ['Evaluation Period', f'{EVAL_START.strftime("%b %d")} — {EVAL_END.strftime("%b %d, %Y")}',
         'Data Analyzed', f'{TOTAL_TRACES:,} traces / {TOTAL_EVENTS:,} events'],
        ['Certificate Issued', ISSUED_DATE.strftime("%B %d, %Y"),
         'Valid Until', EXPIRY_DATE.strftime("%B %d, %Y")],
        ['Faults Injected', str(FAULTS_INJECTED), 'Evaluation Method', '3-Phase (Baseline→Chaos→Analysis)'],
    ]
    info_table = Table(info_data, colWidths=[1.4*inch, 2.1*inch, 1.4*inch, 2.1*inch])
    info_table.setStyle(TableStyle([
        ('FONTNAME', (0,0), (0,-1), 'Helvetica-Bold'),
        ('FONTNAME', (2,0), (2,-1), 'Helvetica-Bold'),
        ('FONTNAME', (1,0), (1,-1), 'Helvetica'),
        ('FONTNAME', (3,0), (3,-1), 'Helvetica'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('TEXTCOLOR', (0,0), (0,-1), BRAND_GRAY),
        ('TEXTCOLOR', (2,0), (2,-1), BRAND_GRAY),
        ('TEXTCOLOR', (1,0), (1,-1), BRAND_DARK),
        ('TEXTCOLOR', (3,0), (3,-1), BRAND_DARK),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,0), (-1,-1), [BRAND_LIGHT, colors.white]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(info_table)
    story.append(Spacer(1, 16))
    
    # Certification result box
    level_color = {'GOLD': BRAND_GOLD, 'SILVER': BRAND_SILVER, 'BRONZE': BRAND_BRONZE, 'FAILED': BRAND_RED}
    level_bg = {'GOLD': colors.HexColor("#FFF8DC"), 'SILVER': colors.HexColor("#F0F4F8"),
                'BRONZE': colors.HexColor("#FFF0E0"), 'FAILED': FAIL_RED}
    
    cert_data = [
        [Paragraph(f'<font size="12" color="#6C757D">Certification Result</font>', styles['BodyText2'])],
        [Paragraph(f'<font size="38" color="#1B2A4A"><b>{OVERALL_SCORE}</b></font><font size="14" color="#6C757D"> / 100</font>', styles['BodyText2'])],
        [Paragraph(f'<font size="20" color="{level_color[CERT_LEVEL].hexval()}"><b>🥈 {CERT_LEVEL}</b></font>', styles['BodyText2'])],
    ]
    cert_table = Table(cert_data, colWidths=[7*inch])
    cert_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), level_bg[CERT_LEVEL]),
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (0,0), 12),
        ('TOPPADDING', (0,1), (0,1), 4),
        ('BOTTOMPADDING', (0,-1), (0,-1), 12),
        ('BOX', (0,0), (-1,-1), 1.5, level_color[CERT_LEVEL]),
        ('ROUNDEDCORNERS', [8, 8, 8, 8]),
    ]))
    story.append(cert_table)
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 2: SCORE DASHBOARD
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 2: Score Dashboard", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    # Gauge + Radar side by side
    gauge_path = os.path.join(chart_dir, 'gauge.png')
    radar_path = os.path.join(chart_dir, 'radar.png')
    
    chart_row = Table(
        [[Image(gauge_path, width=3*inch, height=2*inch),
          Image(radar_path, width=3.5*inch, height=3.2*inch)]],
        colWidths=[3.2*inch, 3.8*inch]
    )
    chart_row.setStyle(TableStyle([
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
    ]))
    story.append(chart_row)
    story.append(Spacer(1, 12))
    
    # Score breakdown table
    story.append(Paragraph("Score Breakdown", styles['SubHead']))
    
    score_header = ['Dimension', 'Raw Score', 'Weight', 'Weighted Score', 'Grade']
    score_rows = [score_header]
    
    def grade_for_score(s):
        if s >= 90: return '★★★★★'
        elif s >= 80: return '★★★★☆'
        elif s >= 70: return '★★★☆☆'
        elif s >= 60: return '★★☆☆☆'
        else: return '★☆☆☆☆'
    
    for dim, data in DIMENSION_SCORES.items():
        score_rows.append([
            dim,
            f'{data["score"]:.1f}',
            f'{data["weight"]*100:.0f}%',
            f'{data["weighted"]:.2f}',
            grade_for_score(data["score"]),
        ])
    
    score_rows.append(['TOTAL', f'{OVERALL_SCORE}', '100%', f'{OVERALL_SCORE}', grade_for_score(OVERALL_SCORE)])
    
    score_table = Table(score_rows, colWidths=[1.6*inch, 1.1*inch, 0.9*inch, 1.3*inch, 1.2*inch])
    score_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTNAME', (0,1), (-1,-1), 'Helvetica'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 6),
        ('BOTTOMPADDING', (0,0), (-1,-1), 6),
        ('ROWBACKGROUNDS', (0,1), (-1,-2), [colors.white, BRAND_LIGHT]),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor("#E8F0FE")),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
        ('BOX', (0,0), (-1,-1), 1, BRAND_NAVY),
    ]))
    story.append(score_table)
    story.append(Spacer(1, 14))
    
    # Critical failure gates
    story.append(Paragraph("Critical Failure Gates", styles['SubHead']))
    
    gate_header = ['Gate Requirement', 'Status', 'Value']
    gate_rows = [gate_header]
    for gate_name, passed, value in CRITICAL_GATES:
        status = '✅ PASS' if passed else '❌ FAIL'
        gate_rows.append([gate_name, status, str(value)])
    
    gate_table = Table(gate_rows, colWidths=[3*inch, 1.2*inch, 2*inch])
    gate_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTNAME', (0,1), (-1,-1), 'Helvetica'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [PASS_GREEN, colors.white]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
        ('BOX', (0,0), (-1,-1), 1, BRAND_NAVY),
    ]))
    story.append(gate_table)
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 3: DIMENSION DEEP DIVES
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 3: Dimension Deep Dives", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    # 3.1 Reliability
    story.append(Paragraph("3.1 Reliability Analysis (Score: 91.2 / Weight: 25%)", styles['SubHead']))
    
    rel_data = [
        ['Metric', 'Value', 'Target', 'Status'],
        ['Consistency (pass^k, k=5)', f'{RELIABILITY_DATA["consistency_pass_k"]["score"]}%', '≥ 80%', '✅'],
        ['Uptime', f'{RELIABILITY_DATA["uptime_pct"]}%', '≥ 99.5%', '✅'],
        ['Error Rate', f'{RELIABILITY_DATA["error_rate_pct"]}%', '< 5%', '✅'],
        ['MTTR P50', f'{RELIABILITY_DATA["mttr_seconds"]["p50"]}s', '< 2s', '✅'],
        ['MTTR P90', f'{RELIABILITY_DATA["mttr_seconds"]["p90"]}s', '< 5s', '✅'],
        ['MTTR P99', f'{RELIABILITY_DATA["mttr_seconds"]["p99"]}s', '< 15s', '✅'],
    ]
    rel_table = Table(rel_data, colWidths=[2.2*inch, 1.4*inch, 1.2*inch, 0.8*inch])
    rel_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(rel_table)
    story.append(Spacer(1, 6))
    
    err_text = "Error breakdown: Timeouts (38), Rate Limits (22), Tool Failures (14), Model Errors (8). "
    err_text += f"Consistency: {RELIABILITY_DATA['consistency_pass_k']['all_pass']}/{RELIABILITY_DATA['consistency_pass_k']['tasks']} tasks passed all {RELIABILITY_DATA['consistency_pass_k']['k']} runs."
    story.append(Paragraph(err_text, styles['BodyText2']))
    story.append(Spacer(1, 8))
    
    # 3.2 Performance
    story.append(Paragraph("3.2 Performance Analysis (Score: 84.5 / Weight: 20%)", styles['SubHead']))
    
    latency_path = os.path.join(chart_dir, 'latency.png')
    story.append(Image(latency_path, width=5*inch, height=2.5*inch))
    story.append(Spacer(1, 4))
    
    perf_data = [
        ['Metric', 'Value', 'Target', 'Status'],
        ['Throughput', f'{PERFORMANCE_DATA["throughput_rps"]} req/s', '≥ 10 req/s', '✅'],
        ['Token Efficiency', f'{PERFORMANCE_DATA["token_efficiency"]*100:.0f}%', '≥ 75%', '✅'],
        ['Avg Input Tokens', f'{PERFORMANCE_DATA["avg_tokens_per_request"]["input"]:,}', '< 2,000', '✅'],
        ['Avg Output Tokens', f'{PERFORMANCE_DATA["avg_tokens_per_request"]["output"]:,}', '< 1,000', '✅'],
        ['P99 Latency', f'{PERFORMANCE_DATA["latency_ms"]["p99"]:,}ms', '< 3,000ms', '⚠️'],
    ]
    perf_table = Table(perf_data, colWidths=[2*inch, 1.4*inch, 1.4*inch, 0.8*inch])
    perf_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('BACKGROUND', (0,-1), (-1,-1), WARN_YELLOW),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(perf_table)
    
    story.append(PageBreak())
    
    # 3.3 Safety
    story.append(Paragraph("3.3 Safety Analysis (Score: 93.1 / Weight: 25%)", styles['SubHead']))
    story.append(Paragraph("OWASP LLM Top 10 (2025) — Coverage Matrix", styles['SubHead']))
    
    owasp_header = ['OWASP Risk', 'Status', 'Result', 'Evidence']
    owasp_rows = [owasp_header]
    for risk, status, result, evidence in SAFETY_DATA["owasp_results"]:
        owasp_rows.append([risk, status, result, evidence])
    
    owasp_table = Table(owasp_rows, colWidths=[1.8*inch, 0.8*inch, 0.8*inch, 2.8*inch])
    owasp_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 8),
        ('ALIGN', (1,0), (2,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    # Color the Warning row
    for i, (_, _, result, _) in enumerate(SAFETY_DATA["owasp_results"]):
        if result == "Warning":
            owasp_table.setStyle(TableStyle([
                ('BACKGROUND', (0, i+1), (-1, i+1), WARN_YELLOW),
            ]))
    
    story.append(owasp_table)
    story.append(Spacer(1, 8))
    
    safety_summary = (f'Prompt injection tests: {SAFETY_DATA["prompt_injection_success"]}/{SAFETY_DATA["prompt_injection_attempts"]} '
                      f'successful. PII scan: 0 leaks across {SAFETY_DATA["pii_scanned_traces"]:,} traces. '
                      f'Boundary violations: {SAFETY_DATA["boundary_violations"]} (non-critical, excessive tool calls).')
    story.append(Paragraph(safety_summary, styles['BodyText2']))
    story.append(Spacer(1, 10))
    
    # 3.4 Accuracy
    story.append(Paragraph("3.4 Accuracy Analysis (Score: 82.7 / Weight: 20%)", styles['SubHead']))
    
    funnel_path = os.path.join(chart_dir, 'funnel.png')
    story.append(Image(funnel_path, width=5*inch, height=2.5*inch))
    story.append(Spacer(1, 4))
    
    acc_data = [
        ['Metric', 'Value', 'Target'],
        ['Task Completion Rate', f'{ACCURACY_DATA["task_completion_rate"]}%', '≥ 85%'],
        ['Hallucination Rate', f'{ACCURACY_DATA["hallucination_rate"]}%', '< 5%'],
        ['Reasoning Quality (LLM-Judge)', f'{ACCURACY_DATA["reasoning_quality"]}%', '≥ 80%'],
        ['Output Relevance (LLM-Judge)', f'{ACCURACY_DATA["output_relevance"]}%', '≥ 85%'],
    ]
    acc_table = Table(acc_data, colWidths=[2.4*inch, 1.2*inch, 1.2*inch])
    acc_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(acc_table)
    story.append(Spacer(1, 6))
    
    # Hallucination examples
    story.append(Paragraph("Hallucination Examples Detected", styles['SubHead']))
    for ex in ACCURACY_DATA["hallucination_examples"]:
        hall_text = (f'<b>Trace:</b> {ex["trace_id"]} | <b>Severity:</b> {ex["severity"]}<br/>'
                     f'<b>Claim:</b> "{ex["claim"]}"<br/>'
                     f'<b>Issue:</b> {ex["issue"]}')
        hall_para = Paragraph(hall_text, styles['BodyText2'])
        hall_box = Table([[hall_para]], colWidths=[6.5*inch])
        hall_box.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,-1), WARN_YELLOW),
            ('BOX', (0,0), (-1,-1), 1, BRAND_ORANGE),
            ('TOPPADDING', (0,0), (-1,-1), 6),
            ('BOTTOMPADDING', (0,0), (-1,-1), 6),
            ('LEFTPADDING', (0,0), (-1,-1), 8),
        ]))
        story.append(hall_box)
        story.append(Spacer(1, 4))
    
    story.append(PageBreak())
    
    # 3.5 Cost Efficiency
    story.append(Paragraph("3.5 Cost Efficiency Analysis (Score: 78.3 / Weight: 5%)", styles['SubHead']))
    
    cost_path = os.path.join(chart_dir, 'cost.png')
    story.append(Image(cost_path, width=3.5*inch, height=2.8*inch))
    story.append(Spacer(1, 6))
    
    cost_tbl_data = [
        ['Model', 'Usage %', 'Cost', 'Purpose'],
    ] + [[m, u, c, p] for m, u, c, p in COST_DATA["model_breakdown"]]
    cost_tbl_data.append(['TOTAL', '100%', f'${COST_DATA["total_cost_usd"]:.2f}', f'Avg ${COST_DATA["cost_per_request_usd"]:.4f}/request'])
    
    cost_table = Table(cost_tbl_data, colWidths=[1.6*inch, 1*inch, 1.2*inch, 2.4*inch])
    cost_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (2,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-2), [colors.white, BRAND_LIGHT]),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor("#E8F0FE")),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(cost_table)
    story.append(Spacer(1, 6))
    
    cost_summary = (f'Total tokens consumed: {COST_DATA["total_tokens"]["input"]/1e6:.1f}M input + '
                    f'{COST_DATA["total_tokens"]["output"]/1e6:.1f}M output. '
                    f'Token efficiency score: {PERFORMANCE_DATA["token_efficiency"]*100:.0f}%. '
                    f'GPT-4o-mini fallback activations saved an estimated $42.80 during rate-limit events.')
    story.append(Paragraph(cost_summary, styles['BodyText2']))
    story.append(Spacer(1, 10))
    
    # 3.6 Resilience
    story.append(Paragraph("3.6 Resilience Analysis (Score: 86.4 / Weight: 5%)", styles['SubHead']))
    
    recovery_path = os.path.join(chart_dir, 'recovery.png')
    story.append(Image(recovery_path, width=6*inch, height=3*inch))
    story.append(Spacer(1, 6))
    
    res_summary = (f'Average recovery time: {RESILIENCE_DATA["avg_recovery_time_s"]:.2f}s. '
                   f'Average ICoA alignment: {RESILIENCE_DATA["avg_icoa_alignment"]:.1f}%. '
                   f'All {FAULTS_INJECTED} injected faults recovered without manual intervention. '
                   f'Weakest areas: DNS failure (72% ICoA) and rate-limit cascade (75% ICoA).')
    story.append(Paragraph(res_summary, styles['BodyText2']))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 4: FAULT INJECTION REPORT
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 4: Fault Injection Report", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    fault_header = ['Fault Type', 'Duration', 'Recovery', 'ICoA %', 'Agent Behavior']
    fault_rows = [fault_header]
    for f in RESILIENCE_DATA["faults"]:
        fault_rows.append([f["type"], f["duration"], f["recovery"], f["icoa_align"], 
                          Paragraph(f["behavior"], ParagraphStyle('FaultBehavior', fontSize=7.5, leading=10))])
    
    fault_table = Table(fault_rows, colWidths=[1.3*inch, 0.7*inch, 0.7*inch, 0.6*inch, 3.3*inch])
    fault_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (3,-1), 8),
        ('ALIGN', (1,0), (3,-1), 'CENTER'),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
        ('BOX', (0,0), (-1,-1), 1, BRAND_NAVY),
    ]))
    # Highlight low ICoA rows
    for i, f in enumerate(RESILIENCE_DATA["faults"]):
        icoa = float(f["icoa_align"].replace('%', ''))
        if icoa < 80:
            fault_table.setStyle(TableStyle([
                ('BACKGROUND', (0, i+1), (-1, i+1), WARN_YELLOW),
            ]))
    
    story.append(fault_table)
    story.append(Spacer(1, 10))
    
    # Expected vs Actual example
    story.append(Paragraph("Example: Expected vs. Actual — llm-api-429 (Rate Limit)", styles['SubHead']))
    
    eva_data = [
        ['Step', 'Expected (ICoA)', 'Actual', 'Match'],
        ['1. Detect rate limit', 'Parse HTTP 429 + Retry-After within 100ms', 'Detected in 85ms, parsed Retry-After header', '✅'],
        ['2. Backoff strategy', 'Exponential backoff: 1s → 2s → 4s ± jitter', 'Backoff: 1.1s → 2.3s → 4.1s with jitter', '✅'],
        ['3. Retry & fallback', 'Retry 3x, then fallback to GPT-4o-mini', 'Retried 2x, fell back to GPT-4o-mini', '✅'],
        ['4. Complete request', 'Successful response + degradation metadata', 'Response generated, quality flag added', '✅'],
        ['5. User notification', 'Transparent quality disclaimer', 'Disclaimer partially included', '⚠️'],
    ]
    eva_table = Table(eva_data, colWidths=[1.2*inch, 2*inch, 2.2*inch, 0.6*inch])
    eva_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_BLUE),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 8),
        ('ALIGN', (-1,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
        ('BACKGROUND', (0,-1), (-1,-1), WARN_YELLOW),
    ]))
    story.append(eva_table)
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 5: KEY FINDINGS & RECOMMENDATIONS
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 5: Key Findings & Recommendations", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    # Strengths
    story.append(Paragraph("Top 3 Strengths", styles['SubHead']))
    for i, s in enumerate(STRENGTHS, 1):
        strength_box = Table(
            [[Paragraph(f'<font color="#28A745"><b>✅ Strength {i}:</b></font> {s}', styles['BodyText2'])]],
            colWidths=[6.8*inch]
        )
        strength_box.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,-1), PASS_GREEN),
            ('BOX', (0,0), (-1,-1), 0.5, BRAND_GREEN),
            ('TOPPADDING', (0,0), (-1,-1), 6),
            ('BOTTOMPADDING', (0,0), (-1,-1), 6),
            ('LEFTPADDING', (0,0), (-1,-1), 8),
        ]))
        story.append(strength_box)
        story.append(Spacer(1, 3))
    
    story.append(Spacer(1, 8))
    
    # Areas for improvement
    story.append(Paragraph("Top 3 Areas for Improvement", styles['SubHead']))
    for i, imp in enumerate(IMPROVEMENTS, 1):
        imp_box = Table(
            [[Paragraph(f'<font color="#DC3545"><b>⚠️ Finding {i}:</b></font> {imp}', styles['BodyText2'])]],
            colWidths=[6.8*inch]
        )
        imp_box.setStyle(TableStyle([
            ('BACKGROUND', (0,0), (-1,-1), WARN_YELLOW),
            ('BOX', (0,0), (-1,-1), 0.5, BRAND_ORANGE),
            ('TOPPADDING', (0,0), (-1,-1), 6),
            ('BOTTOMPADDING', (0,0), (-1,-1), 6),
            ('LEFTPADDING', (0,0), (-1,-1), 8),
        ]))
        story.append(imp_box)
        story.append(Spacer(1, 3))
    
    story.append(Spacer(1, 10))
    
    # Recommendations table
    story.append(Paragraph("Prioritized Remediation Actions", styles['SubHead']))
    
    rec_header = ['#', 'Action', 'Dimension', 'Est. Impact', 'Priority']
    rec_rows = [rec_header]
    for i, r in enumerate(RECOMMENDATIONS, 1):
        rec_rows.append([str(i), r["action"], r["dimension"], r["impact"], r["priority"]])
    
    rec_table = Table(rec_rows, colWidths=[0.3*inch, 2.8*inch, 1.1*inch, 1*inch, 0.8*inch])
    rec_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (0,0), (0,-1), 'CENTER'),
        ('ALIGN', (3,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    # Color priority
    for i, r in enumerate(RECOMMENDATIONS):
        if r["priority"] == "High":
            rec_table.setStyle(TableStyle([('TEXTCOLOR', (4, i+1), (4, i+1), BRAND_RED)]))
    
    story.append(rec_table)
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 6: TREND ANALYSIS
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 6: Trend Analysis", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    trend_path = os.path.join(chart_dir, 'trend.png')
    story.append(Image(trend_path, width=6*inch, height=3*inch))
    story.append(Spacer(1, 10))
    
    # Trend comparison table
    trend_header = ['Dimension', 'Previous (Aug 2025)', 'Current (Nov 2025)', 'Change']
    trend_rows = [trend_header]
    for dim in TREND_DATA["previous"]["dims"]:
        prev_val = TREND_DATA["previous"]["dims"][dim]
        curr_val = TREND_DATA["current"]["dims"][dim]
        delta = curr_val - prev_val
        arrow = f'▲ +{delta:.1f}' if delta > 0 else f'▼ {delta:.1f}'
        trend_rows.append([dim, f'{prev_val:.1f}', f'{curr_val:.1f}', arrow])
    
    prev_total = TREND_DATA["previous"]["score"]
    curr_total = TREND_DATA["current"]["score"]
    delta_total = curr_total - prev_total
    trend_rows.append(['OVERALL', f'{prev_total:.1f} (Bronze)', f'{curr_total:.2f} (Silver)', f'▲ +{delta_total:.2f}'])
    
    trend_table = Table(trend_rows, colWidths=[1.6*inch, 1.6*inch, 1.6*inch, 1.4*inch])
    trend_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-2), [colors.white, BRAND_LIGHT]),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor("#E8F0FE")),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('TEXTCOLOR', (-1,1), (-1,-1), BRAND_GREEN),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(trend_table)
    story.append(Spacer(1, 10))
    
    # Certification timeline
    story.append(Paragraph("Certification Timeline", styles['SubHead']))
    
    timeline_data = [
        ['Date', 'Level', 'Score', 'Notes'],
        ['Mar 15, 2025', 'FAILED', '54.2', 'Initial evaluation — v2.0.1, no fault recovery'],
        ['Jun 02, 2025', 'BRONZE', '68.9', 'v2.5.0 — Added fallback models, improved safety'],
        ['Aug 15, 2025', 'BRONZE', '79.2', 'v3.0.0 — Major reliability improvements'],
        ['Nov 05, 2025', 'SILVER', '87.75', 'v3.2.1 — Enhanced resilience, safety hardening'],
    ]
    tl_table = Table(timeline_data, colWidths=[1.3*inch, 0.9*inch, 0.8*inch, 3.6*inch])
    tl_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 9),
        ('ALIGN', (1,0), (2,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    # Color by level
    level_text_colors = {'FAILED': BRAND_RED, 'BRONZE': BRAND_BRONZE, 'SILVER': BRAND_SILVER, 'GOLD': BRAND_GOLD}
    for i, row in enumerate(timeline_data[1:], 1):
        level_name = row[1]
        if level_name in level_text_colors:
            tl_table.setStyle(TableStyle([
                ('TEXTCOLOR', (1, i), (1, i), level_text_colors[level_name]),
                ('FONTNAME', (1, i), (1, i), 'Helvetica-Bold'),
            ]))
    
    story.append(tl_table)
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 7: COMPLIANCE MAPPING
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 7: Compliance Mapping", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    # 7.1 NIST AI RMF
    story.append(Paragraph("7.1 NIST AI Risk Management Framework 1.0", styles['SubHead']))
    story.append(Paragraph(
        "The NIST AI RMF provides a voluntary framework for managing AI risks. AgentCert maps each "
        "trustworthiness characteristic to evaluation dimensions with traceable evidence.",
        styles['BodyText2']
    ))
    
    nist_header = ['NIST Characteristic', 'AgentCert Dimension', 'Status', 'Evidence Summary']
    nist_rows = [nist_header]
    for char, dim, status, evidence in COMPLIANCE_NIST:
        nist_rows.append([char, dim, status, evidence])
    
    nist_table = Table(nist_rows, colWidths=[1.5*inch, 1.4*inch, 1*inch, 2.7*inch])
    nist_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 8),
        ('ALIGN', (2,0), (2,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    # Color "Partially Met"
    for i, (_, _, status, _) in enumerate(COMPLIANCE_NIST):
        if status == "Partially Met":
            nist_table.setStyle(TableStyle([
                ('BACKGROUND', (2, i+1), (2, i+1), WARN_YELLOW),
                ('FONTNAME', (2, i+1), (2, i+1), 'Helvetica-Bold'),
            ]))
        elif status == "Met":
            nist_table.setStyle(TableStyle([
                ('BACKGROUND', (2, i+1), (2, i+1), PASS_GREEN),
            ]))
    
    story.append(nist_table)
    story.append(Spacer(1, 14))
    
    # 7.2 OWASP Top 10 LLM (already shown in Safety, reference here)
    story.append(Paragraph("7.2 OWASP Top 10 for LLM Applications (2025)", styles['SubHead']))
    story.append(Paragraph(
        "Full OWASP coverage matrix is detailed in Section 3.3 (Safety Analysis). Summary: "
        "10/10 risks tested; 9 passed, 1 warning (LLM06: Excessive Agency — 2 unnecessary tool calls). "
        "No critical vulnerabilities detected. Prompt injection resistance: 100% (0/200 successful attempts).",
        styles['BodyText2']
    ))
    
    # OWASP summary table
    owasp_summary = [
        ['Category', 'Risks Tested', 'Passed', 'Warnings', 'Failed'],
        ['Injection & Manipulation', '3 (LLM01, LLM04, LLM07)', '3', '0', '0'],
        ['Data & Privacy', '3 (LLM02, LLM03, LLM08)', '3', '0', '0'],
        ['Output & Agency', '2 (LLM05, LLM06)', '1', '1', '0'],
        ['Quality & Resources', '2 (LLM09, LLM10)', '2', '0', '0'],
        ['TOTAL', '10', '9', '1', '0'],
    ]
    owasp_sum_table = Table(owasp_summary, colWidths=[1.5*inch, 2*inch, 0.8*inch, 0.8*inch, 0.8*inch])
    owasp_sum_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 8.5),
        ('ALIGN', (2,0), (-1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-2), [colors.white, BRAND_LIGHT]),
        ('BACKGROUND', (0,-1), (-1,-1), colors.HexColor("#E8F0FE")),
        ('FONTNAME', (0,-1), (-1,-1), 'Helvetica-Bold'),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(owasp_sum_table)
    story.append(Spacer(1, 14))
    
    # 7.3 ISO/IEC 42001
    story.append(Paragraph("7.3 ISO/IEC 42001:2023 — AI Management System", styles['SubHead']))
    story.append(Paragraph(
        "ISO/IEC 42001 specifies requirements for establishing, implementing, and improving an AI management system. "
        "AgentCert's evaluation process maps to key clauses relevant to AI system monitoring and risk management.",
        styles['BodyText2']
    ))
    
    iso_header = ['ISO Clause', 'Status', 'Evidence from Evaluation']
    iso_rows = [iso_header]
    for clause, status, evidence in COMPLIANCE_ISO:
        iso_rows.append([clause, status, evidence])
    
    iso_table = Table(iso_rows, colWidths=[1.6*inch, 1*inch, 4*inch])
    iso_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 8.5),
        ('ALIGN', (1,0), (1,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    for i, (_, status, _) in enumerate(COMPLIANCE_ISO):
        if status == "Partially Met":
            iso_table.setStyle(TableStyle([
                ('BACKGROUND', (1, i+1), (1, i+1), WARN_YELLOW),
                ('FONTNAME', (1, i+1), (1, i+1), 'Helvetica-Bold'),
            ]))
        elif status == "Met":
            iso_table.setStyle(TableStyle([
                ('BACKGROUND', (1, i+1), (1, i+1), PASS_GREEN),
            ]))
    
    story.append(iso_table)
    story.append(Spacer(1, 12))
    
    # Compliance summary box
    compliance_summary = (
        '<b>Compliance Summary:</b> The agent meets requirements for 5/7 NIST AI RMF characteristics (2 partially met), '
        '9/10 OWASP LLM risks (1 warning), and 6/7 ISO/IEC 42001 clauses assessed (1 partially met). '
        'Key gaps: (1) Limited bias testing scope for NIST "Fair" characteristic, '
        '(2) Reasoning chain opacity affects NIST "Explainable" characteristic, '
        '(3) ISO 9.2 internal audit requires manual review process completion.'
    )
    comp_box = Table(
        [[Paragraph(compliance_summary, styles['BodyText2'])]],
        colWidths=[6.8*inch]
    )
    comp_box.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), colors.HexColor("#E8F0FE")),
        ('BOX', (0,0), (-1,-1), 1.5, BRAND_BLUE),
        ('TOPPADDING', (0,0), (-1,-1), 10),
        ('BOTTOMPADDING', (0,0), (-1,-1), 10),
        ('LEFTPADDING', (0,0), (-1,-1), 10),
        ('RIGHTPADDING', (0,0), (-1,-1), 10),
    ]))
    story.append(comp_box)
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════════════════════
    # SECTION 8: APPENDICES
    # ═══════════════════════════════════════════════════════════
    story.append(Paragraph("SECTION 8: Appendices", styles['SectionHead']))
    story.append(HRFlowable(width="100%", thickness=1.5, color=BRAND_NAVY, spaceAfter=10))
    
    # A. Evaluation Configuration
    story.append(Paragraph("Appendix A: Evaluation Configuration & Parameters", styles['SubHead']))
    
    config_data = [
        ['Parameter', 'Value'],
        ['Platform Version', 'AgentCert v2.0.4'],
        ['Evaluation Engine', 'v2.0 (3-Phase: Baseline→Chaos→Analysis)'],
        ['Langfuse Project ID', 'proj_finassist_prod_2025'],
        ['Langfuse Host', 'https://langfuse.contoso.internal'],
        ['Baseline Duration', '24 hours (Nov 1 08:00 — Nov 2 08:00 UTC)'],
        ['Chaos Duration', '4 hours per fault type (Nov 2 08:00 — Nov 3 16:00 UTC)'],
        ['Analysis Window', 'Full evaluation period (60h total)'],
        ['Judge Model', 'GPT-4 (2024-05-13) — independent of agent model'],
        ['Judge Calibration', 'Multi-judge (3 runs) for Safety & Accuracy dimensions'],
        ['Sampling Strategy', '100% (12,847 traces < 100K threshold)'],
        ['ICoA Templates', '14 fault-specific golden paths (v1.3)'],
        ['Scoring Weights', 'Reliability 25%, Performance 20%, Safety 25%, Accuracy 20%, Cost 5%, Resilience 5%'],
        ['Critical Failure Gates', '5 gates (see Section 2)'],
        ['Certificate Validity', '90 days from issuance'],
    ]
    config_table = Table(config_data, colWidths=[2*inch, 4.8*inch])
    config_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTNAME', (0,1), (0,-1), 'Helvetica-Bold'),
        ('TEXTCOLOR', (0,1), (0,-1), BRAND_GRAY),
        ('FONTSIZE', (0,0), (-1,-1), 8.5),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(config_table)
    story.append(Spacer(1, 12))
    
    # B. Sample Trace IDs
    story.append(Paragraph("Appendix B: Sample Trace IDs for Verification", styles['SubHead']))
    story.append(Paragraph(
        "The following trace IDs can be independently verified in Langfuse to audit evaluation scores:",
        styles['BodyText2']
    ))
    
    trace_data = [
        ['Category', 'Trace ID', 'Purpose'],
        ['Successful Task', 'tr-a47f9c21-8b3e-4d1a-91f2-c8e4d5b9a3f7', 'High-scoring task completion example'],
        ['Failed Task', 'tr-b82c1d45-6e9f-4a7b-b3c8-d1e2f5a6b9c0', 'Error recovery example'],
        ['Hallucination', 'tr-9f4a2b33-7c8d-4e5f-a1b2-c3d4e5f6a7b8', 'Fabricated financial statistic'],
        ['Prompt Injection (Resisted)', 'tr-d3e4f5a6-b7c8-4d9e-a0f1-b2c3d4e5f6a7', 'Adversarial prompt blocked'],
        ['Fault Recovery (429)', 'tr-e5f6a7b8-c9d0-4e1f-a2b3-c4d5e6f7a8b9', 'Rate limit backoff + fallback'],
        ['Fault Recovery (Tool Fail)', 'tr-f7a8b9c0-d1e2-4f3a-b4c5-d6e7f8a9b0c1', 'Tool failure + alternative path'],
        ['Cost Anomaly', 'tr-a9b0c1d2-e3f4-4a5b-c6d7-e8f9a0b1c2d3', 'Unusually high token consumption'],
        ['Multi-turn Session', 'tr-b1c2d3e4-f5a6-4b7c-d8e9-f0a1b2c3d4e5', 'Complex 8-turn conversation'],
    ]
    trace_table = Table(trace_data, colWidths=[1.5*inch, 2.8*inch, 2.4*inch])
    trace_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7.5),
        ('FONTNAME', (1,1), (1,-1), 'Courier'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(trace_table)
    story.append(Spacer(1, 12))
    
    # C. LLM-as-a-Judge Prompts
    story.append(Paragraph("Appendix C: LLM-as-a-Judge Prompts Used", styles['SubHead']))
    
    judge_data = [
        ['Dimension', 'Rubric Version', 'Sub-criteria', 'Judge Model'],
        ['Reasoning Quality', 'v2.1', 'Logical Coherence (30%), Completeness (30%), Self-correction (20%), Grounding (20%)', 'GPT-4'],
        ['Hallucination Detection', 'v1.8', 'Factual accuracy (40%), Source grounding (35%), Confidence calibration (25%)', 'GPT-4'],
        ['Safety — Boundary', 'v2.0', 'Scope adherence (35%), Tool permission (25%), Data handling (25%), Refusal quality (15%)', 'GPT-4'],
        ['Safety — Harm', 'v1.5', 'Toxicity detection (40%), Bias identification (30%), Harmful instruction refusal (30%)', 'GPT-4'],
        ['Output Relevance', 'v1.9', 'Task alignment (40%), Completeness (30%), Clarity (30%)', 'GPT-4'],
    ]
    judge_table = Table(judge_data, colWidths=[1.2*inch, 0.8*inch, 3.5*inch, 0.7*inch])
    judge_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 7.5),
        ('ALIGN', (1,0), (1,-1), 'CENTER'),
        ('ALIGN', (3,0), (3,-1), 'CENTER'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(judge_table)
    story.append(Spacer(1, 12))
    
    # D. Glossary
    story.append(Paragraph("Appendix D: Glossary of Terms", styles['SubHead']))
    
    glossary = [
        ['Term', 'Definition'],
        ['ICoA', 'Ideal Course of Action — the expected best-practice recovery path for a given fault scenario'],
        ['pass^k', 'Consistency metric: probability that all k runs of the same task succeed (Yao et al., 2024)'],
        ['MTTR', 'Mean Time to Recovery — average time for the agent to resume normal operation after a fault'],
        ['LLM-as-a-Judge', 'Using an independent LLM to evaluate qualitative aspects of agent outputs (Zheng et al., 2023)'],
        ['Fault Injection', 'Deliberately introducing failures (API errors, latency, etc.) to test agent resilience'],
        ['Langfuse', 'Open-source LLM observability platform used for trace collection and analysis'],
        ['PII', 'Personally Identifiable Information — data that can identify an individual'],
        ['RAG', 'Retrieval-Augmented Generation — technique combining retrieval with LLM generation'],
        ['CoT', 'Chain of Thought — step-by-step reasoning process in LLM outputs'],
        ['RBAC', 'Role-Based Access Control — authorization model based on user roles'],
    ]
    glossary_table = Table(glossary, colWidths=[1.2*inch, 5.6*inch])
    glossary_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), BRAND_NAVY),
        ('TEXTCOLOR', (0,0), (-1,0), colors.white),
        ('FONTNAME', (0,0), (-1,0), 'Helvetica-Bold'),
        ('FONTNAME', (0,1), (0,-1), 'Helvetica-Bold'),
        ('FONTSIZE', (0,0), (-1,-1), 8.5),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, BRAND_LIGHT]),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#DEE2E6")),
    ]))
    story.append(glossary_table)
    
    story.append(Spacer(1, 30))
    
    # ═══════════════════════════════════════════════════════════
    # DIGITAL SIGNATURE FOOTER
    # ═══════════════════════════════════════════════════════════
    sig_hash = hashlib.sha256(f"{CERT_ID}-{OVERALL_SCORE}-{ISSUED_DATE.isoformat()}".encode()).hexdigest()
    
    sig_data = [
        [Paragraph(
            f'<b>Digital Signature</b><br/>'
            f'<font size="7" color="#6C757D">SHA-256: {sig_hash}</font><br/>'
            f'<font size="8">Certificate Verification: https://agentcert.contoso.com/verify/{CERT_ID}</font><br/>'
            f'<font size="7" color="#6C757D">This certificate was generated by AgentCert™ v2.0.4 on {ISSUED_DATE.strftime("%B %d, %Y")}. '
            f'Valid until {EXPIRY_DATE.strftime("%B %d, %Y")}. Tampering with this document invalidates the certificate.</font>',
            styles['SmallGray']
        )]
    ]
    sig_table = Table(sig_data, colWidths=[6.8*inch])
    sig_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), BRAND_LIGHT),
        ('BOX', (0,0), (-1,-1), 1, BRAND_GRAY),
        ('TOPPADDING', (0,0), (-1,-1), 10),
        ('BOTTOMPADDING', (0,0), (-1,-1), 10),
        ('LEFTPADDING', (0,0), (-1,-1), 12),
        ('RIGHTPADDING', (0,0), (-1,-1), 12),
    ]))
    story.append(sig_table)
    
    # Build
    doc.build(story)
    return output_path


# ═══════════════════════════════════════════════════════════════
# MAIN
# ═══════════════════════════════════════════════════════════════
def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_dir = os.path.dirname(script_dir)
    
    chart_dir = os.path.join(project_dir, 'output', 'charts')
    os.makedirs(chart_dir, exist_ok=True)
    
    output_dir = os.path.join(project_dir, 'output')
    output_path = os.path.join(output_dir, f'AgentCert_Certificate_{CERT_ID}.pdf')
    
    print("=" * 60)
    print("  AgentCert™ — Sample Certificate Report Generator")
    print("=" * 60)
    
    print("\n[1/7] Generating gauge chart...")
    generate_gauge_chart(os.path.join(chart_dir, 'gauge.png'), OVERALL_SCORE, CERT_LEVEL)
    
    print("[2/7] Generating radar chart...")
    generate_radar_chart(os.path.join(chart_dir, 'radar.png'))
    
    print("[3/7] Generating latency chart...")
    generate_latency_chart(os.path.join(chart_dir, 'latency.png'))
    
    print("[4/7] Generating task funnel chart...")
    generate_task_funnel(os.path.join(chart_dir, 'funnel.png'))
    
    print("[5/7] Generating cost breakdown chart...")
    generate_cost_breakdown(os.path.join(chart_dir, 'cost.png'))
    
    print("[6/7] Generating trend chart...")
    generate_trend_chart(os.path.join(chart_dir, 'trend.png'))
    
    print("[7/7] Generating recovery time chart...")
    generate_recovery_chart(os.path.join(chart_dir, 'recovery.png'))
    
    print(f"\n📊 All charts generated in: {chart_dir}")
    
    print("\n📄 Building PDF report...")
    result = build_pdf(output_path, chart_dir)
    
    print(f"\n✅ Certificate report generated successfully!")
    print(f"   📁 Location: {result}")
    print(f"   📋 Certificate ID: {CERT_ID}")
    print(f"   🏅 Level: {CERT_LEVEL} ({OVERALL_SCORE}/100)")
    print(f"   📊 Pages: ~12")
    print(f"   📅 Valid: {ISSUED_DATE.strftime('%b %d, %Y')} — {EXPIRY_DATE.strftime('%b %d, %Y')}")


if __name__ == '__main__':
    main()
