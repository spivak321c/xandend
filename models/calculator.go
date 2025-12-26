package models

// StorageCostComparison compares costs across providers
type StorageCostComparison struct {
	StorageAmountTB float64              `json:"storage_amount_tb"`
	Duration        string               `json:"duration"` // "monthly", "yearly"
	Providers       []ProviderCostBreakdown `json:"providers"`
	Recommendation  string               `json:"recommendation"`
}

// ProviderCostBreakdown for individual storage providers
type ProviderCostBreakdown struct {
	Name           string  `json:"name"`     // "Xandeum", "AWS S3", "Arweave", "Filecoin"
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
	YearlyCostUSD  float64 `json:"yearly_cost_usd"`
	Features       []string `json:"features"`
	Notes          string  `json:"notes"`
}

// ROIEstimate calculates earnings for running a pNode
type ROIEstimate struct {
	StorageCommitmentTB float64 `json:"storage_commitment_tb"`
	UptimePercent       float64 `json:"uptime_percent"`
	
	// Earnings
	MonthlyXAND        float64 `json:"monthly_xand"`
	MonthlyUSD         float64 `json:"monthly_usd"`
	YearlyXAND         float64 `json:"yearly_xand"`
	YearlyUSD          float64 `json:"yearly_usd"`
	
	// Costs (estimated)
	MonthlyCostsUSD    float64 `json:"monthly_costs_usd"` // Hardware, electricity
	NetProfitMonthly   float64 `json:"net_profit_monthly"`
	BreakEvenMonths    int     `json:"break_even_months"`
	
	// Assumptions
	XANDPriceUSD       float64 `json:"xand_price_usd"`
	RewardPerTBPerDay  float64 `json:"reward_per_tb_per_day"`
}

// RedundancySimulation for erasure coding demo
type RedundancySimulation struct {
	DataShards      int      `json:"data_shards"`      // 4
	ParityShards    int      `json:"parity_shards"`    // 2
	TotalShards     int      `json:"total_shards"`     // 6
	FailedNodes     []int    `json:"failed_nodes"`     // [2, 4]
	CanRecover      bool     `json:"can_recover"`
	RequiredShards  int      `json:"required_shards"`  // 4
	AvailableShards int      `json:"available_shards"` // 4
	Message         string   `json:"message"`
}