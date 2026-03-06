import { useState, useCallback } from 'react'
import { getNodes, getContainers, getHealth } from './api/client'
import { Header } from './components/Header'
import { NodeCard } from './components/NodeCard'
import { ContainerTable } from './components/ContainerTable'
import { SearchBar } from './components/SearchBar'
import { Spinner } from './components/Spinner'
import { EmptyState } from './components/EmptyState'
import { usePolling } from './hooks/usePolling'

export default function App() {
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const health = usePolling(getHealth, 15000)
  const nodes = usePolling(getNodes, 10000)

  const containerFetcher = useCallback(
    () => getContainers(selectedNode ?? undefined, 200),
    [selectedNode],
  )
  const containers = usePolling(containerFetcher, 10000)

  const healthy = health.data?.status === 'ok'

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      <Header healthy={healthy} />

      <main className="mx-auto max-w-7xl px-6 py-6">
        {/* Nodes */}
        <section className="mb-8">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold">Nodes</h2>
            <button onClick={nodes.refresh} className="text-xs text-gray-500 hover:text-white">
              Refresh
            </button>
          </div>

          {nodes.loading && <Spinner />}
          {nodes.error && <p className="text-sm text-red-400">Error: {nodes.error}</p>}

          {nodes.data && nodes.data.length === 0 && <EmptyState message="No agents connected" />}

          {nodes.data && nodes.data.length > 0 && (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {nodes.data.map((agent) => (
                <NodeCard
                  key={agent.name}
                  agent={agent}
                  selected={selectedNode === agent.name}
                  onClick={() => setSelectedNode(selectedNode === agent.name ? null : agent.name)}
                />
              ))}
            </div>
          )}
        </section>

        {/* Containers */}
        <section>
          <div className="mb-4 flex items-center gap-4">
            <h2 className="text-lg font-semibold">
              Containers
              {selectedNode && (
                <span className="ml-2 text-sm font-normal text-gray-500">on {selectedNode}</span>
              )}
              {containers.data && (
                <span className="ml-2 text-sm font-normal text-gray-500">
                  ({containers.data.total})
                </span>
              )}
            </h2>
            <div className="flex-1">
              <SearchBar value={search} onChange={setSearch} placeholder="Filter containers..." />
            </div>
          </div>

          {containers.loading && <Spinner />}
          {containers.error && <p className="text-sm text-red-400">Error: {containers.error}</p>}

          {containers.data &&
            (!containers.data.containers || containers.data.containers.length === 0) && (
              <EmptyState message="No containers found" />
            )}

          {containers.data?.containers && containers.data.containers.length > 0 && (
            <ContainerTable containers={containers.data.containers} search={search} />
          )}
        </section>
      </main>
    </div>
  )
}
