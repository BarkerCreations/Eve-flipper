import { useCallback, useEffect, useRef, useState } from "react";
import type {
  CorrelationSuggestion,
  CorrelationSuggestionsResponse,
  ItemSearchHit,
} from "./types";
import { fetchSuggestions, fetchPair, searchItems } from "./correlationApi";
import { CorrelationResultsTable } from "./CorrelationResultsTable";
import { DualSeriesChart } from "./DualSeriesChart";
import { PriceSparkline } from "./PriceSparkline";

const DEFAULT_REGION_ID = 10000002; // The Forge
const DEFAULT_DAYS = 90 as const;

type Days = 30 | 60 | 90;

export function MarketSignalsTab() {
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<ItemSearchHit[]>([]);
  const [selectedItem, setSelectedItem] = useState<ItemSearchHit | null>(null);
  const [days, setDays] = useState<Days>(DEFAULT_DAYS);
  const [regionId] = useState(DEFAULT_REGION_ID);

  const [response, setResponse] = useState<CorrelationSuggestionsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [selectedCandidate, setSelectedCandidate] = useState<CorrelationSuggestion | null>(null);
  const [pairLoading, setPairLoading] = useState(false);
  const [pairError, setPairError] = useState<string | null>(null);
  const [pairData, setPairData] = useState<{ pricesB: number[]; volumesB: number[] } | null>(null);

  const searchAbort = useRef<AbortController | null>(null);
  const suggestAbort = useRef<AbortController | null>(null);
  const pairAbort = useRef<AbortController | null>(null);

  // Item search
  useEffect(() => {
    if (query.length < 2) { setHits([]); return; }
    if (selectedItem?.type_name === query) { setHits([]); return; }
    searchAbort.current?.abort();
    const ctrl = new AbortController();
    searchAbort.current = ctrl;
    searchItems(query, ctrl.signal)
      .then(setHits)
      .catch(() => {});
    return () => ctrl.abort();
  }, [query, selectedItem]);

  // Fetch suggestions when item or days changes
  const fetchData = useCallback(() => {
    if (!selectedItem) return;
    suggestAbort.current?.abort();
    const ctrl = new AbortController();
    suggestAbort.current = ctrl;
    setLoading(true);
    setError(null);
    setResponse(null);
    setSelectedCandidate(null);
    setPairData(null);
    fetchSuggestions(selectedItem.type_id, regionId, days, ctrl.signal)
      .then((r) => { setResponse(r); setLoading(false); })
      .catch((e: Error) => { if (e.name !== "AbortError") { setError(e.message); setLoading(false); } });
    return () => ctrl.abort();
  }, [selectedItem, regionId, days]);

  useEffect(() => fetchData(), [fetchData]);

  // Fetch pair detail on candidate selection
  useEffect(() => {
    if (!selectedItem || !selectedCandidate) { setPairData(null); return; }
    pairAbort.current?.abort();
    const ctrl = new AbortController();
    pairAbort.current = ctrl;
    setPairLoading(true);
    setPairError(null);
    fetchPair(selectedItem.type_id, selectedCandidate.type_id, regionId, days, ctrl.signal)
      .then((r) => {
        setPairData({ pricesB: r.type_b_prices, volumesB: r.type_b_volumes });
        setPairLoading(false);
      })
      .catch((e: Error) => {
        if (e.name !== "AbortError") { setPairError(e.message); setPairLoading(false); }
      });
    return () => ctrl.abort();
  }, [selectedItem, selectedCandidate, regionId, days]);

  function pickItem(hit: ItemSearchHit) {
    setSelectedItem(hit);
    setQuery(hit.type_name);
    setHits([]);
    setResponse(null);
    setSelectedCandidate(null);
  }

  const priceTrend = response ? trendPct(response.price_series) : null;
  const volTrend   = response ? trendPct(response.volume_series) : null;

  return (
    <div className="flex flex-col gap-3 h-full overflow-y-auto p-2 scrollbar-thin">

      {/* ── Controls ── */}
      <div className="flex items-center gap-3 flex-wrap shrink-0">
        <h2 className="text-sm font-semibold text-eve-accent uppercase tracking-wider">Market Signals</h2>

        {/* Item search */}
        <div className="relative flex-1 min-w-48 max-w-72">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search item…"
            className="w-full bg-eve-input border border-eve-border rounded px-2.5 py-1 text-xs text-eve-text placeholder-eve-dim focus:outline-none focus:border-eve-accent"
          />
          {hits.length > 0 && (
            <ul className="absolute z-50 mt-0.5 w-full bg-eve-panel border border-eve-border rounded shadow-lg max-h-52 overflow-y-auto">
              {hits.map((h) => (
                <li
                  key={h.type_id}
                  className="px-3 py-1.5 text-xs cursor-pointer hover:bg-eve-accent/10 text-eve-text"
                  onClick={() => pickItem(h)}
                >
                  {h.type_name}
                  {h.group_name && <span className="text-eve-dim ml-2">{h.group_name}</span>}
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Days picker */}
        <div className="flex items-center gap-1 text-xs">
          {([30, 60, 90] as Days[]).map((d) => (
            <button
              key={d}
              onClick={() => setDays(d)}
              className={`px-2 py-1 rounded text-[10px] font-semibold uppercase transition-colors ${
                days === d
                  ? "bg-eve-accent text-eve-dark"
                  : "bg-eve-input text-eve-dim hover:text-eve-text border border-eve-border"
              }`}
            >
              {d}d
            </button>
          ))}
        </div>
      </div>

      {/* ── Main layout ── */}
      {selectedItem && (
        <div className="flex gap-3 flex-wrap">

          {/* Primary item panel */}
          <div className="w-52 shrink-0 bg-eve-panel border border-eve-border rounded p-3 flex flex-col gap-2 text-xs">
            <div className="font-semibold text-eve-text">{selectedItem.type_name}</div>
            {response && (
              <>
                <div className="flex items-center gap-2 text-eve-dim">
                  <span>Price</span>
                  <span className={priceTrend !== null && priceTrend >= 0 ? "text-green-400" : "text-red-400"}>
                    {priceTrend !== null ? `${priceTrend >= 0 ? "+" : ""}${priceTrend.toFixed(1)}%` : "—"}
                  </span>
                </div>
                <div className="flex items-center gap-2 text-eve-dim">
                  <span>Vol</span>
                  <span className={volTrend !== null && volTrend >= 0 ? "text-green-400" : "text-red-400"}>
                    {volTrend !== null ? `${volTrend >= 0 ? "+" : ""}${volTrend.toFixed(1)}%` : "—"}
                  </span>
                </div>
                <PriceSparkline data={response.price_series} width={120} height={32} />
                <div className="text-eve-dim/60 text-[10px] mt-1">
                  <div>Source: {response.data_source}</div>
                  <div>Coverage: {response.coverage_days}d</div>
                  {response.excluded_count > 0 && (
                    <div className="text-eve-warning/80">{response.excluded_count} candidates excluded</div>
                  )}
                  {response.ingredient_count === 0 && !loading && (
                    <div className="text-eve-dim mt-1">No blueprint data — ingredient candidates not shown</div>
                  )}
                </div>
              </>
            )}
            {loading && <div className="text-eve-dim/60 text-[10px]">Loading…</div>}
            {error && <div className="text-eve-error text-[10px]">{error}</div>}
          </div>

          {/* Results + detail */}
          <div className="flex-1 min-w-0 flex flex-col gap-3">
            {response && (
              <CorrelationResultsTable
                suggestions={response.suggestions ?? []}
                onSelect={setSelectedCandidate}
                selected={selectedCandidate}
              />
            )}

            {/* Expanded detail */}
            {selectedCandidate && (
              <div className="bg-eve-panel border border-eve-border rounded p-3">
                <div className="text-xs font-semibold text-eve-text mb-2">
                  {selectedItem.type_name} vs {selectedCandidate.type_name}
                  <span className="text-eve-dim font-normal ml-2">
                    r={selectedCandidate.correlation.toFixed(2)}, lag {selectedCandidate.lag_days}d
                  </span>
                </div>
                {pairLoading && <p className="text-eve-dim text-xs">Loading chart…</p>}
                {pairError && <p className="text-eve-error text-xs">{pairError}</p>}
                {pairData && response && (
                  <DualSeriesChart
                    pricesA={response.price_series}
                    pricesB={pairData.pricesB}
                    volumesB={pairData.volumesB}
                    lagDays={selectedCandidate.lag_days}
                    labelA={selectedItem.type_name}
                    labelB={selectedCandidate.type_name}
                  />
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {!selectedItem && (
        <p className="text-eve-dim text-xs mt-8 text-center">
          Search for an item above to explore market correlations.
        </p>
      )}
    </div>
  );
}

function trendPct(series: number[]): number | null {
  if (series.length < 2) return null;
  const w = Math.min(7, series.length);
  const start = series[series.length - w];
  const end = series[series.length - 1];
  if (start === 0) return null;
  return (end - start) / start * 100;
}
