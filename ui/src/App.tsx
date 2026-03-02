import { useMemo, useState } from "react";
import { ContainerDetail } from "./components/ContainerDetail";
import { EmptyState } from "./components/EmptyState";
import { Header } from "./components/Header";
import { NodeGrid } from "./components/NodeGrid";
import { SearchBar } from "./components/SearchBar";
import { Spinner } from "./components/Spinner";
import { useHealth } from "./hooks/useHealth";
import { useNodes } from "./hooks/useNodes";
import type { ContainerStatus, NodeContainers } from "./types";

function filterNodes(nodes: NodeContainers[], query: string): NodeContainers[] {
  if (!query) return nodes;
  const q = query.toLowerCase();

  return nodes
    .map((node) => {
      const nodeMatch = node.node_name.toLowerCase().includes(q);
      if (nodeMatch) return node;

      const filtered = node.containers.filter(
        (c) => c.name.toLowerCase().includes(q) || c.image_tag.toLowerCase().includes(q),
      );
      if (filtered.length === 0) return null;
      return { ...node, containers: filtered };
    })
    .filter((n): n is NodeContainers => n !== null);
}

export default function App() {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<ContainerStatus | null>(null);
  const health = useHealth();
  const nodes = useNodes();

  const healthy = health.data != null ? health.data.status === "healthy" : null;
  const allNodes = nodes.data ?? [];

  const totalNodes = allNodes.length;
  const totalContainers = allNodes.reduce((sum, n) => sum + n.containers.length, 0);

  const filtered = useMemo(() => filterNodes(allNodes, search), [allNodes, search]);

  return (
    <>
      <Header
        healthy={healthy}
        lastUpdated={nodes.lastUpdated}
        totalNodes={totalNodes}
        totalContainers={totalContainers}
      />

      <main className="mx-auto max-w-7xl px-4 py-6 space-y-4">
        <SearchBar value={search} onChange={setSearch} />

        {nodes.loading ? (
          <Spinner />
        ) : nodes.error ? (
          <EmptyState
            title="Connection error"
            message={`Could not reach the API: ${nodes.error}`}
          />
        ) : filtered.length === 0 ? (
          search ? (
            <EmptyState title="No matches" message={`Nothing matched "${search}"`} />
          ) : (
            <EmptyState
              title="No nodes"
              message="No nodes are reporting yet. Start an agent to see data."
            />
          )
        ) : (
          <NodeGrid nodes={filtered} onSelectContainer={setSelected} />
        )}
      </main>

      {selected && <ContainerDetail container={selected} onClose={() => setSelected(null)} />}
    </>
  );
}
