export function formatLastSeen(iso: string | null): string {
  if (!iso) return "never";

  const then = new Date(iso);
  const now = Date.now();
  const diffSeconds = Math.floor((now - then.getTime()) / 1000);

  if (diffSeconds < 0) return "just now";
  if (diffSeconds < 60) return `${diffSeconds}s ago`;

  const minutes = Math.floor(diffSeconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}
