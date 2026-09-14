"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { api, Metrics } from "../lib/api";

export default function Dashboard() {
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [connected, setConnected] = useState(false);
  const [history, setHistory] = useState<{ allowed: number; denied: number }[]>([]);
  const [log, setLog] = useState<{ time: string; status: "allowed" | "denied"; ip: string }[]>([]);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<() => void>(() => {});
  const prevAllowed = useRef(0);
  const prevDenied = useRef(0);

  const connect = useCallback(() => {
    const ws = new WebSocket(api.wsUrl());
    wsRef.current = ws;

    ws.onopen = () => setConnected(true);
    ws.onclose = () => {
      setConnected(false);
      setTimeout(reconnectRef.current, 2000);
    };
    ws.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data);
        if (event.type === "metrics" && event.data) {
          const snap = event.data;
          setMetrics(snap);

          const deltaAllowed = snap.total_allowed - prevAllowed.current;
          const deltaDenied = snap.total_denied - prevDenied.current;
          prevAllowed.current = snap.total_allowed;
          prevDenied.current = snap.total_denied;

          if (deltaAllowed > 0 || deltaDenied > 0) {
            setHistory((prev) => [...prev.slice(-59), { allowed: deltaAllowed, denied: deltaDenied }]);
            const now = new Date().toLocaleTimeString();
            if (deltaDenied > 0) {
              setLog((prev) => [...prev.slice(-19), { time: now, status: "denied", ip: "..." }]);
            }
            if (deltaAllowed > 0) {
              setLog((prev) => [...prev.slice(-19), { time: now, status: "allowed", ip: "..." }]);
            }
          }
        }
      } catch {}
    };
  }, []);

  useEffect(() => {
    reconnectRef.current = connect;
  }, [connect]);

  useEffect(() => {
    connect();
    return () => wsRef.current?.close();
  }, [connect]);

  useEffect(() => {
    if (!connected && !metrics) {
      api.metrics().then(setMetrics).catch(() => {});
    }
  }, [connected]);

  const m = metrics || {
    active_ips: 0,
    total_allowed: 0,
    total_denied: 0,
    total_requests: 0,
    throttle_rate: 0,
    partitions: [],
    num_partitions: 64,
    limit: 10,
    window_seconds: 60,
  };

  const maxPartition = Math.max(...m.partitions, 1);
  const gridCols = Math.ceil(Math.sqrt(m.num_partitions));

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-100 via-slate-50 to-blue-50/50 text-slate-800">
      {/* Header */}
      <header className="border-b border-slate-200/60 bg-slate-100/80 backdrop-blur-xl sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-orange-500 to-red-600 flex items-center justify-center shadow-lg shadow-orange-500/25">
              <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
            <div>
              <h1 className="text-xl font-bold">Partitioned API Throttler</h1>
              <p className="text-xs text-slate-500">Real-time rate limiting dashboard</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="text-right text-xs text-slate-500">
              <div>{m.active_ips} active IPs</div>
              <div>{m.num_partitions} shards</div>
            </div>
            <div className={`w-2.5 h-2.5 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-red-400"}`}></div>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">

          {/* Left Column: Gauge + Config */}
          <div className="space-y-6">
            {/* Throttle Rate Gauge */}
            <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-6 shadow-sm flex flex-col items-center">
              <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-4">Throttle Rate</h3>
              <ThrottleGauge rate={m.throttle_rate} />
              <div className="flex gap-6 mt-4 text-xs">
                <div className="text-center">
                  <div className="text-emerald-600 font-bold text-lg">{m.total_allowed.toLocaleString()}</div>
                  <div className="text-slate-400">Allowed</div>
                </div>
                <div className="text-center">
                  <div className="text-red-500 font-bold text-lg">{m.total_denied.toLocaleString()}</div>
                  <div className="text-slate-400">Denied</div>
                </div>
              </div>
            </div>

            {/* Config */}
            <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-5 shadow-sm">
              <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-3">Configuration</h3>
              <div className="space-y-2">
                <ConfigRow label="Partitions" value={String(m.num_partitions)} />
                <ConfigRow label="Limit" value={`${m.limit} req`} />
                <ConfigRow label="Window" value={`${m.window_seconds}s`} />
              </div>
            </div>

            {/* Request Log */}
            <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-5 shadow-sm">
              <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-3">Live Log</h3>
              <div className="max-h-48 overflow-y-auto space-y-1 font-mono text-xs">
                {log.length === 0 && (
                  <div className="text-slate-400 text-center py-4">No traffic yet</div>
                )}
                {log.map((entry, i) => (
                  <div key={i} className="flex items-center gap-2 py-0.5">
                    <span className="text-slate-400 w-16 shrink-0">{entry.time}</span>
                    <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${entry.status === "allowed" ? "bg-emerald-500" : "bg-red-500"}`} />
                    <span className={entry.status === "allowed" ? "text-emerald-600" : "text-red-500"}>
                      {entry.status === "allowed" ? "200" : "429"}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Right Column: Charts */}
          <div className="lg:col-span-3 space-y-6">
            {/* Stats Row */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <BigStat label="Total Requests" value={m.total_requests.toLocaleString()} icon={<BarChartIcon />} />
              <BigStat label="Active IPs" value={m.active_ips.toLocaleString()} icon={<UsersIcon />} />
              <BigStat label="Allowed" value={m.total_allowed.toLocaleString()} color="text-emerald-600" icon={<CheckIcon />} />
              <BigStat label="Denied" value={m.total_denied.toLocaleString()} color="text-red-500" icon={<XIcon />} />
            </div>

            {/* Request Rate Chart */}
            <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-6 shadow-sm">
              <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-4">Throughput (last 60 ticks)</h3>
              <div className="relative h-40">
                {/* Grid lines */}
                <div className="absolute inset-0 flex flex-col justify-between pointer-events-none">
                  {[0, 1, 2, 3, 4].map((i) => (
                    <div key={i} className="border-b border-slate-100 w-full" />
                  ))}
                </div>
                {/* Bars */}
                <div className="absolute inset-0 flex gap-px items-end">
                  {history.map((h, i) => {
                    const maxVal = Math.max(...history.map((x) => x.allowed + x.denied), 1);
                    const allowedH = (h.allowed / maxVal) * 100;
                    const deniedH = (h.denied / maxVal) * 100;
                    return (
                      <div key={i} className="flex-1 flex flex-col items-center gap-0" title={`+${h.allowed} / -${h.denied}`}>
                        <div
                          className="w-full bg-gradient-to-t from-emerald-600 to-emerald-400 rounded-t-sm transition-all duration-300"
                          style={{ height: `${allowedH}%`, minHeight: h.allowed > 0 ? "2px" : "0" }}
                        />
                        <div
                          className="w-full bg-gradient-to-t from-red-500 to-red-400 transition-all duration-300"
                          style={{ height: `${deniedH}%`, minHeight: h.denied > 0 ? "2px" : "0" }}
                        />
                      </div>
                    );
                  })}
                  {history.length === 0 && (
                    <div className="w-full text-center text-slate-400 text-sm py-12">Waiting for traffic...</div>
                  )}
                </div>
              </div>
              <div className="flex gap-6 mt-3 text-xs text-slate-500 justify-center">
                <div className="flex items-center gap-1.5"><div className="w-3 h-2 rounded-sm bg-emerald-500"></div> Allowed</div>
                <div className="flex items-center gap-1.5"><div className="w-3 h-2 rounded-sm bg-red-500"></div> Denied</div>
              </div>
            </div>

            {/* Partition Heatmap */}
            <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-6 shadow-sm">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider">Partition Heatmap</h3>
                <div className="flex items-center gap-2 text-[10px] text-slate-400">
                  <span>0</span>
                  <div className="flex gap-px">
                    {[0.1, 0.3, 0.5, 0.7, 1.0].map((v) => (
                      <div key={v} className="w-4 h-2 rounded-sm" style={{ backgroundColor: `rgba(14, 165, 233, ${v})` }} />
                    ))}
                  </div>
                  <span>{maxPartition}</span>
                </div>
              </div>
              <div
                className="grid gap-1"
                style={{ gridTemplateColumns: `repeat(${gridCols}, minmax(0, 1fr))` }}
              >
                {m.partitions.map((size, i) => {
                  const intensity = maxPartition > 0 ? size / maxPartition : 0;
                  const hue = intensity > 0.7 ? 0 : 30; // red for hot, orange for warm
                  return (
                    <div
                      key={i}
                      className="aspect-square rounded-md transition-all duration-300 relative group cursor-default border border-transparent hover:border-slate-400"
                      style={{
                        backgroundColor: size === 0
                          ? "rgb(241 245 249)"
                          : `hsl(${200 - intensity * 170}, ${70 + intensity * 20}%, ${65 - intensity * 25}%)`,
                      }}
                    >
                      <div className="absolute -top-8 left-1/2 -translate-x-1/2 bg-slate-800 text-white text-[10px] px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none z-10">
                        Shard {i}: {size} IPs
                      </div>
                    </div>
                  );
                })}
              </div>
              <div className="flex justify-between mt-3 text-[10px] text-slate-400">
                <span>Shard 0</span>
                <span>Shard {m.num_partitions - 1}</span>
              </div>
            </div>

            {/* Partition Bar Chart */}
            <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-6 shadow-sm">
              <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-4">Partition Load</h3>
              <div className="flex gap-px items-end h-24">
                {m.partitions.map((size, i) => (
                  <div
                    key={i}
                    className="flex-1 bg-gradient-to-t from-cyan-600 to-cyan-400 rounded-t transition-all duration-300 relative group"
                    style={{
                      height: `${(size / maxPartition) * 100}%`,
                      minHeight: size > 0 ? "3px" : "1px",
                    }}
                    title={`Shard ${i}: ${size} IPs`}
                  >
                    <div className="absolute -top-6 left-1/2 -translate-x-1/2 bg-slate-800 text-white text-[10px] px-1.5 py-0.5 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none z-10">
                      {size}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

function ThrottleGauge({ rate }: { rate: number }) {
  const pct = Math.min(rate * 100, 100);
  const radius = 50;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (pct / 100) * circumference;

  const color =
    pct < 30 ? "#10b981" : pct < 60 ? "#f59e0b" : pct < 80 ? "#f97316" : "#ef4444";

  return (
    <div className="relative w-36 h-36">
      <svg viewBox="0 0 120 120" className="w-full h-full -rotate-90">
        <circle cx="60" cy="60" r={radius} fill="none" stroke="#e2e8f0" strokeWidth="8" />
        <circle
          cx="60"
          cy="60"
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth="8"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          className="transition-all duration-700 ease-out"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-2xl font-bold font-mono" style={{ color }}>{pct.toFixed(1)}%</span>
        <span className="text-[10px] text-slate-400 mt-0.5">denied</span>
      </div>
    </div>
  );
}

function BigStat({ label, value, color, icon }: { label: string; value: string; color?: string; icon: React.ReactNode }) {
  return (
    <div className="bg-white/80 border border-slate-200/50 rounded-xl p-4 shadow-sm">
      <div className="flex items-center justify-between mb-2">
        <p className="text-xs text-slate-500">{label}</p>
        <div className="text-slate-300">{icon}</div>
      </div>
      <p className={`text-2xl font-bold font-mono ${color || "text-slate-700"}`}>{value}</p>
    </div>
  );
}

function ConfigRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-xs text-slate-500">{label}</span>
      <span className="text-sm font-mono text-slate-700 bg-slate-100 px-2 py-0.5 rounded">{value}</span>
    </div>
  );
}

function BarChartIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
    </svg>
  );
}

function UsersIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
    </svg>
  );
}

function XIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
    </svg>
  );
}
