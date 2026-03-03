import { useCallback, useEffect, useRef, useState } from "react";
import { createAction, fetchAction } from "../api/client";
import type { ActionResponse, ContainerStatus } from "../types";

interface ContainerActionsProps {
  container: ContainerStatus;
  onActionComplete?: () => void;
}

type ActionType =
  | "container_stop"
  | "container_start"
  | "container_restart"
  | "container_logs"
  | "container_inspect";

const actionLabels: Record<ActionType, string> = {
  container_stop: "Stop",
  container_start: "Start",
  container_restart: "Restart",
  container_logs: "Logs",
  container_inspect: "Inspect",
};

const TAIL_OPTIONS = ["100", "500", "1000"];

export function ContainerActions({ container, onActionComplete }: ContainerActionsProps) {
  const [confirming, setConfirming] = useState<ActionType | null>(null);
  const [activeAction, setActiveAction] = useState<ActionType | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionResult, setActionResult] = useState<ActionResponse | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [logTail, setLogTail] = useState("100");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const isRunning = container.status === "running";

  const clearPoll = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  useEffect(() => {
    return clearPoll;
  }, [clearPoll]);

  const pollForResult = useCallback(
    (nodeName: string, commandId: string) => {
      clearPoll();
      pollRef.current = setInterval(async () => {
        try {
          const result = await fetchAction(nodeName, commandId);
          if (result.status === "success" || result.status === "failed") {
            clearPoll();
            setActionLoading(false);
            setActionResult(result);
            if (result.status === "failed") {
              setActionError(result.output || "Action failed");
            }
            onActionComplete?.();
          }
        } catch {
          // keep polling
        }
      }, 2000);
    },
    [clearPoll, onActionComplete],
  );

  async function executeAction(action: ActionType) {
    setConfirming(null);
    setActiveAction(action);
    setActionLoading(true);
    setActionResult(null);
    setActionError(null);

    try {
      const params =
        action === "container_logs" ? { tail: logTail } : undefined;
      const created = await createAction(
        container.node_name,
        action,
        container.container_id,
        params,
      );
      pollForResult(container.node_name, created.command_id);
    } catch (err) {
      setActionLoading(false);
      setActionError(err instanceof Error ? err.message : "Failed to create action");
    }
  }

  function handleActionClick(action: ActionType) {
    // Destructive actions need confirmation
    if (action === "container_stop" || action === "container_start" || action === "container_restart") {
      setConfirming(action);
      setActionResult(null);
      setActionError(null);
      return;
    }
    // Logs and inspect execute immediately
    executeAction(action);
  }

  function dismissResult() {
    setActiveAction(null);
    setActionResult(null);
    setActionError(null);
  }

  const actions: ActionType[] = isRunning
    ? ["container_stop", "container_restart", "container_logs", "container_inspect"]
    : ["container_start", "container_logs", "container_inspect"];

  return (
    <div className="space-y-3">
      <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
        Actions
      </h3>

      {/* Action buttons */}
      <div className="flex flex-wrap gap-2">
        {actions.map((action) => (
          <button
            key={action}
            type="button"
            onClick={() => handleActionClick(action)}
            disabled={actionLoading}
            className={`text-xs font-medium px-2.5 py-1.5 rounded border cursor-pointer transition-colors disabled:opacity-50 ${
              action === "container_stop"
                ? "text-red-400 bg-red-400/10 border-red-400/20 hover:bg-red-400/20"
                : action === "container_start"
                  ? "text-emerald-400 bg-emerald-400/10 border-emerald-400/20 hover:bg-emerald-400/20"
                  : action === "container_restart"
                    ? "text-yellow-400 bg-yellow-400/10 border-yellow-400/20 hover:bg-yellow-400/20"
                    : "text-gray-300 bg-white/5 border-white/10 hover:bg-white/10"
            }`}
          >
            {actionLabels[action]}
          </button>
        ))}
      </div>

      {/* Log tail selector — shown when logs is about to run or confirming */}
      {(activeAction === "container_logs" || confirming === null) && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">Log lines:</span>
          {TAIL_OPTIONS.map((opt) => (
            <button
              key={opt}
              type="button"
              onClick={() => setLogTail(opt)}
              className={`text-[10px] px-1.5 py-0.5 rounded cursor-pointer ${
                logTail === opt
                  ? "bg-blue-500/20 text-blue-400 border border-blue-400/30"
                  : "text-gray-500 hover:text-gray-300 border border-transparent"
              }`}
            >
              {opt}
            </button>
          ))}
        </div>
      )}

      {/* Confirmation step for destructive actions */}
      {confirming && (
        <div className="flex items-center gap-2 p-2 rounded bg-white/5 border border-white/10">
          <span className="text-xs text-gray-300">
            {actionLabels[confirming]} container?
          </span>
          <button
            type="button"
            onClick={() => setConfirming(null)}
            className="text-[10px] text-gray-500 hover:text-gray-300 px-1.5 py-0.5 cursor-pointer"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => executeAction(confirming)}
            className="text-[10px] font-medium text-emerald-400 hover:text-emerald-300 bg-emerald-400/10 border border-emerald-400/20 rounded px-1.5 py-0.5 cursor-pointer"
          >
            Confirm
          </button>
        </div>
      )}

      {/* Loading state */}
      {actionLoading && activeAction && (
        <div className="flex items-center gap-2 text-xs text-gray-400">
          <span className="inline-block h-3 w-3 border-2 border-gray-500 border-t-gray-200 rounded-full animate-spin" />
          {actionLabels[activeAction]}...
        </div>
      )}

      {/* Error display */}
      {actionError && !actionLoading && (
        <div className="text-xs text-red-400 p-2 rounded bg-red-400/10 border border-red-400/20">
          {actionError}
        </div>
      )}

      {/* Result display for logs/inspect */}
      {actionResult && !actionLoading && actionResult.status === "success" && (
        <div className="space-y-1">
          <div className="flex items-center justify-between">
            <span className="text-xs text-emerald-400">
              {activeAction ? actionLabels[activeAction] : "Result"} —{" "}
              {actionResult.duration_ms > 0
                ? `${(actionResult.duration_ms / 1000).toFixed(1)}s`
                : "done"}
            </span>
            <button
              type="button"
              onClick={dismissResult}
              className="text-[10px] text-gray-500 hover:text-gray-300 cursor-pointer"
            >
              Dismiss
            </button>
          </div>
          {actionResult.output && (
            <pre className="text-[11px] text-gray-300 font-mono whitespace-pre-wrap bg-black/30 border border-surface-border rounded p-3 max-h-96 overflow-y-auto">
              {actionResult.output}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
