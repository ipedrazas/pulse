import type { Container } from '../types'

interface ContainerDetailProps {
  container: Container
}

export function ContainerDetail({ container }: ContainerDetailProps) {
  return (
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
    <div className="flex gap-2 text-sm">
      <span className="text-gray-500">{label}:</span>
      <span className={`text-gray-300 ${mono ? 'font-mono text-xs' : ''}`}>{value}</span>
    </div>
  )
}
