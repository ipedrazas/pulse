import { useCallback, useEffect, useState } from "react";
import { fetchActions } from "../api/client";
import type { ActionResponse } from "../types";

interface ActionsPanelProps {
  nodeName: string;
  onClose: () => void;
}

const statusColors: Record<string, string> = {
  pending: "text-yellow-400",
  running: "text-blue-400",
  success: "text-emerald-400",
  failed: "text-red-400",
};

export function ActionsPanel({ nodeName, onClose }: ActionsPanelProps) {
  const [actions, setActions] = useState<ActionResponse[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await fetchActions(nodeName);
      setActions(data);
    } catch {
      // silently ignore
    }
  }, [nodeName]);

  useEffect(() => {
    load();
    // Poll while any action is pending/running
    const id = setInterval(load, 3000);
    return () => clearInterval(id);
  }, [load]);

  return (
    <div
      role="dialog"
      className="fixed inset-0 z-20 flex justify-end"
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
      }}
    >
      <button
        type="button"
        className="absolute inset-0 bg-black/40 cursor-default"
        onClick={onClose}
        aria-label="Close panel"
      />

      <section className="relative w-full max-w-md bg-surface-bg border-l border-surface-border h-full overflow-y-auto animate-slide-in">
        <div className="sticky top-0 z-10 flex items-center justify-between px-5 py-4 border-b border-surface-border bg-surface-bg/90 backdrop-blur-sm">
          <h2 className="text-base font-semibold text-gray-100">Actions &mdash; {nodeName}</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-gray-500 hover:text-gray-300 text-xl leading-none shrink-0"
            aria-label="Close"
          >
            &times;
          </button>
        </div>

        <div className="px-5 py-4 space-y-2">
          {actions.length === 0 ? (
            <p className="text-sm text-gray-500">No recent actions.</p>
          ) : (
            actions.map((a) => (
              <div
                key={a.command_id}
                className="border border-surface-border rounded-lg overflow-hidden"
              >
                <button
                  type="button"
                  onClick={() => setExpanded(expanded === a.command_id ? null : a.command_id)}
                  className="flex items-center justify-between w-full px-4 py-2.5 text-sm text-left hover:bg-white/5 transition-colors cursor-pointer"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span className={`font-medium ${statusColors[a.status] ?? "text-gray-400"}`}>
                      {a.status}
                    </span>
                    <span className="text-gray-300 truncate">{a.action.replace("_", " ")}</span>
                    {a.target && <span className="text-gray-500 text-xs truncate">{a.target}</span>}
                  </div>
                  <span className="text-xs text-gray-500 shrink-0 ml-2">
                    {a.duration_ms > 0 ? `${(a.duration_ms / 1000).toFixed(1)}s` : "..."}
                  </span>
                </button>

                {expanded === a.command_id && a.output && (
                  <div className="px-4 pb-3 border-t border-surface-border">
                    <pre className="text-xs text-gray-400 whitespace-pre-wrap font-mono mt-2 max-h-60 overflow-y-auto">
                      {a.output}
                    </pre>
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  );
}
