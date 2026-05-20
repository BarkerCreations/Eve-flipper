// Standalone fetch module — intentionally does not import from lib/api.ts
// to minimise rebase surface on this fork.

import type { CorrelationSuggestionsResponse, CorrelationPairResponse, ItemSearchHit } from "./types";

const BASE = (import.meta.env.VITE_API_URL as string | undefined)?.trim() ?? "";
const UID_HEADER = "X-EveFlipper-UID";
const UID_KEY = "eveflipper_uid_v1";

function uid(): string {
  try { return window.localStorage.getItem(UID_KEY) ?? "eveflipper_desktop"; } catch { return "eveflipper_desktop"; }
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await window.fetch(`${BASE}${path}`, {
    credentials: "include",
    headers: { [UID_HEADER]: uid() },
    signal,
  });
  if (!res.ok) {
    const err = await res.json().catch((e) => { if (signal?.aborted) throw e; return {}; }) as { error?: string };
    throw new Error(err.error ?? `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export function fetchSuggestions(
  typeId: number,
  regionId: number,
  days: 30 | 60 | 90,
  signal?: AbortSignal,
): Promise<CorrelationSuggestionsResponse> {
  return get(`/api/correlation/suggestions?type_id=${typeId}&region_id=${regionId}&days=${days}`, signal);
}

export function fetchPair(
  typeA: number,
  typeB: number,
  regionId: number,
  days: 30 | 60 | 90,
  signal?: AbortSignal,
): Promise<CorrelationPairResponse> {
  return get(`/api/correlation/pair?type_a=${typeA}&type_b=${typeB}&region_id=${regionId}&days=${days}`, signal);
}

export function searchItems(q: string, signal?: AbortSignal): Promise<ItemSearchHit[]> {
  return get(`/api/items/search?q=${encodeURIComponent(q)}&limit=10`, signal);
}
