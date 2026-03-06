interface EmptyStateProps {
  message: string
}

export function EmptyState({ message }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-gray-800 bg-gray-900 p-12 text-center">
      <p className="text-gray-500">{message}</p>
    </div>
  )
}
