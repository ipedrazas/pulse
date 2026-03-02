import type { ContainerStatus, NodeContainers } from "../types";
import { getContainerStaleness } from "../utils/containerStaleness";
import { ContainerRow } from "./ContainerRow";

interface NodeCardProps {
  node: NodeContainers;
  onSelectContainer: (container: ContainerStatus) => void;
}

function groupByProject(
  containers: ContainerStatus[],
): { project: string; containers: ContainerStatus[] }[] {
  const grouped = new Map<string, ContainerStatus[]>();
  for (const c of containers) {
    const project = c.compose_project || "";
    const list = grouped.get(project);
    if (list) {
      list.push(c);
    } else {
      grouped.set(project, [c]);
    }
  }
  return Array.from(grouped.entries()).map(([project, containers]) => ({ project, containers }));
}

export function NodeCard({ node, onSelectContainer }: NodeCardProps) {
  const visibleContainers = node.containers.filter(
    (c) => getContainerStaleness(c.last_seen) !== "expired",
  );

  if (visibleContainers.length === 0) return null;

  const running = visibleContainers.filter((c) => c.status === "running").length;
  const total = visibleContainers.length;
  const stacks = groupByProject(visibleContainers);
  const hasMultipleGroups =
    stacks.length > 1 || (stacks.length === 1 && (stacks[0]?.project ?? "") !== "");

  return (
    <div className="rounded-xl border border-surface-border bg-surface-card overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3">
        <h2 className="text-sm font-semibold text-gray-200">{node.node_name}</h2>
        <span className="text-xs text-gray-500">
          {running}/{total} running
        </span>
      </div>

      <div>
        {hasMultipleGroups
          ? stacks.map((stack) => (
              <div key={stack.project || "_standalone"}>
                <div className="px-4 py-1.5 bg-white/[0.02] border-t border-surface-border">
                  <span className="text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                    {stack.project || "standalone"}
                  </span>
                </div>
                {stack.containers.map((c) => (
                  <ContainerRow key={c.container_id} container={c} onSelect={onSelectContainer} />
                ))}
              </div>
            ))
          : visibleContainers.map((c) => (
              <ContainerRow key={c.container_id} container={c} onSelect={onSelectContainer} />
            ))}
      </div>
    </div>
  );
}
