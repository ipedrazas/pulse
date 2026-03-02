export type Staleness = "fresh" | "warning" | "critical" | "expired";

export function getContainerStaleness(lastSeen: string | null): Staleness {
  if (!lastSeen) return "expired";

  const ageMs = Date.now() - new Date(lastSeen).getTime();
  const ageMin = ageMs / 60_000;

  if (ageMin < 15) return "fresh";
  if (ageMin < 30) return "warning";
  if (ageMin < 60) return "critical";
  return "expired";
}
