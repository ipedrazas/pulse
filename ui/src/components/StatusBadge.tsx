import { statusColor } from '../utils/format'

interface StatusBadgeProps {
  status: string
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const bgMap: Record<string, string> = {
    running: 'bg-green-400/10',
    exited: 'bg-red-400/10',
    paused: 'bg-yellow-400/10',
    created: 'bg-blue-400/10',
  }
  const bg = bgMap[status.toLowerCase()] ?? 'bg-gray-400/10'

  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${bg} ${statusColor(status)}`}
    >
      {status}
    </span>
  )
}
