import { Component } from 'react'
import type { ErrorInfo, ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="flex min-h-[200px] items-center justify-center rounded-lg border border-red-800 bg-gray-900 p-6">
          <div className="text-center">
            <h2 className="mb-2 text-lg font-semibold text-red-400">Something went wrong</h2>
            <p className="mb-4 text-sm text-gray-400">{this.state.error.message}</p>
            <button
              onClick={() => this.setState({ error: null })}
              className="rounded border border-gray-600 px-4 py-2 text-sm text-gray-300 hover:border-white hover:text-white"
            >
              Try again
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
