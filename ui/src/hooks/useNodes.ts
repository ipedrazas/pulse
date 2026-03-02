import { useCallback } from "react";
import { fetchNodes } from "../api/client";
import type { NodeContainers } from "../types";
import { usePolling } from "./usePolling";

const NODES_INTERVAL = 10_000;

export function useNodes() {
  const fetcher = useCallback(() => fetchNodes(), []);
  return usePolling<NodeContainers[]>(fetcher, NODES_INTERVAL);
}
