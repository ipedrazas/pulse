export interface ContainerStatus {
  container_id: string;
  node_name: string;
  name: string;
  image_tag: string;
  status: string | null;
  uptime_seconds: number | null;
  last_seen: string | null;
  labels: Record<string, string> | null;
  env_vars: Record<string, string> | null;
  compose_project: string;
}

export interface NodeContainers {
  node_name: string;
  containers: ContainerStatus[];
}

export interface HealthResponse {
  status: "healthy" | "unhealthy";
}

export interface ActionResponse {
  command_id: string;
  node_name: string;
  action: string;
  target: string;
  params: Record<string, string> | null;
  status: "pending" | "running" | "success" | "failed";
  output: string;
  duration_ms: number;
  created_at: string;
  updated_at: string;
}
