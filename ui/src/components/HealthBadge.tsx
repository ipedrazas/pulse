interface HealthBadgeProps {
  healthy: boolean | null;
}

export function HealthBadge({ healthy }: HealthBadgeProps) {
  if (healthy === null) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-gray-500">
        <span className="h-2 w-2 rounded-full bg-gray-500" />
        Checking...
      </span>
    );
  }

  return (
    <span
      className={`inline-flex items-center gap-1.5 text-xs ${healthy ? "text-green-400" : "text-red-400"}`}
    >
      <span
        className={`h-2 w-2 rounded-full ${healthy ? "bg-green-500 animate-pulse_dot" : "bg-red-500"}`}
      />
      API {healthy ? "healthy" : "unreachable"}
    </span>
  );
}
