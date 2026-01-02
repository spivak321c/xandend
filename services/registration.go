// package services

// import (
// 	"encoding/csv"
// 	"fmt"
// 	"log"
// 	"os"
// 	"strings"
// 	"sync"
// )

// type RegistrationService struct {
// 	registeredPubkeys map[string]bool
// 	mutex             sync.RWMutex
// 	csvPath           string
// }

// // NewRegistrationService creates a new registration service from a CSV file
// // CSV format: single column with pubkey per line (optional header)
// func NewRegistrationService(csvPath string) (*RegistrationService, error) {
// 	rs := &RegistrationService{
// 		registeredPubkeys: make(map[string]bool),
// 		csvPath:           csvPath,
// 	}
	
// 	if csvPath == "" {
// 		log.Println("No registration CSV provided, all nodes will be unregistered")
// 		return rs, nil
// 	}
	
// 	if err := rs.loadCSV(); err != nil {
// 		return nil, fmt.Errorf("failed to load registration CSV: %w", err)
// 	}
	
// 	log.Printf("✓ Registration service loaded %d registered pubkeys from %s", 
// 		len(rs.registeredPubkeys), csvPath)
	
// 	return rs, nil
// }

// func (rs *RegistrationService) loadCSV() error {
// 	file, err := os.Open(rs.csvPath)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()
	
// 	reader := csv.NewReader(file)
// 	reader.TrimLeadingSpace = true
	
// 	records, err := reader.ReadAll()
// 	if err != nil {
// 		return err
// 	}
	
// 	rs.mutex.Lock()
// 	defer rs.mutex.Unlock()
	
// 	for i, record := range records {
// 		// Skip empty lines
// 		if len(record) == 0 {
// 			continue
// 		}
		
// 		pubkey := strings.TrimSpace(record[0])
		
// 		// Skip header (if first line looks like "pubkey" or "public_key")
// 		if i == 0 && (strings.ToLower(pubkey) == "pubkey" || 
// 			strings.ToLower(pubkey) == "public_key" ||
// 			strings.ToLower(pubkey) == "pod_id") {
// 			continue
// 		}
		
// 		// Skip empty pubkeys
// 		if pubkey == "" {
// 			continue
// 		}
		
// 		rs.registeredPubkeys[pubkey] = true
// 	}
	
// 	return nil
// }

// // IsRegistered checks if a pubkey is in the registration list
// func (rs *RegistrationService) IsRegistered(pubkey string) bool {
// 	if pubkey == "" {
// 		return false
// 	}
	
// 	rs.mutex.RLock()
// 	defer rs.mutex.RUnlock()
	
// 	return rs.registeredPubkeys[pubkey]
// }

// // GetRegisteredCount returns the total number of registered pubkeys
// func (rs *RegistrationService) GetRegisteredCount() int {
// 	rs.mutex.RLock()
// 	defer rs.mutex.RUnlock()
	
// 	return len(rs.registeredPubkeys)
// }

// // Reload reloads the CSV file (useful for updates without restart)
// func (rs *RegistrationService) Reload() error {
// 	if rs.csvPath == "" {
// 		return nil
// 	}
	
// 	rs.mutex.Lock()
// 	rs.registeredPubkeys = make(map[string]bool)
// 	rs.mutex.Unlock()
	
// 	return rs.loadCSV()
// }


package services

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RegistrationInfo holds registration details for a pubkey
type RegistrationInfo struct {
	Pubkey         string
	RegisteredTime time.Time
}

type RegistrationService struct {
	registeredNodes map[string]*RegistrationInfo // map[pubkey]info
	mutex           sync.RWMutex
	csvPath         string
}

// NewRegistrationService creates a new registration service from a CSV file
// CSV format: Index, pNode Identity Pubkey, Manager, Registered Time, Version
func NewRegistrationService(csvPath string) (*RegistrationService, error) {
	rs := &RegistrationService{
		registeredNodes: make(map[string]*RegistrationInfo),
		csvPath:         csvPath,
	}
	
	if csvPath == "" {
		log.Println("No registration CSV provided, all nodes will be unregistered")
		return rs, nil
	}
	
	if err := rs.loadCSV(); err != nil {
		return nil, fmt.Errorf("failed to load registration CSV: %w", err)
	}
	
	log.Printf("✓ Registration service loaded %d registered pubkeys from %s", 
		len(rs.registeredNodes), csvPath)
	
	return rs, nil
}

func (rs *RegistrationService) loadCSV() error {
	file, err := os.Open(rs.csvPath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	// Regex to extract the data we need
	// Matches: any_number, "pubkey", "manager", "timestamp", version
	pattern := regexp.MustCompile(`\s*\d+\s*,\s*"([^"]+)"\s*,\s*"[^"]+"\s*,\s*"([^"]+)"\s*,`)
	
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Skip header (contains "Index" or "pNode")
		if lineNum == 1 && (strings.Contains(line, "Index") || strings.Contains(line, "pNode")) {
			continue
		}
		
		// Try to match the pattern
		matches := pattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			log.Printf("Skipping line %d: couldn't parse format", lineNum)
			continue
		}
		
		// Extract pubkey (first capture group)
		pubkey := strings.TrimSpace(matches[1])
		if pubkey == "" {
			continue
		}
		
		// Extract registered time (second capture group)
		registeredTimeStr := strings.TrimSpace(matches[2])
		
		// Parse the time
		var registeredTime time.Time
		formats := []string{
			"1/2/2006, 3:04:05 PM",  // M/D/YYYY, H:MM:SS AM/PM
			"1/2/2006 3:04:05 PM",   // Without comma
			"1/2/2006, 15:04:05",    // 24-hour format with comma
			"1/2/2006 15:04:05",     // 24-hour format without comma
			time.RFC3339,
		}
		
		parsed := false
		for _, format := range formats {
			if t, err := time.Parse(format, registeredTimeStr); err == nil {
				registeredTime = t
				parsed = true
				break
			}
		}
		
		if !parsed {
			log.Printf("Warning: Could not parse time '%s' for pubkey %s (line %d), using zero time", 
				registeredTimeStr, pubkey, lineNum)
			registeredTime = time.Time{}
		}
		
		rs.registeredNodes[pubkey] = &RegistrationInfo{
			Pubkey:         pubkey,
			RegisteredTime: registeredTime,
		}
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}
	
	return nil
}

// IsRegistered checks if a pubkey is in the registration list
func (rs *RegistrationService) IsRegistered(pubkey string) bool {
	if pubkey == "" {
		return false
	}
	
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	_, exists := rs.registeredNodes[pubkey]
	return exists
}

// GetRegistrationInfo returns the registration info for a pubkey
func (rs *RegistrationService) GetRegistrationInfo(pubkey string) (*RegistrationInfo, bool) {
	if pubkey == "" {
		return nil, false
	}
	
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	info, exists := rs.registeredNodes[pubkey]
	return info, exists
}

// GetAllRegistrations returns all registered nodes
func (rs *RegistrationService) GetAllRegistrations() []*RegistrationInfo {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	result := make([]*RegistrationInfo, 0, len(rs.registeredNodes))
	for _, info := range rs.registeredNodes {
		result = append(result, info)
	}
	
	return result
}

// GetRegisteredCount returns the total number of registered pubkeys
func (rs *RegistrationService) GetRegisteredCount() int {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	return len(rs.registeredNodes)
}

// Reload reloads the CSV file (useful for updates without restart)
func (rs *RegistrationService) Reload() error {
	if rs.csvPath == "" {
		return nil
	}
	
	rs.mutex.Lock()
	rs.registeredNodes = make(map[string]*RegistrationInfo)
	rs.mutex.Unlock()
	
	return rs.loadCSV()
}