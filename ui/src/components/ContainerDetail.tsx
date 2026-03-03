import type { ContainerStatus } from "../types";
import { getContainerStaleness } from "../utils/containerStaleness";
import { containerStatusColor, containerStatusTextColor } from "../utils/containerStatusColor";
import { formatLastSeen } from "../utils/formatLastSeen";
import { formatUptime } from "../utils/formatUptime";
import { ContainerActions } from "./ContainerActions";

interface ContainerDetailProps {
  container: ContainerStatus;
  onClose: () => void;
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-start gap-4 py-2 border-b border-surface-border">
      <span className="text-xs text-gray-500 shrink-0">{label}</span>
      <span className="text-sm text-gray-200 text-right break-all">{value}</span>
    </div>
  );
}

const stalenessText: Record<string, string> = {
  warning: "connection missing",
  critical: "connection lost",
};

export function ContainerDetail({ container, onClose }: ContainerDetailProps) {
  const staleness = getContainerStaleness(container.last_seen);
  const statusText = stalenessText[staleness] ?? container.status ?? "unknown";
  const statusColor = containerStatusColor(container.status, staleness);
  const statusTextColor = containerStatusTextColor(container.status, staleness);
  const animate =
    container.status === "running" && staleness === "fresh" ? "animate-pulse_dot" : "";

  return (
    <div
      role="dialog"
      className="fixed inset-0 z-20 flex justify-end"
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
      }}
    >
      {/* Backdrop — clickable to dismiss */}
      <button
        type="button"
        className="absolute inset-0 bg-black/40 cursor-default"
        onClick={onClose}
        aria-label="Close panel"
      />

      {/* Panel */}
      <section className="relative w-full max-w-md bg-surface-bg border-l border-surface-border h-full overflow-y-auto animate-slide-in">
        <div className="sticky top-0 z-10 flex items-center justify-between px-5 py-4 border-b border-surface-border bg-surface-bg/90 backdrop-blur-sm">
          <h2 className="text-base font-semibold text-gray-100 truncate pr-4">{container.name}</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-gray-500 hover:text-gray-300 text-xl leading-none shrink-0"
            aria-label="Close"
          >
            &times;
          </button>
        </div>

        <div className="px-5 py-4 space-y-5">
          {/* Status badge */}
          <div className="flex items-center gap-2">
            <span className={`inline-block h-3 w-3 rounded-full ${statusColor} ${animate}`} />
            <span className={`text-sm font-medium ${statusTextColor}`}>{statusText}</span>
          </div>

          {/* Container actions */}
          <ContainerActions container={container} />

          {/* Details */}
          <div>
            <DetailRow label="Container ID" value={container.container_id} />
            <DetailRow label="Node" value={container.node_name} />
            {container.compose_project && (
              <DetailRow label="Stack" value={container.compose_project} />
            )}
            <DetailRow label="Image" value={container.image_tag} />
            <DetailRow label="Uptime" value={formatUptime(container.uptime_seconds)} />
            <DetailRow label="Last seen" value={formatLastSeen(container.last_seen)} />
          </div>

          {/* Labels */}
          {container.labels && Object.keys(container.labels).length > 0 && (
            <div>
              <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">
                Labels
              </h3>
              <div>
                {Object.entries(container.labels)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([key, value]) => (
                    <DetailRow key={key} label={key} value={value} />
                  ))}
              </div>
            </div>
          )}

          {/* Environment Variables */}
          {container.env_vars && Object.keys(container.env_vars).length > 0 && (
            <div>
              <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">
                Environment
              </h3>
              <div>
                {Object.entries(container.env_vars)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([key, value]) => (
                    <DetailRow key={key} label={key} value={value} />
                  ))}
              </div>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
