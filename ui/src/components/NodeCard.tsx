import { useState } from "react";
import type { ContainerStatus, NodeContainers } from "../types";
import { getContainerStaleness } from "../utils/containerStaleness";
import { ActionsPanel } from "./ActionsPanel";
import { ContainerRow } from "./ContainerRow";
import { UpdateButton } from "./UpdateButton";

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
  const [showActions, setShowActions] = useState(false);

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
    <>
      <div className="rounded-xl border border-surface-border bg-surface-card overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3">
          <h2 className="text-sm font-semibold text-gray-200">{node.node_name}</h2>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setShowActions(true)}
              className="text-[10px] font-medium text-gray-500 hover:text-gray-300 cursor-pointer transition-colors"
            >
              Actions
            </button>
            <span className="text-xs text-gray-500">
              {running}/{total} running
            </span>
          </div>
        </div>

        <div>
          {hasMultipleGroups
            ? stacks.map((stack) => (
                <div key={stack.project || "_standalone"}>
                  <div className="flex items-center justify-between px-4 py-1.5 bg-white/[0.02] border-t border-surface-border">
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                      {stack.project || "standalone"}
                    </span>
                    {stack.project && (
                      <UpdateButton nodeName={node.node_name} project={stack.project} />
                    )}
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

      {showActions && (
        <ActionsPanel nodeName={node.node_name} onClose={() => setShowActions(false)} />
      )}
    </>
  );
}
