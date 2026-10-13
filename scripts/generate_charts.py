#!/usr/bin/env python3
"""
KubeLite Benchmark Chart Generator
Generates clean standalone SVG graphs with embedded mathematical formulas
without needing third-party libraries (uses standard library Python).
"""

import sys
import os
import csv
import math

def generate_charts(csv_path, output_dir, formula_text):
    if not os.path.exists(csv_path):
        print(f"⚠️  Error: Metrics CSV file was not found at: {csv_path}")
        print("   Please ensure Master node is running on http://localhost:8080 and that you have write permissions.")
        return

    rows = []
    with open(csv_path, 'r') as f:
        reader = csv.DictReader(f)
        for r in reader:
            try:
                rows.append({
                    'time': float(r['timestamp_sec']),
                    'cpu': float(r['cpu_percent']),
                    'mem': float(r['memory_percent']),
                    'queue': float(r['queue_length']),
                    'workers': float(r['worker_count']),
                    'rps': float(r['rps']),
                    'tps': float(r['tps']),
                })
            except Exception:
                continue

    if not rows:
        print(f"⚠️  Warning: No data rows found in CSV: {csv_path}")
        print("   Master node was either not receiving traffic or unreachable during the benchmark.")
        return

    # Normalize time to start at 0
    t0 = rows[0]['time']
    for r in rows:
        r['t'] = r['time'] - t0

    max_t = max(r['t'] for r in rows) if rows[-1]['t'] > 0 else 1.0

    # 1. Dedicated Metric Graphs
    # Graph 1: Traffic Velocity (RPS & TPS)
    max_rate = max(max(r['rps'] for r in rows), max(r['tps'] for r in rows), 2.0) * 1.2
    create_single_chart_svg(
        rows, max_t, os.path.join(output_dir, "1_traffic_throughput.svg"),
        title="TRAFFIC VELOCITY: CLIENT RPS vs TASK COMPLETION TPS",
        subtitle="Evaluates ingress rate (RPS) vs cluster processing capacity (TPS)",
        y_min=0, y_max=max_rate, y_unit="req/s",
        series=[
            {'key': 'rps', 'label': 'Incoming Client RPS', 'color': '#f59e0b', 'dash': '4,4', 'width': 2.0},
            {'key': 'tps', 'label': 'Completed Tasks TPS', 'color': '#10b981', 'dash': '', 'width': 2.5},
        ]
    )

    # Graph 2: Queue Backlog
    max_q = max(max(r['queue'] for r in rows), 5.0) * 1.15
    create_single_chart_svg(
        rows, max_t, os.path.join(output_dir, "2_queue_backlog.svg"),
        title="QUEUE BACKLOG: PENDING WORKLOAD IN REDIS",
        subtitle="Unprocessed task payloads waiting in 'task_queue' broker",
        y_min=0, y_max=max_q, y_unit="tasks",
        series=[
            {'key': 'queue', 'label': 'Redis Queue Depth (task_queue)', 'color': '#ef4444', 'dash': '', 'width': 2.5},
        ]
    )

    # Graph 3: Worker Scaling
    max_w = max(max(r['workers'] for r in rows), 4.0) + 2
    create_single_chart_svg(
        rows, max_t, os.path.join(output_dir, "3_worker_scaling.svg"),
        title="AUTOSCALER REPLICA MANAGEMENT: ACTIVE WORKERS",
        subtitle="Docker worker containers spawned with label 'role=worker'",
        y_min=0, y_max=max_w, y_unit="workers",
        series=[
            {'key': 'workers', 'label': 'Active Worker Replicas', 'color': '#22c55e', 'dash': '', 'width': 3.0},
        ],
        reference_lines=[
            {'val': 16.0, 'label': 'Host 80% Safety Ceiling: 16 Workers', 'color': '#a855f7'}
        ]
    )

    # Graph 4: CPU Utilization
    create_single_chart_svg(
        rows, max_t, os.path.join(output_dir, "4_cpu_utilization.svg"),
        title="CLUSTER CPU UTILIZATION & EQUILIBRIUM",
        subtitle="Average CPU utilization across workers (capped at 0.5 CPU cores per container)",
        y_min=0, y_max=100.0, y_unit="%",
        series=[
            {'key': 'cpu', 'label': 'Average Cluster CPU %', 'color': '#f97316', 'dash': '', 'width': 2.5},
        ],
        reference_lines=[
            {'val': 70.0, 'label': 'Target CPU: 70%', 'color': '#ef4444'}
        ]
    )

    # Graph 5: Memory Utilization (Zoomed to show real variations)
    max_mem_obs = max(max(r['mem'] for r in rows), 0.5)
    mem_y_max = max(max_mem_obs * 1.5, 5.0)
    create_single_chart_svg(
        rows, max_t, os.path.join(output_dir, "5_memory_utilization.svg"),
        title="CLUSTER RAM CONSUMPTION (ZOOMED VIEW)",
        subtitle=f"Average worker memory % (Zoomed 0–{mem_y_max:.1f}% | 256MB allocated per container)",
        y_min=0, y_max=mem_y_max, y_unit="%",
        series=[
            {'key': 'mem', 'label': 'Average Cluster RAM %', 'color': '#38bdf8', 'dash': '', 'width': 2.5},
        ]
    )

    # 2. Dual-Axis Composite Overview Graphs (Preserved for backwards compatibility)
    create_autoscaling_svg(rows, max_t, os.path.join(output_dir, "autoscaling_performance.svg"), formula_text)
    create_equilibrium_svg(rows, max_t, os.path.join(output_dir, "cpu_memory_equilibrium.svg"), formula_text)

    print(f"✅ Generated 5 dedicated metric charts + 2 composite charts in: {output_dir}")

def create_single_chart_svg(rows, max_t, filepath, title, subtitle, y_min, y_max, y_unit, series, reference_lines=None):
    W, H = 1000, 520
    pad_l, pad_r, pad_t, pad_b = 90, 80, 110, 70
    pw = W - pad_l - pad_r
    ph = H - pad_t - pad_b

    def tx(t):
        return pad_l + (t / max_t) * pw

    def ty(val):
        span = y_max - y_min
        if span <= 0: span = 1.0
        clamped = min(max(val, y_min), y_max)
        return pad_t + ph - ((clamped - y_min) / span) * ph

    # Build grid lines (4 horizontal divisions)
    grid_lines = []
    y_labels = []
    for i in range(5):
        frac = i / 4.0
        val = y_min + frac * (y_max - y_min)
        y_pos = pad_t + ph - frac * ph
        grid_lines.append(f'<line x1="{pad_l}" y1="{y_pos:.1f}" x2="{pad_l + pw}" y2="{y_pos:.1f}" class="grid" />')
        y_labels.append(f'<text x="{pad_l - 12}" y="{y_pos + 4:.1f}" class="axis" text-anchor="end">{val:.1f} {y_unit}</text>')

    # Vertical time divisions (4 time points)
    t_labels = []
    for i in range(5):
        frac = i / 4.0
        t_val = frac * max_t
        x_pos = pad_l + frac * pw
        grid_lines.append(f'<line x1="{x_pos:.1f}" y1="{pad_t}" x2="{x_pos:.1f}" y2="{pad_t + ph}" class="grid" />')
        anchor = "start" if i == 0 else ("end" if i == 4 else "middle")
        t_labels.append(f'<text x="{x_pos:.1f}" y="{H - 30}" class="axis" text-anchor="{anchor}">{t_val:.1f}s</text>')

    # Reference lines (e.g. Target lines or Safety Ceilings)
    ref_svg = []
    if reference_lines:
        for ref in reference_lines:
            if y_min <= ref['val'] <= y_max:
                ry = ty(ref['val'])
                ref_svg.append(f'<line x1="{pad_l}" y1="{ry:.1f}" x2="{pad_l + pw}" y2="{ry:.1f}" stroke="{ref["color"]}" stroke-width="1.5" stroke-dasharray="6,4" />')
                ref_svg.append(f'<text x="{pad_l + pw - 10}" y="{ry - 6:.1f}" class="axis" fill="{ref["color"]}" text-anchor="end">{ref["label"]}</text>')

    # Data series lines
    series_svg = []
    legend_svg = []
    leg_x = pad_l + 10
    for s in series:
        pts = [f"{tx(r['t']):.1f},{ty(r[s['key']]):.1f}" for r in rows]
        dash_attr = f' stroke-dasharray="{s["dash"]}"' if s.get('dash') else ''
        series_svg.append(f'<polyline fill="none" stroke="{s["color"]}" stroke-width="{s["width"]}"{dash_attr} points="{" ".join(pts)}" />')

        # Legend item
        legend_svg.append(f'<line x1="{leg_x}" y1="88" x2="{leg_x + 20}" y2="88" stroke="{s["color"]}" stroke-width="{s["width"]}"{dash_attr} />')
        legend_svg.append(f'<text x="{leg_x + 28}" y="92" class="axis" fill="#fafaf9">{s["label"]}</text>')
        leg_x += len(s["label"]) * 8 + 60

    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" style="background:#0c0a09; font-family:'Inter',system-ui,sans-serif;">
  <style>
    .grid {{ stroke: #201d1a; stroke-width: 1; }}
    .title {{ font-size: 15px; font-weight: 700; fill: #fafaf9; letter-spacing: 0.5px; }}
    .sub {{ font-size: 11px; fill: #a8a29e; font-family: monospace; }}
    .axis {{ font-size: 11px; fill: #78716c; font-family: monospace; }}
    .box {{ fill: #141210; stroke: #3d3832; stroke-width: 1; rx: 6px; }}
  </style>

  <!-- Header Box -->
  <rect x="25" y="15" width="{W - 50}" height="85" class="box" />
  <text x="40" y="38" class="title">{title}</text>
  <text x="40" y="58" class="sub">{subtitle}</text>
  {''.join(legend_svg)}

  <!-- Plot Background -->
  <rect x="{pad_l}" y="{pad_t}" width="{pw}" height="{ph}" fill="#141210" stroke="#27231f" />

  <!-- Grid & Axes -->
  {''.join(grid_lines)}
  {''.join(y_labels)}
  {''.join(t_labels)}
  <text x="{pad_l + pw / 2:.1f}" y="{H - 12}" class="axis" text-anchor="middle">Elapsed Time (seconds)</text>

  <!-- Reference Lines -->
  {''.join(ref_svg)}

  <!-- Data Polyline -->
  {''.join(series_svg)}
</svg>
"""
    with open(filepath, "w") as f:
        f.write(svg)

def create_autoscaling_svg(rows, max_t, filepath, formula_text):
    W, H = 1000, 650
    pad_l, pad_r, pad_t, pad_b = 90, 90, 180, 80
    pw = W - pad_l - pad_r
    ph = H - pad_t - pad_b

    max_workers = max(max(r['workers'] for r in rows), 4.0)
    max_queue = max(max(r['queue'] for r in rows), 10.0)
    max_rps = max(max(r['rps'] for r in rows), 5.0)

    def tx(t):
        return pad_l + (t / max_t) * pw

    def ty_workers(val):
        return pad_t + ph - (val / max_workers) * ph

    def ty_queue(val):
        return pad_t + ph - (val / max_queue) * ph

    def ty_rps(val):
        return pad_t + ph - (val / max_rps) * ph

    workers_pts = [f"{tx(r['t']):.1f},{ty_workers(r['workers']):.1f}" for r in rows]
    queue_pts = [f"{tx(r['t']):.1f},{ty_queue(r['queue']):.1f}" for r in rows]
    rps_pts = [f"{tx(r['t']):.1f},{ty_rps(r['rps']):.1f}" for r in rows]

    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" style="background:#0c0a09; font-family:'Inter',system-ui,sans-serif;">
  <style>
    .grid {{ stroke: #27231f; stroke-width: 1; }}
    .title {{ font-size: 16px; font-weight: 700; fill: #fafaf9; }}
    .sub {{ font-size: 11px; fill: #a8a29e; font-family: monospace; }}
    .axis {{ font-size: 11px; fill: #78716c; font-family: monospace; }}
    .box {{ fill: #141210; stroke: #3d3832; stroke-width: 1; rx: 6px; }}
  </style>

  <!-- Formula Header Box -->
  <rect x="25" y="15" width="{W - 50}" height="145" class="box" />
  <text x="40" y="38" class="title">AUTOSCALING FORMULA &amp; MATHEMATICAL DEFINITIONS</text>
  <text x="40" y="60" class="sub" fill="#ea580c">1. ResourceDemand = noOfWorkers × max(cpuUsage / 70.0, memoryUsage / 75.0)</text>
  <text x="40" y="80" class="sub" fill="#ea580c">2. TrafficDemand  = (queueLength / 5.0) + max(0, (rps - tps) / 2.0)</text>
  <text x="40" y="100" class="sub" fill="#ea580c">3. x (Desired)    = max(1, ⌈max(ResourceDemand, TrafficDemand)⌉)</text>
  <text x="40" y="120" class="sub" fill="#16a34a">4. Target Workers = min(x, min(y, z))  where y = ⌊(CPUs×0.8)/0.5⌋, z = ⌊(RAM_MB×0.8)/256⌋</text>
  <text x="40" y="142" class="axis">Plot: Workload Velocity (RPS &amp; Queue Backlog) vs. Replicas Scaled (role=worker)</text>

  <!-- Plot Background -->
  <rect x="{pad_l}" y="{pad_t}" width="{pw}" height="{ph}" fill="#141210" stroke="#27231f" />

  <!-- Grid lines -->
  <line x1="{pad_l}" y1="{pad_t + ph/2}" x2="{pad_l + pw}" y2="{pad_t + ph/2}" class="grid" />
  <line x1="{pad_l + pw/2}" y1="{pad_t}" x2="{pad_l + pw/2}" y2="{pad_t + ph}" class="grid" />

  <!-- Data Lines -->
  <!-- RPS (amber) - Scaled to max_rps -->
  <polyline fill="none" stroke="#d97706" stroke-width="2" stroke-dasharray="4,4" points="{' '.join(rps_pts)}" />
  <!-- Queue (red) - Scaled to max_queue -->
  <polyline fill="none" stroke="#dc2626" stroke-width="2" points="{' '.join(queue_pts)}" />
  <!-- Workers (green) - Scaled to max_workers -->
  <polyline fill="none" stroke="#16a34a" stroke-width="3" points="{' '.join(workers_pts)}" />

  <!-- Axes Labels -->
  <text x="{pad_l}" y="{H - 30}" class="axis">0s</text>
  <text x="{pad_l + pw/2}" y="{H - 30}" class="axis" text-anchor="middle">Elapsed Time ({max_t:.1f}s)</text>
  <text x="{pad_l + pw}" y="{H - 30}" class="axis" text-anchor="end">{max_t:.1f}s</text>

  <!-- Left Axis (Workers) -->
  <text x="25" y="{pad_t + 15}" class="axis" fill="#16a34a">Workers: {int(max_workers)}</text>
  <text x="25" y="{pad_t + ph}" class="axis" fill="#16a34a">0</text>

  <!-- Right Axis (Queue / RPS) -->
  <text x="{W - 80}" y="{pad_t + 15}" class="axis" fill="#dc2626">Queue: {int(max_queue)}</text>
  <text x="{W - 80}" y="{pad_t + 35}" class="axis" fill="#d97706">RPS: {max_rps:.1f}</text>
  <text x="{W - 80}" y="{pad_t + ph}" class="axis" fill="#dc2626">0</text>

  <!-- Legend -->
  <circle cx="{pad_l + 20}" cy="{pad_t + 25}" r="5" fill="#16a34a" />
  <text x="{pad_l + 32}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Active Workers (Left Axis)</text>

  <circle cx="{pad_l + 220}" cy="{pad_t + 25}" r="5" fill="#dc2626" />
  <text x="{pad_l + 232}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Queue Backlog (Right Axis)</text>

  <line x1="{pad_l + 420}" y1="{pad_t + 25}" x2="{pad_l + 440}" y2="{pad_t + 25}" stroke="#d97706" stroke-width="2" stroke-dasharray="3,3" />
  <text x="{pad_l + 448}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Incoming RPS (Normalized)</text>
</svg>
"""
    with open(filepath, "w") as f:
        f.write(svg)

def create_equilibrium_svg(rows, max_t, filepath, formula_text):
    W, H = 1000, 650
    pad_l, pad_r, pad_t, pad_b = 90, 90, 180, 80
    pw = W - pad_l - pad_r
    ph = H - pad_t - pad_b

    def tx(t):
        return pad_l + (t / max_t) * pw

    def ty_pct(val):
        return pad_t + ph - (min(max(val, 0.0), 100.0) / 100.0) * ph

    cpu_pts = [f"{tx(r['t']):.1f},{ty_pct(r['cpu']):.1f}" for r in rows]
    mem_pts = [f"{tx(r['t']):.1f},{ty_pct(r['mem']):.1f}" for r in rows]

    target_cpu_y = ty_pct(70.0)
    target_mem_y = ty_pct(75.0)

    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" style="background:#0c0a09; font-family:'Inter',system-ui,sans-serif;">
  <style>
    .grid {{ stroke: #27231f; stroke-width: 1; }}
    .title {{ font-size: 16px; font-weight: 700; fill: #fafaf9; }}
    .sub {{ font-size: 11px; fill: #a8a29e; font-family: monospace; }}
    .axis {{ font-size: 11px; fill: #78716c; font-family: monospace; }}
    .box {{ fill: #141210; stroke: #3d3832; stroke-width: 1; rx: 6px; }}
  </style>

  <!-- Formula Header Box -->
  <rect x="25" y="15" width="{W - 50}" height="145" class="box" />
  <text x="40" y="38" class="title">CLUSTER EQUILIBRIUM &amp; UTILIZATION TARGETS</text>
  <text x="40" y="60" class="sub" fill="#ea580c">Target CPU = 70.0%  (cpuRatio = cpuUsage / 70.0)</text>
  <text x="40" y="80" class="sub" fill="#ea580c">Target RAM = 75.0%  (memRatio = memoryUsage / 75.0)</text>
  <text x="40" y="100" class="sub" fill="#ea580c">Resource Pressure: D_resource = noOfWorkers × max(cpuRatio, memRatio)</text>
  <text x="40" y="120" class="sub" fill="#16a34a">Goal: Stabilize cluster load near dotted 70%/75% reference thresholds during bursts</text>
  <text x="40" y="142" class="axis">Plot: Average Container Utilization % vs. Stabilization Setpoints</text>

  <!-- Plot Background -->
  <rect x="{pad_l}" y="{pad_t}" width="{pw}" height="{ph}" fill="#141210" stroke="#27231f" />

  <!-- Reference Target Lines -->
  <line x1="{pad_l}" y1="{target_cpu_y:.1f}" x2="{pad_l + pw}" y2="{target_cpu_y:.1f}" stroke="#ea580c" stroke-width="1.5" stroke-dasharray="6,4" />
  <text x="{pad_l + pw + 8}" y="{target_cpu_y + 4:.1f}" class="axis" fill="#ea580c">Target CPU: 70%</text>

  <line x1="{pad_l}" y1="{target_mem_y:.1f}" x2="{pad_l + pw}" y2="{target_mem_y:.1f}" stroke="#3b82f6" stroke-width="1.5" stroke-dasharray="6,4" />
  <text x="{pad_l + pw + 8}" y="{target_mem_y + 4:.1f}" class="axis" fill="#3b82f6">Target RAM: 75%</text>

  <!-- Data Lines -->
  <polyline fill="none" stroke="#3b82f6" stroke-width="2" points="{' '.join(mem_pts)}" />
  <polyline fill="none" stroke="#ea580c" stroke-width="2.5" points="{' '.join(cpu_pts)}" />

  <!-- Axes Labels -->
  <text x="{pad_l}" y="{H - 30}" class="axis">0s</text>
  <text x="{pad_l + pw/2}" y="{H - 30}" class="axis" text-anchor="middle">Elapsed Time ({max_t:.1f}s)</text>
  <text x="{pad_l + pw}" y="{H - 30}" class="axis" text-anchor="end">{max_t:.1f}s</text>

  <!-- Left Y-Axis (Percentages) -->
  <text x="35" y="{pad_t + 10}" class="axis">100%</text>
  <text x="35" y="{target_cpu_y + 4:.1f}" class="axis" fill="#ea580c">70%</text>
  <text x="35" y="{pad_t + ph/2}" class="axis">50%</text>
  <text x="35" y="{pad_t + ph}" class="axis">0%</text>

  <!-- Legend -->
  <circle cx="{pad_l + 20}" cy="{pad_t + 25}" r="5" fill="#ea580c" />
  <text x="{pad_l + 32}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Average CPU % (all workers)</text>

  <circle cx="{pad_l + 240}" cy="{pad_t + 25}" r="5" fill="#3b82f6" />
  <text x="{pad_l + 252}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Average Memory % (all workers)</text>
</svg>
"""
    with open(filepath, "w") as f:
        f.write(svg)

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: generate_charts.py <metrics_csv_path> <output_dir> [formula_text]")
        sys.exit(1)
    csv_file = sys.argv[1]
    out_dir = sys.argv[2]
    formula = sys.argv[3] if len(sys.argv) > 3 else "Default Balanced Autoscaler"
    generate_charts(csv_file, out_dir, formula)
