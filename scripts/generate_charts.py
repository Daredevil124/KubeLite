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
        print("No data rows found in CSV.")
        return

    # Normalize time to start at 0
    t0 = rows[0]['time']
    for r in rows:
        r['t'] = r['time'] - t0

    max_t = max(r['t'] for r in rows) if rows[-1]['t'] > 0 else 1.0

    # 1. Autoscaling Performance SVG (RPS, Queue, Workers)
    create_autoscaling_svg(rows, max_t, os.path.join(output_dir, "autoscaling_performance.svg"), formula_text)

    # 2. CPU and Memory Equilibrium SVG (CPU %, Mem % vs Targets)
    create_equilibrium_svg(rows, max_t, os.path.join(output_dir, "cpu_memory_equilibrium.svg"), formula_text)

    print(f"✅ Generated benchmark charts in: {output_dir}")

def create_autoscaling_svg(rows, max_t, filepath, formula_text):
    W, H = 1000, 650
    pad_l, pad_r, pad_t, pad_b = 90, 90, 180, 80
    pw = W - pad_l - pad_r
    ph = H - pad_t - pad_b

    max_workers = max(max(r['workers'] for r in rows), 4.0)
    max_queue = max(max(r['queue'] for r in rows), max(r['rps'] for r in rows), 10.0)

    def tx(t):
        return pad_l + (t / max_t) * pw

    def ty_workers(val):
        return pad_t + ph - (val / max_workers) * ph

    def ty_queue(val):
        return pad_t + ph - (val / max_queue) * ph

    # Build path strings
    workers_pts = [f"{tx(r['t']):.1f},{ty_workers(r['workers']):.1f}" for r in rows]
    queue_pts = [f"{tx(r['t']):.1f},{ty_queue(r['queue']):.1f}" for r in rows]
    rps_pts = [f"{tx(r['t']):.1f},{ty_queue(r['rps']):.1f}" for r in rows]

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
  <!-- RPS (amber) -->
  <polyline fill="none" stroke="#d97706" stroke-width="2" stroke-dasharray="4,4" points="{' '.join(rps_pts)}" />
  <!-- Queue (red) -->
  <polyline fill="none" stroke="#dc2626" stroke-width="2" points="{' '.join(queue_pts)}" />
  <!-- Workers (green) -->
  <polyline fill="none" stroke="#16a34a" stroke-width="3" points="{' '.join(workers_pts)}" />

  <!-- Axes Labels -->
  <text x="{pad_l}" y="{H - 30}" class="axis">0s</text>
  <text x="{pad_l + pw/2}" y="{H - 30}" class="axis" text-anchor="middle">Elapsed Time ({max_t:.1f}s)</text>
  <text x="{pad_l + pw}" y="{H - 30}" class="axis" text-anchor="end">{max_t:.1f}s</text>

  <!-- Left Axis (Workers) -->
  <text x="25" y="{pad_t + 15}" class="axis" fill="#16a34a">Workers: {int(max_workers)}</text>
  <text x="25" y="{pad_t + ph}" class="axis" fill="#16a34a">0</text>

  <!-- Right Axis (Queue / RPS) -->
  <text x="{W - 80}" y="{pad_t + 15}" class="axis" fill="#dc2626">Queue/RPS: {int(max_queue)}</text>
  <text x="{W - 80}" y="{pad_t + ph}" class="axis" fill="#dc2626">0</text>

  <!-- Legend -->
  <circle cx="{pad_l + 20}" cy="{pad_t + 25}" r="5" fill="#16a34a" />
  <text x="{pad_l + 32}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Active Workers (Left Axis)</text>

  <circle cx="{pad_l + 220}" cy="{pad_t + 25}" r="5" fill="#dc2626" />
  <text x="{pad_l + 232}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Queue Backlog (Right Axis)</text>

  <line x1="{pad_l + 420}" y1="{pad_t + 25}" x2="{pad_l + 440}" y2="{pad_t + 25}" stroke="#d97706" stroke-width="2" stroke-dasharray="3,3" />
  <text x="{pad_l + 448}" y="{pad_t + 29}" class="axis" fill="#fafaf9">Incoming RPS</text>
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
