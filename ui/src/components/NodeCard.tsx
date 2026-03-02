import type { NodeContainers } from "../types";
import { ContainerRow } from "./ContainerRow";

interface NodeCardProps {
  node: NodeContainers;
}

export function NodeCard({ node }: NodeCardProps) {
  const running = node.containers.filter((c) => c.status === "running").length;
  const total = node.containers.length;

  return (
    <div className="rounded-xl border border-surface-border bg-surface-card overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3">
        <h2 className="text-sm font-semibold text-gray-200">{node.node_name}</h2>
        <span className="text-xs text-gray-500">
          {running}/{total} running
        </span>
      </div>

      <div>
        {node.containers.map((c) => (
          <ContainerRow key={c.container_id} container={c} />
        ))}
      </div>
    </div>
  );
}
