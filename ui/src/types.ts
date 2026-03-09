export interface NodeMetadata {
  hostname?: string
  ip_address?: string
  os_name?: string
  os_version?: string
  kernel_version?: string
  uptime_seconds?: number
  packages_to_update?: number
}

export interface Agent {
  name: string
  status: string
  version: string
  last_seen?: string
  container_count: number
  metadata?: NodeMetadata
}

export interface Port {
  host_ip: string
  host_port: number
  container_port: number
  protocol: string
}

export interface Container {
  container_id: string
  agent_name: string
  name: string
  image: string
  status: string
  env_vars: Record<string, string>
  mounts: string[]
  labels: Record<string, string>
  ports: Port[]
  compose_project: string
  command: string
  uptime_seconds: number
  created_at: string
  removed_at?: string
}

export interface ContainerListResponse {
  containers: Container[]
  total: number
  page_size: number
  offset: number
}

export interface NodeDetailResponse {
  agent: Agent
  containers: Container[]
}
