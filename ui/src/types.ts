export interface ContainerStatus {
  container_id: string;
  node_name: string;
  name: string;
  image_tag: string;
  status: string | null;
  uptime_seconds: number | null;
  last_seen: string | null;
}

export interface NodeContainers {
  node_name: string;
  containers: ContainerStatus[];
}

export interface HealthResponse {
  status: "healthy" | "unhealthy";
}
