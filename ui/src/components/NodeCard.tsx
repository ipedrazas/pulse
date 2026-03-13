import type { Agent } from '../types'
import { formatLastSeen, formatUptime } from '../utils/format'
import { StatusDot } from './StatusDot'

interface NodeCardProps {
  agent: Agent
  selected: boolean
  onClick: () => void
}

export function NodeCard({ agent, selected, onClick }: NodeCardProps) {
  const meta = agent.metadata

  return (
    <button
      onClick={onClick}
      aria-label={`Node ${agent.name}, ${agent.status}, ${agent.container_count} containers`}
      aria-pressed={selected}
      className={`w-full rounded-lg border p-4 text-left transition ${
        selected
          ? 'border-blue-500 bg-gray-800'
          : 'border-gray-800 bg-gray-900 hover:border-gray-700'
      }`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <StatusDot status={agent.status} />
          <span className="font-medium text-white">{agent.name}</span>
        </div>
        <span className="text-xs text-gray-500">v{agent.version}</span>
      </div>
      <div className="mt-2 flex items-center gap-4 text-sm text-gray-400">
        <span>{agent.container_count} containers</span>
        <span>{formatLastSeen(agent.last_seen)}</span>
      </div>

      {selected && meta && (
        <div className="mt-3 border-t border-gray-700 pt-3 space-y-1 text-sm">
          {meta.hostname && <MetaRow label="Hostname" value={meta.hostname} />}
          {meta.ip_address && <MetaRow label="IP" value={meta.ip_address} />}
          {meta.os_name && (
            <MetaRow
              label="OS"
              value={`${meta.os_name}${meta.os_version ? ` ${meta.os_version}` : ''}`}
            />
          )}
          {meta.kernel_version && <MetaRow label="Kernel" value={meta.kernel_version} />}
          {meta.uptime_seconds != null && meta.uptime_seconds > 0 && (
            <MetaRow label="Uptime" value={formatUptime(meta.uptime_seconds)} />
          )}
          {meta.packages_to_update != null && meta.packages_to_update >= 0 && (
            <MetaRow label="Updates" value={`${meta.packages_to_update} packages`} />
          )}
        </div>
      )}
    </button>
  )
}

function MetaRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <span className="text-gray-500">{label}:</span>
      <span className="text-gray-300 font-mono text-xs">{value}</span>
    </div>
  )
}
