const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
};
