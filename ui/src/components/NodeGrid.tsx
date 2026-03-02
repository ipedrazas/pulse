import type { NodeContainers } from "../types";
import { NodeCard } from "./NodeCard";

interface NodeGridProps {
  nodes: NodeContainers[];
}

export function NodeGrid({ nodes }: NodeGridProps) {
  return (
    <div className="grid gap-4 grid-cols-1 md:grid-cols-2 xl:grid-cols-3">
      {nodes.map((node) => (
        <NodeCard key={node.node_name} node={node} />
      ))}
    </div>
  );
}
