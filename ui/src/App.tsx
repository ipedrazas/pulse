import { useCallback } from 'react'
import { getNodes, getContainers, getHealth } from './api/client'
import { Header } from './components/Header'
import { NodeCard } from './components/NodeCard'
import { ContainerTable } from './components/ContainerTable'
import { SearchBar } from './components/SearchBar'
import { Spinner } from './components/Spinner'
import { EmptyState } from './components/EmptyState'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ToastProvider } from './components/ToastProvider'
import { usePolling } from './hooks/usePolling'
import { useLocalStorage } from './hooks/useLocalStorage'

export default function App() {
  const [selectedNode, setSelectedNode] = useLocalStorage<string | null>('pulse:selectedNode', null)
  const [search, setSearch] = useLocalStorage('pulse:search', '')

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
      <ToastProvider />
      <Header healthy={healthy} />

      <main className="mx-auto max-w-7xl px-3 py-4 sm:px-6 sm:py-6">
        {/* Nodes */}
        <section className="mb-6 sm:mb-8">
          <div className="mb-3 flex items-center justify-between sm:mb-4">
            <h2 className="text-base font-semibold sm:text-lg">Nodes</h2>
            <button
              onClick={nodes.refresh}
              aria-label="Refresh nodes"
              className="text-xs text-gray-500 hover:text-white"
            >
              Refresh
            </button>
          </div>

          {nodes.loading && <Spinner />}
          {nodes.error && <p className="text-sm text-red-400">Error: {nodes.error}</p>}

          {nodes.data && nodes.data.length === 0 && <EmptyState message="No agents connected" />}

          {nodes.data && nodes.data.length > 0 && (
            <ErrorBoundary>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {nodes.data.map((agent) => (
                  <NodeCard
                    key={agent.name}
                    agent={agent}
                    selected={selectedNode === agent.name}
                    onClick={() => setSelectedNode(selectedNode === agent.name ? null : agent.name)}
                  />
                ))}
              </div>
            </ErrorBoundary>
          )}
        </section>

        {/* Containers */}
        <section>
          <div className="mb-3 flex flex-col gap-3 sm:mb-4 sm:flex-row sm:items-center sm:gap-4">
            <h2 className="text-base font-semibold sm:text-lg">
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

          {containers.data && (!containers.data.data || containers.data.data.length === 0) && (
            <EmptyState message="No containers found" />
          )}

          {containers.data?.data && containers.data.data.length > 0 && (
            <ErrorBoundary>
              <ContainerTable containers={containers.data.data} search={search} />
            </ErrorBoundary>
          )}
        </section>
      </main>
    </div>
  )
}
