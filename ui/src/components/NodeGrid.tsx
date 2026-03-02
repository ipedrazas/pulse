import type { ContainerStatus, NodeContainers } from "../types";
import { NodeCard } from "./NodeCard";

interface NodeGridProps {
  nodes: NodeContainers[];
  onSelectContainer: (container: ContainerStatus) => void;
}

export function NodeGrid({ nodes, onSelectContainer }: NodeGridProps) {
  return (
    <div className="grid gap-4 grid-cols-1 md:grid-cols-2 xl:grid-cols-3">
      {nodes.map((node) => (
        <NodeCard key={node.node_name} node={node} onSelectContainer={onSelectContainer} />
      ))}
    </div>
  );
}
