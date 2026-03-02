import { containerStatusColor } from "../utils/containerStatusColor";

export function StatusDot({ status }: { status: string | null }) {
  const color = containerStatusColor(status);
  const animate = status === "running" ? "animate-pulse_dot" : "";

  return (
    <span
      className={`inline-block h-2.5 w-2.5 rounded-full ${color} ${animate}`}
      title={status ?? "unknown"}
    />
  );
}
