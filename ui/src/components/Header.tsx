import { PulseLogo } from './PulseLogo'

interface HeaderProps {
  healthy: boolean
}

export function Header({ healthy }: HeaderProps) {
  return (
    <header className="border-b border-gray-800 bg-gray-950 px-4 py-3 sm:px-6 sm:py-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 sm:gap-3">
          <PulseLogo />
          <h1 className="text-lg font-bold text-white sm:text-xl">Pulse</h1>
          <span className="hidden text-sm text-gray-500 sm:inline">v{__APP_VERSION__}</span>
        </div>
        <div
          className="flex items-center gap-2 text-sm"
          role="status"
          aria-label={healthy ? 'API Connected' : 'API Disconnected'}
        >
          <span
            className={`inline-block h-2 w-2 rounded-full ${healthy ? 'bg-green-400' : 'bg-red-400'}`}
            aria-hidden="true"
          />
          <span className="hidden text-gray-400 sm:inline">
            {healthy ? 'API Connected' : 'API Disconnected'}
          </span>
        </div>
      </div>
    </header>
  )
}
