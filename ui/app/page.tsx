"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { api, Metrics } from "../lib/api";

export default function Dashboard() {
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [connected, setConnected] = useState(false);
  const [history, setHistory] = useState<number[]>([]);
  const [deniedHistory, setDeniedHistory] = useState<number[]>([]);
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

          setHistory((prev) => [...prev.slice(-59), deltaAllowed]);
          setDeniedHistory((prev) => [...prev.slice(-59), deltaDenied]);
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
          <div className="flex items-center gap-2 text-xs text-slate-500">
            <div className={`w-2 h-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-red-400"}`}></div>
            {connected ? "Live" : "Disconnected"}
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        <div className="space-y-6">
          {/* Stats Row */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
            <StatCard label="Throttle Rate" value={`${(m.throttle_rate * 100).toFixed(1)}%`} color="text-orange-600" />
            <StatCard label="Active IPs" value={m.active_ips.toLocaleString()} color="text-cyan-600" />
            <StatCard label="Allowed" value={m.total_allowed.toLocaleString()} color="text-emerald-600" />
            <StatCard label="Denied" value={m.total_denied.toLocaleString()} color="text-red-500" />
            <StatCard label="Total" value={m.total_requests.toLocaleString()} color="text-slate-600" />
          </div>

          {/* Config */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <ConfigCard label="Partitions" value={String(m.num_partitions)} />
            <ConfigCard label="Limit" value={`${m.limit} req/window`} />
            <ConfigCard label="Window" value={`${m.window_seconds}s`} />
          </div>

          {/* Request Rate Chart */}
          <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-6 shadow-sm">
            <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-4">Requests / sec (last 60 ticks)</h3>
            <div className="flex gap-px items-end h-32">
              {history.map((v, i) => {
                const max = Math.max(...history, 1);
                return (
                  <div key={i} className="flex-1 flex flex-col gap-px">
                    <div
                      className="w-full rounded-t bg-gradient-to-t from-emerald-600 to-emerald-400 transition-all duration-200"
                      style={{ height: `${(v / max) * 100}%`, minHeight: v > 0 ? "2px" : "0" }}
                    />
                    <div
                      className="w-full rounded-b bg-gradient-to-b from-red-500 to-red-400 transition-all duration-200"
                      style={{ height: `${(deniedHistory[i] / max) * 50}%`, minHeight: deniedHistory[i] > 0 ? "2px" : "0" }}
                    />
                  </div>
                );
              })}
              {history.length === 0 && (
                <div className="w-full text-center text-slate-400 text-sm py-8">Waiting for traffic...</div>
              )}
            </div>
            <div className="flex gap-4 mt-3 text-xs text-slate-500">
              <div className="flex items-center gap-1"><div className="w-3 h-3 rounded bg-emerald-500"></div> Allowed</div>
              <div className="flex items-center gap-1"><div className="w-3 h-3 rounded bg-red-500"></div> Denied</div>
            </div>
          </div>

          {/* Partition Heatmap */}
          <div className="bg-white/80 border border-slate-200/50 rounded-2xl p-6 shadow-sm">
            <h3 className="text-sm font-bold text-slate-500 uppercase tracking-wider mb-4">Partition Distribution</h3>
            <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${Math.min(m.num_partitions, 32)}, minmax(0, 1fr))` }}>
              {m.partitions.map((size, i) => {
                const intensity = size / maxPartition;
                return (
                  <div
                    key={i}
                    className="aspect-square rounded transition-all duration-300 relative group"
                    style={{
                      backgroundColor: `rgba(14, 165, 233, ${0.1 + intensity * 0.9})`,
                    }}
                    title={`Shard ${i}: ${size} IPs`}
                  >
                    <div className="absolute inset-0 flex items-center justify-center text-[8px] text-slate-700 opacity-0 group-hover:opacity-100 transition-opacity">
                      {size}
                    </div>
                  </div>
                );
              })}
            </div>
            <div className="flex justify-between mt-3 text-xs text-slate-400">
              <span>Shard 0</span>
              <span>Shard {m.num_partitions - 1}</span>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="bg-white/80 border border-slate-200/50 rounded-xl p-4 shadow-sm">
      <p className="text-xs text-slate-500 mb-1">{label}</p>
      <p className={`text-lg font-bold font-mono ${color}`}>{value}</p>
    </div>
  );
}

function ConfigCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-white/80 border border-slate-200/50 rounded-xl p-4 shadow-sm">
      <p className="text-xs text-slate-500 mb-1">{label}</p>
      <p className="text-sm font-mono text-slate-700">{value}</p>
    </div>
  );
}
