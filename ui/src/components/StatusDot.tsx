interface StatusDotProps {
  status: string
  className?: string
}

export function StatusDot({ status, className = '' }: StatusDotProps) {
  const colorMap: Record<string, string> = {
    online: 'bg-green-400',
    offline: 'bg-orange-400',
    lost: 'bg-red-400',
  }
  const color = colorMap[status] ?? 'bg-gray-500'
  return (
    <span
      className={`inline-block h-2.5 w-2.5 rounded-full ${color} ${className}`}
      title={status}
    />
  )
}
