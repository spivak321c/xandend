package models

import "time"

// PodCredits represents the reputation/reliability score for a pNode
type PodCredits struct {
	Pubkey         string    `json:"pubkey" bson:"pubkey"`
	Credits        int64     `json:"credits" bson:"credits"`
	LastUpdated    time.Time `json:"last_updated" bson:"last_updated"`
	Rank           int       `json:"rank,omitempty" bson:"rank,omitempty"`
	CreditsChange  int64     `json:"credits_change,omitempty" bson:"credits_change,omitempty"` // Change since last check
}

// PodCreditsResponse from the API
type PodCreditsResponse struct {
	PodsCredits []PodCreditsEntry `json:"pods_credits"`
	Status      string            `json:"status"`
}

type PodCreditsEntry struct {
	PodID   string `json:"pod_id"`
	Credits int64  `json:"credits"`
}