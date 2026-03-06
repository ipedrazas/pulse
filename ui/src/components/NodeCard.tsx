import type { Agent } from '../types'
import { formatLastSeen } from '../utils/format'
import { StatusDot } from './StatusDot'

interface NodeCardProps {
  agent: Agent
  selected: boolean
  onClick: () => void
}

export function NodeCard({ agent, selected, onClick }: NodeCardProps) {
  return (
    <button
      onClick={onClick}
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
    </button>
  )
}
