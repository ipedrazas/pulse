import { HealthBadge } from "./HealthBadge";
import { RefreshIndicator } from "./RefreshIndicator";

interface HeaderProps {
  healthy: boolean | null;
  lastUpdated: Date | null;
}

export function Header({ healthy, lastUpdated }: HeaderProps) {
  return (
    <header className="sticky top-0 z-10 border-b border-surface-border bg-surface-bg/80 backdrop-blur-sm">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-bold tracking-tight text-gray-100">
            Pulse
          </h1>
          <HealthBadge healthy={healthy} />
        </div>
        <RefreshIndicator lastUpdated={lastUpdated} />
      </div>
    </header>
  );
}
