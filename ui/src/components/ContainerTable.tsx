import { useState, useMemo, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import type { Container } from '../types'
import { formatUptime } from '../utils/format'
import { StatusBadge } from './StatusBadge'
import { ContainerDetail } from './ContainerDetail'

interface ContainerTableProps {
  containers: Container[]
  search: string
}

type SortKey = 'name' | 'image' | 'status' | 'agent_name' | 'uptime_seconds'

const ROW_HEIGHT = 48
const DETAIL_HEIGHT = 400

export function ContainerTable({ containers, search }: ContainerTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>('name')
  const [sortAsc, setSortAsc] = useState(true)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const parentRef = useRef<HTMLDivElement>(null)

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

  const virtualizer = useVirtualizer({
    count: sorted.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) => {
      const c = sorted[index]
      return c && expandedId === c.container_id ? ROW_HEIGHT + DETAIL_HEIGHT : ROW_HEIGHT
    },
    overscan: 10,
  })

  return (
    <>
      {/* Mobile card view (not virtualised — typically small lists on mobile) */}
      <div className="space-y-3 sm:hidden" role="list" aria-label="Container list">
        {sorted.map((c) => (
          <div key={c.container_id} role="listitem">
            <button
              className="w-full rounded-lg border border-gray-800 bg-gray-900 p-4 text-left transition hover:border-gray-700"
              onClick={() => setExpandedId(expandedId === c.container_id ? null : c.container_id)}
              aria-expanded={expandedId === c.container_id}
              aria-label={`Container ${c.name}, ${c.status}`}
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

      {/* Desktop virtualised table view */}
      <div className="hidden sm:block rounded-lg border border-gray-800">
        <table className="w-full" role="table" aria-label="Container table">
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
        </table>

        <div ref={parentRef} className="overflow-y-auto bg-gray-950" style={{ maxHeight: '70vh' }}>
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const c = sorted[virtualRow.index]
              const isExpanded = expandedId === c.container_id
              return (
                <div
                  key={c.container_id}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  <table className="w-full">
                    <tbody>
                      <tr
                        className="cursor-pointer hover:bg-gray-900 border-b border-gray-800"
                        style={{ height: `${ROW_HEIGHT}px` }}
                        onClick={() => setExpandedId(isExpanded ? null : c.container_id)}
                        aria-expanded={isExpanded}
                        role="row"
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
                      {isExpanded && (
                        <tr>
                          <td colSpan={5} className="bg-gray-900 px-4 py-4">
                            <ContainerDetail container={c} />
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              )
            })}
          </div>
        </div>
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
      aria-sort={sortKey === field ? (sortAsc ? 'ascending' : 'descending') : 'none'}
      role="columnheader"
    >
      {label}
      {arrow}
    </th>
  )
}
