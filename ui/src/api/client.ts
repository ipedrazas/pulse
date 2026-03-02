import type { HealthResponse, NodeContainers } from "../types";

async function fetchWithRetry(url: string, retries = 3, delayMs = 1000): Promise<Response> {
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const res = await fetch(url);
      if (res.ok || res.status < 500) return res;
      if (attempt === retries) return res;
    } catch (err) {
      if (attempt === retries) throw err;
    }
    await new Promise((r) => setTimeout(r, delayMs * 2 ** attempt));
  }
  throw new Error("unreachable");
}

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetchWithRetry(url);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

export function fetchHealth(): Promise<HealthResponse> {
  return fetchJSON<HealthResponse>("/healthz");
}

export function fetchNodes(): Promise<NodeContainers[]> {
  return fetchJSON<NodeContainers[]>("/nodes");
}
