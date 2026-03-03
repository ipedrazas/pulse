import type { ActionResponse, HealthResponse, NodeContainers } from "../types";

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

export async function createAction(
  nodeName: string,
  action: string,
  target: string,
  params?: Record<string, string>,
): Promise<ActionResponse> {
  const res = await fetch(`/nodes/${encodeURIComponent(nodeName)}/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, target, params }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<ActionResponse>;
}

export function fetchActions(nodeName: string): Promise<ActionResponse[]> {
  return fetchJSON<ActionResponse[]>(`/nodes/${encodeURIComponent(nodeName)}/actions`);
}

export function fetchAction(nodeName: string, commandId: string): Promise<ActionResponse> {
  return fetchJSON<ActionResponse>(
    `/nodes/${encodeURIComponent(nodeName)}/actions/${encodeURIComponent(commandId)}`,
  );
}
