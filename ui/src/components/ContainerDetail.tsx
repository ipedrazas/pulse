import { useState } from 'react'
import type { Container } from '../types'
import { ContainerLogs } from './ContainerLogs'

interface ContainerDetailProps {
  container: Container
}

export function ContainerDetail({ container }: ContainerDetailProps) {
  const [showLogs, setShowLogs] = useState(false)

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
      <div className="mt-4 flex gap-2">
        <button
          onClick={() => setShowLogs(!showLogs)}
          className="rounded border border-gray-600 px-3 py-1.5 text-xs font-medium text-gray-300 hover:border-blue-500 hover:text-white transition"
        >
          {showLogs ? 'Hide Logs' : 'View Logs'}
        </button>
      </div>

      {showLogs && (
        <ContainerLogs
          containerId={container.container_id}
          onClose={() => setShowLogs(false)}
        />
      )}
    </div>
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
