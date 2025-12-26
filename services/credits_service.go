// ============================================
// FILE: backend/services/credits_service.go
// REPLACE the entire file with proper sorting
// ============================================
package services

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"xand/models"
)

type CreditsService struct {
	httpClient    *http.Client
	credits       map[string]*models.PodCredits // Key: pubkey
	creditsMutex  sync.RWMutex
	stopChan      chan struct{}
	apiEndpoint   string
}

func NewCreditsService() *CreditsService {
	return &CreditsService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		credits:     make(map[string]*models.PodCredits),
		stopChan:    make(chan struct{}),
		apiEndpoint: "https://podcredits.xandeum.network/api/pods-credits",
	}
}

func (cs *CreditsService) Start() {
	log.Println("Starting Pod Credits Service (updates every 30 seconds)...")
	
	cs.fetchCredits() // Initial fetch
	
	ticker := time.NewTicker(30 * time.Second)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				cs.fetchCredits()
			case <-cs.stopChan:
				ticker.Stop()
				log.Println("Pod Credits Service stopped")
				return
			}
		}
	}()
}

func (cs *CreditsService) Stop() {
	close(cs.stopChan)
}

func (cs *CreditsService) fetchCredits() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", cs.apiEndpoint, nil)
	if err != nil {
		log.Printf("Error creating credits request: %v", err)
		return
	}

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		log.Printf("Error fetching pod credits: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Pod credits API returned status %d", resp.StatusCode)
		return
	}

	var creditsResp models.PodCreditsResponse
	if err := json.NewDecoder(resp.Body).Decode(&creditsResp); err != nil {
		log.Printf("Error decoding credits response: %v", err)
		return
	}

	if creditsResp.Status != "success" {
		log.Printf("Pod credits API returned non-success status: %s", creditsResp.Status)
		return
	}

	// CRITICAL FIX: Sort by credits descending (API might not be sorted)
	entries := creditsResp.PodsCredits
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Credits > entries[j].Credits
	})

	// Update credits map with SORTED data
	cs.creditsMutex.Lock()
	defer cs.creditsMutex.Unlock()

	newCredits := make(map[string]*models.PodCredits)
	
	for i, entry := range entries {
		pubkey := entry.PodID // Use pod_id as the key/pubkey

		oldCredits := int64(0)
		if existing, exists := cs.credits[pubkey]; exists {
			oldCredits = existing.Credits
		}

		newCredits[pubkey] = &models.PodCredits{
			Pubkey:        pubkey,
			Credits:       entry.Credits,
			LastUpdated:   time.Now(),
			Rank:          i + 1, // NOW guaranteed to be correct (sorted)
			CreditsChange: entry.Credits - oldCredits,
		}
	}

	cs.credits = newCredits
	
	// Calculate statistics
	totalCredits := int64(0)
	for _, c := range newCredits {
		totalCredits += c.Credits
	}
	avgCredits := int64(0)
	if len(newCredits) > 0 {
		avgCredits = totalCredits / int64(len(newCredits))
	}
	
	log.Printf("Updated pod credits: %d nodes tracked, avg credits: %d", len(cs.credits), avgCredits)
}

// GetCredits returns credits for a specific pubkey/pod_id
func (cs *CreditsService) GetCredits(pubkey string) (*models.PodCredits, bool) {
	if pubkey == "" {
		return nil, false
	}
	
	cs.creditsMutex.RLock()
	defer cs.creditsMutex.RUnlock()
	
	credits, exists := cs.credits[pubkey]
	return credits, exists
}

// GetAllCredits returns a sorted copy of all pod credits
func (cs *CreditsService) GetAllCredits() []*models.PodCredits {
	cs.creditsMutex.RLock()
	defer cs.creditsMutex.RUnlock()
	
	result := make([]*models.PodCredits, 0, len(cs.credits))
	for _, c := range cs.credits {
		result = append(result, c)
	}
	
	// Sort by rank (which is already calculated correctly)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Rank < result[j].Rank
	})
	
	return result
}

// GetTopCredits returns top N nodes by credits
func (cs *CreditsService) GetTopCredits(limit int) []*models.PodCredits {
	cs.creditsMutex.RLock()
	defer cs.creditsMutex.RUnlock()
	
	// Collect all credits
	all := make([]*models.PodCredits, 0, len(cs.credits))
	for _, c := range cs.credits {
		all = append(all, c)
	}
	
	// Sort by credits descending
	sort.Slice(all, func(i, j int) bool {
		return all[i].Credits > all[j].Credits
	})
	
	// Return top N
	if limit > len(all) {
		limit = len(all)
	}
	
	return all[:limit]
}

// GetCreditsByRank returns credits for nodes in a specific rank range
func (cs *CreditsService) GetCreditsByRank(startRank, endRank int) []*models.PodCredits {
	cs.creditsMutex.RLock()
	defer cs.creditsMutex.RUnlock()
	
	result := make([]*models.PodCredits, 0)
	for _, c := range cs.credits {
		if c.Rank >= startRank && c.Rank <= endRank {
			result = append(result, c)
		}
	}
	
	// Sort by rank
	sort.Slice(result, func(i, j int) bool {
		return result[i].Rank < result[j].Rank
	})
	
	return result
}

// GetCreditsStats returns statistics about the credits distribution
func (cs *CreditsService) GetCreditsStats() map[string]interface{} {
	cs.creditsMutex.RLock()
	defer cs.creditsMutex.RUnlock()
	
	if len(cs.credits) == 0 {
		return map[string]interface{}{
			"total_nodes": 0,
			"error":       "no credits data available",
		}
	}
	
	var totalCredits int64
	var maxCredits int64
	var minCredits int64 = 9999999999
	
	for _, c := range cs.credits {
		totalCredits += c.Credits
		if c.Credits > maxCredits {
			maxCredits = c.Credits
		}
		if c.Credits < minCredits {
			minCredits = c.Credits
		}
	}
	
	avgCredits := totalCredits / int64(len(cs.credits))
	
	// Calculate median
	sorted := make([]*models.PodCredits, 0, len(cs.credits))
	for _, c := range cs.credits {
		sorted = append(sorted, c)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Credits < sorted[j].Credits
	})
	medianCredits := sorted[len(sorted)/2].Credits
	
	return map[string]interface{}{
		"total_nodes":    len(cs.credits),
		"total_credits":  totalCredits,
		"average_credits": avgCredits,
		"median_credits":  medianCredits,
		"max_credits":     maxCredits,
		"min_credits":     minCredits,
	}
}