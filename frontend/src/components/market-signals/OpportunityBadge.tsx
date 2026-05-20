import type { OpportunityStatus } from "./types";

interface Props { status: OpportunityStatus; }

const CFG: Record<OpportunityStatus, { label: string; cls: string }> = {
  NOT_MOVED_YET: { label: "Not moved yet", cls: "bg-green-500/20 text-green-300 border-green-500/40" },
  FOLLOWING:     { label: "Following",     cls: "bg-yellow-500/20 text-yellow-300 border-yellow-500/40" },
  ALREADY_MOVED: { label: "Already moved", cls: "bg-red-500/20 text-red-300 border-red-500/40" },
  NO_SIGNAL:     { label: "Flat",          cls: "bg-slate-500/20 text-slate-400 border-slate-500/40" },
};

export function OpportunityBadge({ status }: Props) {
  const { label, cls } = CFG[status];
  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium border ${cls}`}>
      {label}
    </span>
  );
}
