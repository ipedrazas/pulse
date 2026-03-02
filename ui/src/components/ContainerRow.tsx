import type { ContainerStatus } from "../types";
import { containerStatusTextColor } from "../utils/containerStatusColor";
import { formatLastSeen } from "../utils/formatLastSeen";
import { formatUptime } from "../utils/formatUptime";
import { StatusDot } from "./StatusDot";

interface ContainerRowProps {
  container: ContainerStatus;
}

export function ContainerRow({ container }: ContainerRowProps) {
  const statusText = container.status ?? "unknown";
  const statusColor = containerStatusTextColor(container.status);

  return (
    <div className="flex items-center gap-3 border-t border-surface-border px-4 py-2.5 text-sm">
      <StatusDot status={container.status} />

      <div className="min-w-0 flex-1">
        <span className="font-medium text-gray-200 truncate block">{container.name}</span>
        <span className="font-mono text-xs text-gray-500 truncate block">
          {container.image_tag}
        </span>
      </div>

      <span className={`text-xs font-medium ${statusColor} shrink-0`}>{statusText}</span>

      <span className="text-xs text-gray-500 w-16 text-right shrink-0">
        {formatUptime(container.uptime_seconds)}
      </span>

      <span className="text-xs text-gray-500 w-20 text-right shrink-0 hidden sm:block">
        {formatLastSeen(container.last_seen)}
      </span>
    </div>
  );
}
