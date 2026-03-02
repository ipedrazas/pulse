import type { ContainerStatus, NodeContainers } from "../types";
import { getContainerStaleness } from "../utils/containerStaleness";
import { ContainerRow } from "./ContainerRow";

interface NodeCardProps {
  node: NodeContainers;
  onSelectContainer: (container: ContainerStatus) => void;
}

export function NodeCard({ node, onSelectContainer }: NodeCardProps) {
  const visibleContainers = node.containers.filter(
    (c) => getContainerStaleness(c.last_seen) !== "expired",
  );

  if (visibleContainers.length === 0) return null;

  const running = visibleContainers.filter((c) => c.status === "running").length;
  const total = visibleContainers.length;

  return (
    <div className="rounded-xl border border-surface-border bg-surface-card overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3">
        <h2 className="text-sm font-semibold text-gray-200">{node.node_name}</h2>
        <span className="text-xs text-gray-500">
          {running}/{total} running
        </span>
      </div>

      <div>
        {visibleContainers.map((c) => (
          <ContainerRow key={c.container_id} container={c} onSelect={onSelectContainer} />
        ))}
      </div>
    </div>
  );
}
