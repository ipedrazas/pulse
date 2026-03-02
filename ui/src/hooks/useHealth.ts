import { useCallback } from "react";
import { fetchHealth } from "../api/client";
import type { HealthResponse } from "../types";
import { usePolling } from "./usePolling";

const HEALTH_INTERVAL = 15_000;

export function useHealth() {
  const fetcher = useCallback(() => fetchHealth(), []);
  return usePolling<HealthResponse>(fetcher, HEALTH_INTERVAL);
}
