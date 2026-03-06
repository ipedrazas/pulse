export function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
  return `${Math.floor(seconds / 86400)}d`;
}

export function formatLastSeen(iso: string | undefined): string {
  if (!iso) return 'Never';
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return `${diffSec}s ago`;
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
  return date.toLocaleDateString();
}

export function statusColor(status: string): string {
  switch (status.toLowerCase()) {
    case 'running':
      return 'text-green-400';
    case 'exited':
      return 'text-red-400';
    case 'paused':
      return 'text-yellow-400';
    case 'created':
      return 'text-blue-400';
    default:
      return 'text-gray-400';
  }
}
