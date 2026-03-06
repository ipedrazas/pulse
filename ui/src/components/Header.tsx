interface HeaderProps {
  healthy: boolean
}

export function Header({ healthy }: HeaderProps) {
  return (
    <header className="border-b border-gray-800 bg-gray-950 px-6 py-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-xl font-bold text-white">Pulse</h1>
          <span className="text-sm text-gray-500">Container Management</span>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span
            className={`inline-block h-2 w-2 rounded-full ${healthy ? 'bg-green-400' : 'bg-red-400'}`}
          />
          <span className="text-gray-400">{healthy ? 'API Connected' : 'API Disconnected'}</span>
        </div>
      </div>
    </header>
  )
}
