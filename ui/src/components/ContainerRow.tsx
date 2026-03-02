import type { ContainerStatus } from "../types";
import { getContainerStaleness } from "../utils/containerStaleness";
import { containerStatusTextColor } from "../utils/containerStatusColor";
import { formatLastSeen } from "../utils/formatLastSeen";
import { formatUptime } from "../utils/formatUptime";
import { StatusDot } from "./StatusDot";

interface ContainerRowProps {
  container: ContainerStatus;
  onSelect: (container: ContainerStatus) => void;
}

const stalenessText: Record<string, string> = {
  warning: "connection missing",
  critical: "connection lost",
};

export function ContainerRow({ container, onSelect }: ContainerRowProps) {
  const staleness = getContainerStaleness(container.last_seen);
  const statusText = stalenessText[staleness] ?? container.status ?? "unknown";
  const statusColor = containerStatusTextColor(container.status, staleness);

  return (
    <button
      type="button"
      onClick={() => onSelect(container)}
      className="flex items-center gap-3 border-t border-surface-border px-4 py-2.5 text-sm w-full text-left hover:bg-white/5 transition-colors cursor-pointer"
    >
      <StatusDot status={container.status} staleness={staleness} />

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-200 truncate">{container.name}</span>
          {container.labels?.["com.docker.compose.project"] && (
            <span className="text-[10px] font-medium text-gray-400 bg-white/5 border border-white/10 rounded px-1.5 py-0.5 shrink-0">
              {container.labels["com.docker.compose.project"]}
            </span>
          )}
        </div>
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
    </button>
  );
}
