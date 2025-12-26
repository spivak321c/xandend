package services

import (
	"xand/models"
)

type CalculatorService struct{}

func NewCalculatorService() *CalculatorService {
	return &CalculatorService{}
}

// CompareCosts compares storage costs across providers
func (cs *CalculatorService) CompareCosts(storageTB float64) models.StorageCostComparison {
	comparison := models.StorageCostComparison{
		StorageAmountTB: storageTB,
		Duration:        "monthly",
		Providers:       make([]models.ProviderCostBreakdown, 0),
	}

	// Xandeum (Pricing pending real-time feed)
	xandeum := models.ProviderCostBreakdown{
		Name:           "Xandeum",
		MonthlyCostUSD: 0,
		YearlyCostUSD:  0,
		Features:       []string{"Decentralized", "Solana-native", "Erasure coded", "High durability"},
		Notes:          "Pricing pending live network data",
	}

	// AWS S3 Standard (as of 2024)
	awsS3 := models.ProviderCostBreakdown{
		Name:           "AWS S3",
		MonthlyCostUSD: storageTB * 23.0, // ~$0.023/GB = $23/TB
		YearlyCostUSD:  storageTB * 23.0 * 12,
		Features:       []string{"Centralized", "99.999999999% durability", "Global CDN", "Instant access"},
		Notes:          "Standard tier, excludes data transfer & API costs",
	}

	// Arweave (Permanent storage)
	arweave := models.ProviderCostBreakdown{
		Name:           "Arweave",
		MonthlyCostUSD: storageTB * 1000 / 240, // ~$1000/TB one-time / 240 months (20 years)
		YearlyCostUSD:  storageTB * 1000 / 20,
		Features:       []string{"Permanent storage", "Decentralized", "Pay once", "Blockchain-based"},
		Notes:          "One-time payment model, amortized over 20 years",
	}

	// Filecoin
	filecoin := models.ProviderCostBreakdown{
		Name:           "Filecoin",
		MonthlyCostUSD: storageTB * 1.5, // Variable, ~$1.5/TB/month average
		YearlyCostUSD:  storageTB * 1.5 * 12,
		Features:       []string{"Decentralized", "Proof-of-storage", "Market-driven pricing", "Retrieval fees apply"},
		Notes:          "Pricing varies by storage provider and deal terms",
	}

	comparison.Providers = []models.ProviderCostBreakdown{xandeum, awsS3, arweave, filecoin}

	// Determine recommendation
	if storageTB < 10 {
		comparison.Recommendation = "For small storage needs, Xandeum offers competitive pricing with decentralization benefits."
	} else if storageTB < 100 {
		comparison.Recommendation = "Xandeum provides significant cost savings compared to AWS S3 at this scale."
	} else {
		comparison.Recommendation = "At enterprise scale, Xandeum's decentralized model offers both cost efficiency and censorship resistance."
	}

	return comparison
}

// EstimateROI calculates earnings for running a pNode
func (cs *CalculatorService) EstimateROI(storageTB float64, uptimePercent float64) models.ROIEstimate {
	// XAND token price (pending real-time feed)
	xandPrice := 0.0

	// Reward structure (pending governance/network parameters)
	baseRewardPerTBPerDay := 0.0

	// Adjust for uptime
	actualRewardPerTBPerDay := baseRewardPerTBPerDay * (uptimePercent / 100.0)

	// Calculate monthly/yearly
	monthlyXAND := storageTB * actualRewardPerTBPerDay * 30
	yearlyXAND := monthlyXAND * 12

	monthlyUSD := monthlyXAND * xandPrice
	yearlyUSD := yearlyXAND * xandPrice

	// Estimate costs (hardware + electricity)
	// Assuming: 1TB requires ~$5/month for storage + power
	monthlyCosts := storageTB * 5.0

	estimate := models.ROIEstimate{
		StorageCommitmentTB: storageTB,
		UptimePercent:       uptimePercent,
		MonthlyXAND:         monthlyXAND,
		MonthlyUSD:          monthlyUSD,
		YearlyXAND:          yearlyXAND,
		YearlyUSD:           yearlyUSD,
		MonthlyCostsUSD:     monthlyCosts,
		NetProfitMonthly:    monthlyUSD - monthlyCosts,
		XANDPriceUSD:        xandPrice,
		RewardPerTBPerDay:   actualRewardPerTBPerDay,
	}

	// Calculate break-even (initial hardware investment)
	// Assuming $100/TB for hardware
	initialInvestment := storageTB * 100.0
	if estimate.NetProfitMonthly > 0 {
		estimate.BreakEvenMonths = int(initialInvestment / estimate.NetProfitMonthly)
	} else {
		estimate.BreakEvenMonths = -1 // Never breaks even
	}

	return estimate
}

// SimulateRedundancy demonstrates erasure coding
func (cs *CalculatorService) SimulateRedundancy(failedNodes []int) models.RedundancySimulation {
	sim := models.RedundancySimulation{
		DataShards:     4,
		ParityShards:   2,
		TotalShards:    6,
		FailedNodes:    failedNodes,
		RequiredShards: 4, // Need at least 4 shards to reconstruct
	}

	sim.AvailableShards = sim.TotalShards - len(failedNodes)
	sim.CanRecover = sim.AvailableShards >= sim.RequiredShards

	if sim.CanRecover {
		if len(failedNodes) == 0 {
			sim.Message = "All shards available. Data is fully accessible with optimal redundancy."
		} else if len(failedNodes) == 1 {
			sim.Message = "1 shard lost. Data can be fully reconstructed from remaining 5 shards. Redundancy maintained."
		} else if len(failedNodes) == 2 {
			sim.Message = "2 shards lost. Data can still be reconstructed from 4 remaining shards. Minimum redundancy threshold reached."
		}
	} else {
		sim.Message = "CRITICAL: Too many shards lost. Data cannot be fully reconstructed. Minimum 4 shards required."
	}

	return sim
}
