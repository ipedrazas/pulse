import type { Staleness } from "../utils/containerStaleness";
import { containerStatusColor } from "../utils/containerStatusColor";

interface StatusDotProps {
  status: string | null;
  staleness?: Staleness;
}

export function StatusDot({ status, staleness }: StatusDotProps) {
  const color = containerStatusColor(status, staleness);
  const animate =
    status === "running" && (!staleness || staleness === "fresh") ? "animate-pulse_dot" : "";

  return (
    <span
      className={`inline-block h-2.5 w-2.5 rounded-full ${color} ${animate}`}
      title={status ?? "unknown"}
    />
  );
}
