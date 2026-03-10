import { useState, useMemo } from 'react'
import type { Container } from '../types'
import { formatUptime } from '../utils/format'
import { StatusBadge } from './StatusBadge'
import { ContainerDetail } from './ContainerDetail'

interface ContainerTableProps {
  containers: Container[]
  search: string
}

type SortKey = 'name' | 'image' | 'status' | 'agent_name' | 'uptime_seconds'

export function ContainerTable({ containers, search }: ContainerTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>('name')
  const [sortAsc, setSortAsc] = useState(true)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const filtered = useMemo(() => {
    const q = search.toLowerCase()
    return containers.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.image.toLowerCase().includes(q) ||
        c.agent_name.toLowerCase().includes(q) ||
        c.container_id.toLowerCase().includes(q),
    )
  }, [containers, search])

  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (typeof av === 'number' && typeof bv === 'number') {
        return sortAsc ? av - bv : bv - av
      }
      const as = String(av)
      const bs = String(bv)
      return sortAsc ? as.localeCompare(bs) : bs.localeCompare(as)
    })
  }, [filtered, sortKey, sortAsc])

  function handleSort(key: SortKey) {
    if (sortKey === key) {
      setSortAsc(!sortAsc)
    } else {
      setSortKey(key)
      setSortAsc(true)
    }
  }

  return (
    <>
      {/* Mobile card view */}
      <div className="space-y-3 sm:hidden">
        {sorted.map((c) => (
          <div key={c.container_id}>
            <button
              className="w-full rounded-lg border border-gray-800 bg-gray-900 p-4 text-left transition hover:border-gray-700"
              onClick={() => setExpandedId(expandedId === c.container_id ? null : c.container_id)}
            >
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-white truncate">{c.name}</span>
                <StatusBadge status={c.status} />
              </div>
              <p className="mt-1 truncate font-mono text-xs text-gray-400">{c.image}</p>
              <div className="mt-2 flex items-center gap-3 text-xs text-gray-500">
                <span>{c.agent_name}</span>
                <span>{formatUptime(c.uptime_seconds)}</span>
              </div>
            </button>
            {expandedId === c.container_id && (
              <div className="rounded-b-lg border border-t-0 border-gray-800 bg-gray-900 px-4 py-4">
                <ContainerDetail container={c} />
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Desktop table view */}
      <div className="hidden overflow-x-auto rounded-lg border border-gray-800 sm:block">
        <table className="w-full">
          <thead className="bg-gray-900">
            <tr>
              <SortHeader
                label="Name"
                field="name"
                sortKey={sortKey}
                sortAsc={sortAsc}
                onSort={handleSort}
              />
              <SortHeader
                label="Image"
                field="image"
                sortKey={sortKey}
                sortAsc={sortAsc}
                onSort={handleSort}
              />
              <SortHeader
                label="Status"
                field="status"
                sortKey={sortKey}
                sortAsc={sortAsc}
                onSort={handleSort}
              />
              <SortHeader
                label="Node"
                field="agent_name"
                sortKey={sortKey}
                sortAsc={sortAsc}
                onSort={handleSort}
              />
              <SortHeader
                label="Uptime"
                field="uptime_seconds"
                sortKey={sortKey}
                sortAsc={sortAsc}
                onSort={handleSort}
              />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800 bg-gray-950">
            {sorted.map((c) => (
              <>
                <tr
                  key={c.container_id}
                  className="cursor-pointer hover:bg-gray-900"
                  onClick={() =>
                    setExpandedId(expandedId === c.container_id ? null : c.container_id)
                  }
                >
                  <td className="px-4 py-3 text-sm text-white">{c.name}</td>
                  <td className="px-4 py-3 text-sm text-gray-300 font-mono">{c.image}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={c.status} />
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-400">{c.agent_name}</td>
                  <td className="px-4 py-3 text-sm text-gray-400">
                    {formatUptime(c.uptime_seconds)}
                  </td>
                </tr>
                {expandedId === c.container_id && (
                  <tr key={`${c.container_id}-detail`}>
                    <td colSpan={5} className="bg-gray-900 px-4 py-4">
                      <ContainerDetail container={c} />
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}

function SortHeader({
  label,
  field,
  sortKey,
  sortAsc,
  onSort,
}: {
  label: string
  field: SortKey
  sortKey: SortKey
  sortAsc: boolean
  onSort: (key: SortKey) => void
}) {
  const arrow = sortKey === field ? (sortAsc ? ' \u2191' : ' \u2193') : ''
  return (
    <th
      className="cursor-pointer px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-400 hover:text-white"
      onClick={() => onSort(field)}
    >
      {label}
      {arrow}
    </th>
  )
}
