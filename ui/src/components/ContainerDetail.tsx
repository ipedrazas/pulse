import { useState } from 'react'
import type { Container } from '../types'
import {
  stopContainer,
  restartContainer,
  pullContainerImage,
  getCommandResult,
} from '../api/client'
import { ContainerLogs } from './ContainerLogs'

interface ContainerDetailProps {
  container: Container
}

type ActionState = 'idle' | 'pending' | 'success' | 'error'

export function ContainerDetail({ container }: ContainerDetailProps) {
  const [showLogs, setShowLogs] = useState(false)
  const [actionState, setActionState] = useState<Record<string, ActionState>>({})
  const [actionError, setActionError] = useState<string | null>(null)

  async function runAction(name: string, action: () => Promise<{ command_id: string }>) {
    setActionState((s) => ({ ...s, [name]: 'pending' }))
    setActionError(null)
    try {
      const { command_id } = await action()
      // Poll for completion
      const poll = async () => {
        const result = await getCommandResult(command_id)
        if (result.status === 'completed') {
          setActionState((s) => ({ ...s, [name]: 'success' }))
        } else if (result.status === 'failed') {
          setActionState((s) => ({ ...s, [name]: 'error' }))
          setActionError(result.result || 'Command failed')
        } else {
          setTimeout(poll, 1000)
        }
      }
      setTimeout(poll, 500)
    } catch (err) {
      setActionState((s) => ({ ...s, [name]: 'error' }))
      setActionError(err instanceof Error ? err.message : 'Action failed')
    }
  }

  const isRunning = container.status.toLowerCase() === 'running'

  return (
    <div>
      <div className="grid gap-4 md:grid-cols-2">
        <Section title="Info">
          <KV label="ID" value={container.container_id} mono />
          <KV label="Command" value={container.command || '-'} mono />
          <KV label="Compose Project" value={container.compose_project || '-'} />
        </Section>

      <Section title="Environment Variables">
        {!container.env_vars || Object.keys(container.env_vars).length === 0 ? (
          <p className="text-sm text-gray-500">None</p>
        ) : (
          Object.entries(container.env_vars).map(([k, v]) => (
            <KV key={k} label={k} value={v} mono />
          ))
        )}
      </Section>

      <Section title="Ports">
        {!container.ports || container.ports.length === 0 ? (
          <p className="text-sm text-gray-500">None</p>
        ) : (
          container.ports.map((p, i) => (
            <KV
              key={i}
              label={`${p.protocol}`}
              value={`${p.host_ip || '0.0.0.0'}:${p.host_port} -> ${p.container_port}`}
              mono
            />
          ))
        )}
      </Section>

      <Section title="Mounts">
        {!container.mounts || container.mounts.length === 0 ? (
          <p className="text-sm text-gray-500">None</p>
        ) : (
          container.mounts.map((m, i) => (
            <p key={i} className="font-mono text-xs text-gray-300">
              {m}
            </p>
          ))
        )}
      </Section>

      <Section title="Labels">
        {!container.labels || Object.keys(container.labels).length === 0 ? (
          <p className="text-sm text-gray-500">None</p>
        ) : (
          Object.entries(container.labels).map(([k, v]) => <KV key={k} label={k} value={v} mono />)
        )}
      </Section>
      </div>

      {/* Actions */}
      <div className="mt-4 flex flex-wrap gap-2">
        <ActionButton
          label={showLogs ? 'Hide Logs' : 'View Logs'}
          onClick={() => setShowLogs(!showLogs)}
        />
        {isRunning && (
          <>
            <ActionButton
              label="Stop"
              state={actionState.stop}
              variant="danger"
              onClick={() => runAction('stop', () => stopContainer(container.container_id))}
            />
            <ActionButton
              label="Restart"
              state={actionState.restart}
              variant="warning"
              onClick={() => runAction('restart', () => restartContainer(container.container_id))}
            />
          </>
        )}
        <ActionButton
          label="Pull Image"
          state={actionState.pull}
          onClick={() => runAction('pull', () => pullContainerImage(container.container_id))}
        />
      </div>

      {actionError && (
        <p className="mt-2 text-xs text-red-400">{actionError}</p>
      )}

      {showLogs && (
        <ContainerLogs
          containerId={container.container_id}
          onClose={() => setShowLogs(false)}
        />
      )}
    </div>
  )
}

function ActionButton({
  label,
  onClick,
  state = 'idle',
  variant = 'default',
}: {
  label: string
  onClick: () => void
  state?: ActionState
  variant?: 'default' | 'danger' | 'warning'
}) {
  const isPending = state === 'pending'

  const variantClasses = {
    default: 'border-gray-600 hover:border-blue-500 hover:text-white',
    danger: 'border-gray-600 hover:border-red-500 hover:text-red-400',
    warning: 'border-gray-600 hover:border-yellow-500 hover:text-yellow-400',
  }

  const stateLabel =
    state === 'pending' ? '...' : state === 'success' ? 'Done' : state === 'error' ? 'Failed' : null

  return (
    <button
      onClick={onClick}
      disabled={isPending}
      className={`rounded border px-3 py-1.5 text-xs font-medium text-gray-300 transition disabled:opacity-50 ${variantClasses[variant]}`}
    >
      {stateLabel ?? label}
    </button>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500">{title}</h4>
      <div className="space-y-1">{children}</div>
    </div>
  )
}

function KV({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex flex-col gap-0.5 text-sm sm:flex-row sm:gap-2">
      <span className="text-gray-500 shrink-0">{label}:</span>
      <span className={`text-gray-300 break-all ${mono ? 'font-mono text-xs' : ''}`}>{value}</span>
    </div>
  )
}
