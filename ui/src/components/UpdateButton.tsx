import { useState } from "react";
import { createAction } from "../api/client";

interface UpdateButtonProps {
  nodeName: string;
  project: string;
  onActionCreated?: () => void;
}

export function UpdateButton({ nodeName, project, onActionCreated }: UpdateButtonProps) {
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await createAction(nodeName, "compose_update", project);
      setConfirming(false);
      onActionCreated?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setLoading(false);
    }
  }

  if (confirming) {
    return (
      <span className="flex items-center gap-1.5">
        {error && <span className="text-[10px] text-red-400">{error}</span>}
        <button
          type="button"
          onClick={() => setConfirming(false)}
          disabled={loading}
          className="text-[10px] text-gray-500 hover:text-gray-300 px-1.5 py-0.5 cursor-pointer"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={handleConfirm}
          disabled={loading}
          className="text-[10px] font-medium text-emerald-400 hover:text-emerald-300 bg-emerald-400/10 border border-emerald-400/20 rounded px-1.5 py-0.5 cursor-pointer disabled:opacity-50"
        >
          {loading ? "Updating..." : "Confirm"}
        </button>
      </span>
    );
  }

  return (
    <button
      type="button"
      onClick={() => setConfirming(true)}
      className="text-[10px] font-medium text-gray-400 hover:text-gray-200 bg-white/5 border border-white/10 rounded px-1.5 py-0.5 cursor-pointer transition-colors"
      title={`Pull and restart ${project}`}
    >
      Update
    </button>
  );
}
