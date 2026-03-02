export function containerStatusColor(status: string | null): string {
  switch (status) {
    case "running":
      return "bg-green-500";
    case "exited":
    case "dead":
      return "bg-red-500";
    case "paused":
      return "bg-yellow-500";
    case "restarting":
      return "bg-blue-500";
    case "created":
      return "bg-cyan-500";
    default:
      return "bg-gray-500";
  }
}

export function containerStatusTextColor(status: string | null): string {
  switch (status) {
    case "running":
      return "text-green-400";
    case "exited":
    case "dead":
      return "text-red-400";
    case "paused":
      return "text-yellow-400";
    case "restarting":
      return "text-blue-400";
    case "created":
      return "text-cyan-400";
    default:
      return "text-gray-400";
  }
}
