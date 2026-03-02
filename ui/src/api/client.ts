import type { HealthResponse, NodeContainers } from "../types";

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

export function fetchHealth(): Promise<HealthResponse> {
  return fetchJSON<HealthResponse>("/healthz");
}

export function fetchNodes(): Promise<NodeContainers[]> {
  return fetchJSON<NodeContainers[]>("/nodes");
}
