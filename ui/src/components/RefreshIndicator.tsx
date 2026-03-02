import { useEffect, useState } from "react";

interface RefreshIndicatorProps {
  lastUpdated: Date | null;
}

export function RefreshIndicator({ lastUpdated }: RefreshIndicatorProps) {
  const [, setTick] = useState(0);

  useEffect(() => {
    const id = setInterval(() => setTick((t) => t + 1), 1000);
    return () => clearInterval(id);
  }, []);

  if (!lastUpdated) return null;

  const seconds = Math.floor((Date.now() - lastUpdated.getTime()) / 1000);
  const label = seconds < 5 ? "just now" : `${seconds}s ago`;

  return (
    <span className="text-xs text-gray-500">Updated {label}</span>
  );
}
