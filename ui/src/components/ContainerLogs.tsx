import { useState, useEffect, useRef } from 'react'
import { requestContainerLogs, getCommandResult } from '../api/client'

interface ContainerLogsProps {
  containerId: string
  onClose: () => void
}

export function ContainerLogs({ containerId, onClose }: ContainerLogsProps) {
  const [lines, setLines] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setTimeout>

    async function fetchLogs() {
      setLoading(true)
      setError(null)
      try {
        const { command_id } = await requestContainerLogs(containerId, 200)

        // Poll for result
        const poll = async () => {
          if (cancelled) return
          try {
            const result = await getCommandResult(command_id)
            if (result.status === 'completed' || result.status === 'failed') {
              if (result.result) {
                setLines(result.result.split('\n'))
              } else {
                setLines(['(no output)'])
              }
              setLoading(false)
            } else {
              timer = setTimeout(poll, 1000)
            }
          } catch (err) {
            if (!cancelled) {
              setError(err instanceof Error ? err.message : 'Failed to fetch logs')
              setLoading(false)
            }
          }
        }
        timer = setTimeout(poll, 500)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to request logs')
          setLoading(false)
        }
      }
    }

    fetchLogs()
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [containerId])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  return (
    <div className="mt-4 rounded-lg border border-gray-700 bg-gray-950">
      <div className="flex items-center justify-between border-b border-gray-700 px-4 py-2">
        <h4 className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Logs
        </h4>
        <button
          onClick={onClose}
          className="text-xs text-gray-500 hover:text-white"
        >
          Close
        </button>
      </div>
      <div className="max-h-80 overflow-auto p-4 sm:max-h-96">
        {loading && (
          <p className="text-sm text-gray-500 animate-pulse">Loading logs...</p>
        )}
        {error && <p className="text-sm text-red-400">{error}</p>}
        {!loading && !error && lines.length > 0 && (
          <pre className="whitespace-pre-wrap break-all font-mono text-xs leading-5 text-gray-300">
            {lines.map((line, i) => (
              <div key={i} className="hover:bg-gray-900">
                {line}
              </div>
            ))}
            <div ref={bottomRef} />
          </pre>
        )}
      </div>
    </div>
  )
}
