import { useState } from "react";
import type { CorrelationSuggestion, OpportunityStatus } from "./types";
import { PriceSparkline } from "./PriceSparkline";
import { OpportunityBadge } from "./OpportunityBadge";

interface Props {
  suggestions: CorrelationSuggestion[];
  onSelect: (s: CorrelationSuggestion) => void;
  selected: CorrelationSuggestion | null;
}

type SortKey = "correlation" | "lag_days" | "price_trend_7d" | "volume_trend_7d";

const CANDIDATE_LABELS: Record<string, string> = {
  ingredient: "Ingredient",
  rig: "Rig",
  module: "Module",
  shared_mat: "Shared mat",
};

function badge(s: CorrelationSuggestion): OpportunityStatus {
  return s.opportunity_status;
}

function corrColor(r: number): string {
  const abs = Math.abs(r);
  if (abs >= 0.7) return "text-green-400";
  if (abs >= 0.4) return "text-yellow-400";
  return "text-eve-dim";
}

export function CorrelationResultsTable({ suggestions, onSelect, selected }: Props) {
  const [sortKey, setSortKey] = useState<SortKey>("correlation");
  const [sortDir, setSortDir] = useState<1 | -1>(-1);

  function toggleSort(k: SortKey) {
    if (sortKey === k) setSortDir((d) => (d === -1 ? 1 : -1));
    else { setSortKey(k); setSortDir(-1); }
  }

  const sorted = [...suggestions].sort((a, b) => {
    const av = Number(a[sortKey]);
    const bv = Number(b[sortKey]);
    // correlation and lag: rank by magnitude (strong negative = strong signal)
    // trends: rank by actual value so ascending/descending is directionally meaningful
    const sa = (sortKey === "correlation" || sortKey === "lag_days") ? Math.abs(av) : av;
    const sb = (sortKey === "correlation" || sortKey === "lag_days") ? Math.abs(bv) : bv;
    return sortDir * (sa - sb);
  });

  if (sorted.length === 0) {
    return (
      <p className="text-eve-dim text-xs py-6 text-center">
        No significant correlations found for this item in the selected window.
      </p>
    );
  }

  const th = (k: SortKey, label: string) => (
    <th
      className="px-2 py-1 text-left text-[10px] font-semibold text-eve-dim uppercase cursor-pointer select-none hover:text-eve-text"
      onClick={() => toggleSort(k)}
    >
      {label}{sortKey === k ? (sortDir === -1 ? " ↓" : " ↑") : ""}
    </th>
  );

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs border-collapse">
        <thead>
          <tr className="border-b border-eve-border">
            <th className="px-2 py-1 text-left text-[10px] font-semibold text-eve-dim uppercase">Item</th>
            <th className="px-2 py-1 text-left text-[10px] font-semibold text-eve-dim uppercase">Set</th>
            {th("correlation", "Corr")}
            {th("lag_days", "Lag")}
            {th("price_trend_7d", "Price Δ7d")}
            {th("volume_trend_7d", "Vol Δ7d")}
            <th className="px-2 py-1 text-left text-[10px] font-semibold text-eve-dim uppercase">Sparkline</th>
            <th className="px-2 py-1 text-left text-[10px] font-semibold text-eve-dim uppercase">Status</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((s) => {
            const isSelected = selected?.type_id === s.type_id;
            return (
              <tr
                key={s.type_id}
                className={`border-b border-eve-border/40 cursor-pointer transition-colors ${
                  isSelected ? "bg-eve-accent/10" : "hover:bg-eve-panel/40"
                }`}
                onClick={() => onSelect(s)}
              >
                <td className="px-2 py-1.5 font-medium text-eve-text">{s.type_name}</td>
                <td className="px-2 py-1.5 text-eve-dim">{CANDIDATE_LABELS[s.candidate_set] ?? s.candidate_set}</td>
                <td className={`px-2 py-1.5 font-mono tabular-nums ${corrColor(s.correlation)}`}>
                  {s.correlation.toFixed(2)}
                </td>
                <td className="px-2 py-1.5 text-eve-dim tabular-nums">{s.lag_days}d</td>
                <td className={`px-2 py-1.5 tabular-nums ${Number.isFinite(s.price_trend_7d) ? (s.price_trend_7d >= 0 ? "text-green-400" : "text-red-400") : "text-eve-dim"}`}>
                  {Number.isFinite(s.price_trend_7d) ? `${s.price_trend_7d >= 0 ? "+" : ""}${s.price_trend_7d.toFixed(1)}%` : "—"}
                </td>
                <td className={`px-2 py-1.5 tabular-nums ${Number.isFinite(s.volume_trend_7d) ? (s.volume_trend_7d >= 0 ? "text-green-400" : "text-red-400") : "text-eve-dim"}`}>
                  {Number.isFinite(s.volume_trend_7d) ? `${s.volume_trend_7d >= 0 ? "+" : ""}${s.volume_trend_7d.toFixed(1)}%` : "—"}
                </td>
                <td className="px-2 py-1.5">
                  <PriceSparkline data={s.price_series} />
                </td>
                <td className="px-2 py-1.5">
                  <OpportunityBadge status={badge(s)} />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
