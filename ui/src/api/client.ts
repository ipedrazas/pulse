import type { Agent, Container, ContainerListResponse, NodeDetailResponse } from '../types'

const BASE = '/api/v1'
const DEFAULT_TIMEOUT_MS = 15_000

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url, { signal: AbortSignal.timeout(DEFAULT_TIMEOUT_MS) })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json() as Promise<T>
}

export async function getNodes(): Promise<Agent[]> {
  const res = await fetchJSON<{ data: Agent[] }>(`${BASE}/nodes`)
  return res.data
}

export async function getNode(name: string): Promise<NodeDetailResponse> {
  return fetchJSON<NodeDetailResponse>(`${BASE}/nodes/${encodeURIComponent(name)}`)
}

export async function getContainers(
  node?: string,
  pageSize = 50,
  offset = 0,
): Promise<ContainerListResponse> {
  const params = new URLSearchParams()
  if (node) params.set('node', node)
  params.set('page_size', String(pageSize))
  params.set('offset', String(offset))
  return fetchJSON<ContainerListResponse>(`${BASE}/containers?${params}`)
}

export async function getContainer(id: string): Promise<Container> {
  const res = await fetchJSON<{ data: Container }>(`${BASE}/containers/${encodeURIComponent(id)}`)
  return res.data
}

export async function getHealth(): Promise<{ status: string }> {
  return fetchJSON<{ status: string }>('/healthz')
}

export interface CommandResponse {
  command_id: string
  status: string
  result?: string
}

export async function requestContainerLogs(
  containerId: string,
  tail = 100,
): Promise<CommandResponse> {
  const res = await fetch(`${BASE}/containers/${encodeURIComponent(containerId)}/logs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tail }),
    signal: AbortSignal.timeout(DEFAULT_TIMEOUT_MS),
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  const json = (await res.json()) as { data: CommandResponse }
  return json.data
}

export async function getCommandResult(commandId: string): Promise<CommandResponse> {
  const res = await fetchJSON<{ data: CommandResponse }>(
    `${BASE}/commands/${encodeURIComponent(commandId)}`,
  )
  return res.data
}

async function postContainerAction(containerId: string, action: string): Promise<CommandResponse> {
  const res = await fetch(`${BASE}/containers/${encodeURIComponent(containerId)}/${action}`, {
    method: 'POST',
    signal: AbortSignal.timeout(DEFAULT_TIMEOUT_MS),
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  const json = (await res.json()) as { data: CommandResponse }
  return json.data
}

export function stopContainer(containerId: string): Promise<CommandResponse> {
  return postContainerAction(containerId, 'stop')
}

export function restartContainer(containerId: string): Promise<CommandResponse> {
  return postContainerAction(containerId, 'restart')
}

export function pullContainerImage(containerId: string): Promise<CommandResponse> {
  return postContainerAction(containerId, 'pull')
}
