export type OpportunityStatus = "NOT_MOVED_YET" | "FOLLOWING" | "ALREADY_MOVED" | "NO_SIGNAL";
export type CandidateSet = "ingredient" | "rig" | "module" | "shared_mat";

export interface CorrelationSuggestion {
  type_id: number;
  type_name: string;
  candidate_set: CandidateSet;
  correlation: number;
  lag_days: number;
  primary_moved: boolean;
  secondary_moved: boolean;
  opportunity: boolean;
  opportunity_status: OpportunityStatus;
  price_trend_7d: number;
  volume_trend_7d: number;
  data_source: string;
  price_series: number[];
  volume_series: number[];
}

export interface CorrelationSuggestionsResponse {
  type_id: number;
  type_name: string;
  price_series: number[];
  volume_series: number[];
  data_source: string;
  coverage_days: number;
  excluded_count: number;
  ingredient_count: number;
  suggestions: CorrelationSuggestion[];
}

export interface CorrelationPairResponse {
  type_a: number;
  type_a_name: string;
  type_a_prices: number[];
  type_a_volumes: number[];
  type_a_source: string;
  type_b: number;
  type_b_name: string;
  type_b_prices: number[];
  type_b_volumes: number[];
  type_b_source: string;
  correlation: number;
  lag_days: number;
}

export interface ItemSearchHit {
  type_id: number;
  type_name: string;
  group_name?: string;
}
