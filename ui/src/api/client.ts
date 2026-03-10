import type { Agent, Container, ContainerListResponse, NodeDetailResponse } from '../types'

const BASE = '/api/v1'

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json() as Promise<T>
}

export async function getNodes(): Promise<Agent[]> {
  return fetchJSON<Agent[]>(`${BASE}/nodes`)
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
  return fetchJSON<Container>(`${BASE}/containers/${encodeURIComponent(id)}`)
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
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json() as Promise<CommandResponse>
}

export async function getCommandResult(commandId: string): Promise<CommandResponse> {
  return fetchJSON<CommandResponse>(`${BASE}/commands/${encodeURIComponent(commandId)}`)
}

async function postContainerAction(containerId: string, action: string): Promise<CommandResponse> {
  const res = await fetch(`${BASE}/containers/${encodeURIComponent(containerId)}/${action}`, {
    method: 'POST',
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json() as Promise<CommandResponse>
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
