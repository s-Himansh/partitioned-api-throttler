const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface RequestEntry {
  time: string;
  ip: string;
  endpoint: string;
  status: string;
  latency_ms: number;
  status_code: number;
}

export interface Metrics {
  active_ips: number;
  total_allowed: number;
  total_denied: number;
  total_requests: number;
  throttle_rate: number;
  partitions: number[];
  num_partitions: number;
  limit: number;
  window_seconds: number;
  avg_latency_ms: number;
  adaptive: boolean;
  whitelist: string[];
  blacklist: string[];
  log: RequestEntry[];
}

export const api = {
  wsUrl(): string {
    const base = API_URL.replace(/^http/, "ws");
    return `${base}/ws`;
  },

  async metrics(): Promise<Metrics> {
    const res = await fetch(`${API_URL}/api/metrics`);
    return res.json();
  },

  async addWhitelist(ip: string) {
    await fetch(`${API_URL}/admin/whitelist`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ip }),
    });
  },

  async removeWhitelist(ip: string) {
    await fetch(`${API_URL}/admin/whitelist?ip=${encodeURIComponent(ip)}`, {
      method: "DELETE",
    });
  },

  async addBlacklist(ip: string) {
    await fetch(`${API_URL}/admin/blacklist`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ip }),
    });
  },

  async removeBlacklist(ip: string) {
    await fetch(`${API_URL}/admin/blacklist?ip=${encodeURIComponent(ip)}`, {
      method: "DELETE",
    });
  },
};
