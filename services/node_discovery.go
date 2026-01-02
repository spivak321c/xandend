package services

import (
	"log"
	"net"
	"strconv"
	"sync"
	"time"
	"sort"

	"xand/config"
	"xand/models"
	"xand/utils"
)

type NodeDiscovery struct {
	cfg     *config.Config
	prpc    *PRPCClient
	geo     *utils.GeoResolver
	credits *CreditsService
	registration *RegistrationService // NEW

	// CHANGED: Now uses composite key "pubkey|ip" or "unknown|ip"
	knownNodes map[string]*models.Node
	nodesMutex sync.RWMutex

	// NEW: Track ALL nodes including duplicates (by IP)
	allNodesByIP map[string]*models.Node // Key: IP address
	allNodesMutex sync.RWMutex

	// NEW: Index for looking up all nodes with a given pubkey
	// pubkeyToNodes map[string][]*models.Node
	// pubkeyMutex   sync.RWMutex

	// Track IP->nodes for reverse lookup (KEPT for compatibility)
	ipToNodes map[string][]*models.Node
	ipMutex   sync.RWMutex

	// Track failed addresses to avoid retry spam
	failedAddresses map[string]time.Time
	failedMutex     sync.RWMutex

	stopChan    chan struct{}
	rateLimiter chan struct{}
}



func NewNodeDiscovery(cfg *config.Config, prpc *PRPCClient, geo *utils.GeoResolver, 
	credits *CreditsService, registration *RegistrationService) *NodeDiscovery {
	return &NodeDiscovery{
		cfg:             cfg,
		prpc:            prpc,
		geo:             geo,
		credits:         credits,
		registration:    registration,
		knownNodes:      make(map[string]*models.Node),
		allNodesByIP:    make(map[string]*models.Node),
		//pubkeyToNodes:   make(map[string][]*models.Node),
		ipToNodes:       make(map[string][]*models.Node),
		failedAddresses: make(map[string]time.Time),
		stopChan:        make(chan struct{}),
		rateLimiter:     make(chan struct{}, 50),
	}
}

func (nd *NodeDiscovery) Start() {
	go nd.Bootstrap()
	go nd.runDiscoveryLoop()
	go nd.runStatsLoop()
	go nd.runHealthLoop()
	go nd.runCleanupLoop()
}

func (nd *NodeDiscovery) Stop() {
	close(nd.stopChan)
}

func (nd *NodeDiscovery) runCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			nd.cleanupFailedAddresses()
		case <-nd.stopChan:
			return
		}
	}
}

func (nd *NodeDiscovery) cleanupFailedAddresses() {
	nd.failedMutex.Lock()
	defer nd.failedMutex.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	cleaned := 0

	for addr, lastFailed := range nd.failedAddresses {
		if lastFailed.Before(cutoff) {
			delete(nd.failedAddresses, addr)
			cleaned++
		}
	}

	if cleaned > 0 {
		log.Printf("Cleaned up %d old failed addresses (total: %d)", cleaned, len(nd.failedAddresses))
	}
}

func (nd *NodeDiscovery) runDiscoveryLoop() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	
	discoveryCount := 0
	
	for {
		select {
		case <-ticker.C:
			discoveryCount++
			
			nd.nodesMutex.RLock()
			totalNodes := len(nd.knownNodes)
			nd.nodesMutex.RUnlock()
			
			log.Printf("Discovery cycle #%d (total nodes: %d)", discoveryCount, totalNodes)
			
			nd.discoverPeers()
			
			if discoveryCount == 5 {
				ticker.Stop()
				configInterval := time.Duration(nd.cfg.Polling.DiscoveryInterval) * time.Second
				ticker = time.NewTicker(configInterval)
				log.Printf("Switching to normal discovery interval: %v", configInterval)
			}
			
		case <-nd.stopChan:
			return
		}
	}
}

func (nd *NodeDiscovery) runStatsLoop() {
	ticker := time.NewTicker(time.Duration(nd.cfg.Polling.StatsInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			nd.collectStats()
		case <-nd.stopChan:
			return
		}
	}
}




func (nd *NodeDiscovery) runHealthLoop() {
	ticker := time.NewTicker(time.Duration(nd.cfg.Polling.HealthCheckInterval) * time.Second)
	defer ticker.Stop()
	
	var healthCheckRunning sync.Mutex  // NEW: Prevent overlapping health checks
	
	for {
		select {
		case <-ticker.C:
			// Try to acquire lock (non-blocking)
			if !healthCheckRunning.TryLock() {
				log.Println("⚠️  Skipping health check - previous check still running")
				continue
			}
			
			// Run health check in goroutine but hold lock
			go func() {
				defer healthCheckRunning.Unlock()
				nd.healthCheckOptimized()
			}()
			
		case <-nd.stopChan:
			return
		}
	}
}



func (nd *NodeDiscovery) Bootstrap() {
	log.Println("Starting optimized Bootstrap (1s gossip propagation)...")
	
	// Seed discovery
	var wg sync.WaitGroup
	for _, seed := range nd.cfg.Server.SeedNodes {
		wg.Add(1)
		go func(seedAddr string) {
			defer wg.Done()
			nd.processNodeAddress(seedAddr)
		}(seed)
		time.Sleep(200 * time.Millisecond)
	}
	wg.Wait()
	
	// With 1s gossip, one strategic discovery is enough
	log.Println("Strategic peer discovery (single round with fast gossip)...")
	nd.discoverPeersStrategic()
	
	// Quick validation of subset
	log.Println("Quick validation of discovered nodes...")
	nd.quickValidation()
	
	// Give gossip time to propagate (2 min = 120 gossip cycles)
	log.Println("Waiting for gossip propagation (30 seconds)...")
	time.Sleep(30 * time.Second)
	
	// One more discovery to catch stragglers
	log.Println("Final discovery sweep...")
	nd.discoverPeers()
	
	log.Printf("Bootstrap complete. Nodes discovered: %d", len(nd.knownNodes))
}



// 2. STRATEGIC PEER DISCOVERY - Query fewer, smarter nodes
func (nd *NodeDiscovery) discoverPeersStrategic() {
	nodes := nd.GetNodes()
	
	// Select diverse, high-quality nodes
	candidates := nd.selectStrategicNodes(nodes, 5) // Only query 5 best nodes
	
	log.Printf("Querying %d strategic nodes for complete peer list", len(candidates))
	
	var wg sync.WaitGroup
	for _, node := range candidates {
		wg.Add(1)
		go func(n *models.Node) {
			defer wg.Done()
			nd.discoverPeersFromNode(n.Address)
		}(node)
		time.Sleep(300 * time.Millisecond)
	}
	wg.Wait()
	
	totalIPs, uniquePubkeys := nd.GetNodeCounts()
	log.Printf("Strategic discovery complete. IPs: %d, Pubkeys: %d", totalIPs, uniquePubkeys)
}








func (nd *NodeDiscovery) selectStrategicNodes(nodes []*models.Node, count int) []*models.Node {
	// Filter online nodes with good track record
	candidates := make([]*models.Node, 0)
	for _, n := range nodes {
		if n.IsOnline && n.UptimeScore > 80 && n.Status == "online" {
			candidates = append(candidates, n)
		}
	}
	
	if len(candidates) <= count {
		return candidates
	}
	
	// Diversify by country
	countryMap := make(map[string]*models.Node)
	for _, n := range candidates {
		if _, exists := countryMap[n.Country]; !exists {
			countryMap[n.Country] = n
		}
	}
	
	result := make([]*models.Node, 0, count)
	for _, n := range countryMap {
		result = append(result, n)
		if len(result) >= count {
			break
		}
	}
	
	// Fill remaining with best performers
	for _, n := range candidates {
		if len(result) >= count {
			break
		}
		found := false
		for _, existing := range result {
			if existing.ID == n.ID {
				found = true
				break
			}
		}
		if !found {
			result = append(result, n)
		}
	}
	
	return result
}

// 3. QUICK VALIDATION - Fast connectivity check on subset
func (nd *NodeDiscovery) quickValidation() {
	nodes := nd.GetNodes()
	
	// Only validate 20% of nodes, prioritize recently seen
	sampleSize := len(nodes) / 5
	if sampleSize > 50 {
		sampleSize = 50
	}
	if sampleSize < 10 {
		sampleSize = len(nodes)
	}
	
	// Sort by last seen, take most recent
	type nodePriority struct {
		node     *models.Node
		priority time.Duration
	}
	
	priorities := make([]nodePriority, len(nodes))
	for i, n := range nodes {
		priorities[i] = nodePriority{
			node:     n,
			priority: time.Since(n.LastSeen),
		}
	}
	
	// Sort by most recent first
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].priority < priorities[j].priority
	})
	
	toValidate := make([]*models.Node, 0, sampleSize)
	for i := 0; i < sampleSize && i < len(priorities); i++ {
		toValidate = append(toValidate, priorities[i].node)
	}
	
	log.Printf("Quick validation of %d nodes...", len(toValidate))
	
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	
	for _, node := range toValidate {
		wg.Add(1)
		go func(n *models.Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			
			rpcAddr := nd.getRPCAddress(n)
			verResp, err := nd.prpc.GetVersion(rpcAddr)
			
			nd.nodesMutex.Lock()
			if stored, exists := nd.knownNodes[n.ID]; exists {
				updateCallHistory(stored, err == nil)
				if err == nil {
					stored.LastSeen = time.Now()
					stored.Version = verResp.Version
				}
				utils.CalculateScore(stored)
				utils.DetermineStatus(stored)
			}
			nd.nodesMutex.Unlock()
		}(node)
	}
	
	wg.Wait()
}




func (nd *NodeDiscovery) healthCheckOptimized() {
	nodes := nd.GetNodes()
	
	// With 1s gossip, prioritize nodes that SHOULD be fresh but aren't responding
	// This helps identify truly offline nodes faster
	
	// Separate into priority groups
	highPriority := make([]*models.Node, 0)  // Public nodes or nodes with stale gossip
	lowPriority := make([]*models.Node, 0)   // Private nodes with recent gossip
	
	for _, node := range nodes {
		lastSeen := time.Since(node.LastSeen)
		
		// High priority: Check public nodes or nodes approaching stale threshold
		if node.IsPublic || lastSeen > 3*time.Minute {
			highPriority = append(highPriority, node)
		} else if lastSeen > 1*time.Minute {
			// Medium priority: Private nodes with slightly old gossip
			lowPriority = append(lowPriority, node)
		}
		// Skip: Private nodes with very recent gossip (<1 min) - trust gossip data
	}
	
	// Combine priority groups
	nodesToCheck := append(highPriority, lowPriority...)
	
	batchSize := 10
	delay := 200 * time.Millisecond
	
	start := time.Now()
	log.Printf("Starting health check (%d nodes: %d high priority, %d low priority, %d skipped)...", 
		len(nodesToCheck), len(highPriority), len(lowPriority), len(nodes)-len(nodesToCheck))
	
	successCount := 0
	failureCount := 0
	timeoutCount := 0
	var resultMutex sync.Mutex
	
	// Process in batches
	for i := 0; i < len(nodesToCheck); i += batchSize {
		end := i + batchSize
		if end > len(nodesToCheck) {
			end = len(nodesToCheck)
		}
		
		batch := nodesToCheck[i:end]
		var wg sync.WaitGroup
		
		for _, node := range batch {
			wg.Add(1)
			go func(n *models.Node) {
				defer wg.Done()
				
				checkStart := time.Now()
				rpcAddr := nd.getRPCAddress(n)
				verResp, err := nd.prpc.GetVersion(rpcAddr)
				latency := time.Since(checkStart).Milliseconds()
				
				nd.nodesMutex.Lock()
				if stored, exists := nd.knownNodes[n.ID]; exists {
					stored.ResponseTime = latency
					updateCallHistory(stored, err == nil)
					stored.TotalCalls++
					
					if err == nil {
						stored.SuccessCalls++
						stored.LastSeen = time.Now()
						stored.Version = verResp.Version
						
						// Mark RPC address as working
						for i := range stored.Addresses {
							if stored.Addresses[i].Type == "rpc" && stored.Addresses[i].Address == rpcAddr {
								stored.Addresses[i].IsWorking = true
								stored.Addresses[i].LastSeen = time.Now()
								break
							}
						}
						
						resultMutex.Lock()
						successCount++
						resultMutex.Unlock()
					} else {
						// Check if timeout
						if latency > 9000 {
							resultMutex.Lock()
							timeoutCount++
							resultMutex.Unlock()
						}
						
						resultMutex.Lock()
						failureCount++
						resultMutex.Unlock()
					}
					
					utils.CalculateScore(stored)
					utils.DetermineStatus(stored)
				}
				nd.nodesMutex.Unlock()
			}(node)
			
			time.Sleep(delay)
		}
		
		wg.Wait()
		
		// Brief pause between batches
		if end < len(nodesToCheck) {
			time.Sleep(1 * time.Second)
		}
	}
	
	elapsed := time.Since(start)
	log.Printf("Health check complete in %s: %d success, %d fail (%d timeouts), %d nodes skipped (recent gossip)", 
		elapsed, successCount, failureCount, timeoutCount, len(nodes)-len(nodesToCheck))
}






















func (nd *NodeDiscovery) processNodeAddress(address string) {
		nd.failedMutex.RLock()
	lastFailed, failed := nd.failedAddresses[address]
	nd.failedMutex.RUnlock()
	
	if failed && time.Since(lastFailed) < 5*time.Minute {
		return
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		// Handle address without port
		host = address
	}
	port, _ := strconv.Atoi(portStr)

	// CRITICAL FIX: Check if this IP is already tracked
	nd.allNodesMutex.RLock()
	existingByIP, ipExists := nd.allNodesByIP[host]
	nd.allNodesMutex.RUnlock()
	
	if ipExists {
		// IP already tracked - just update existing node instead of creating duplicate
		nd.nodesMutex.Lock()
		if stored, exists := nd.knownNodes[existingByIP.ID]; exists {
			stored.LastSeen = time.Now()  // Update last seen
			utils.DetermineStatus(stored)
		}
		nd.nodesMutex.Unlock()
		return
	}

	nd.rateLimiter <- struct{}{}
	defer func() { <-nd.rateLimiter }()

	verResp, err := nd.prpc.GetVersion(address)
	if err != nil {
		nd.failedMutex.Lock()
		nd.failedAddresses[address] = time.Now()
		nd.failedMutex.Unlock()
		
		pubkey := nd.findPubkeyForIP(host)
		nodeID := address
		if pubkey != "" {
			nodeID = pubkey
		}
		
		nd.nodesMutex.RLock()
		_, exists := nd.knownNodes[nodeID]
		nd.nodesMutex.RUnlock()
		
		if !exists {
			offlineNode := nd.createOfflineNode(address, host, port, pubkey)
			// REMOVED: offlineNode.IsOnline = false
			// Let DetermineStatus handle it
			utils.DetermineStatus(offlineNode)
			
			nd.allNodesMutex.Lock()
			nd.allNodesByIP[host] = offlineNode
			nd.allNodesMutex.Unlock()
		}
		return
	}

	nd.failedMutex.Lock()
	delete(nd.failedAddresses, address)
	nd.failedMutex.Unlock()

	pubkey := nd.findPubkeyForIP(host)
	var nodeID string
	if pubkey != "" {
		nodeID = pubkey
	} else {
		nodeID = address
	}
	
	nd.nodesMutex.RLock()
	existingNode, nodeExists := nd.knownNodes[nodeID]
	nd.nodesMutex.RUnlock()
	
	if nodeExists {
		nd.nodesMutex.Lock()
		// REMOVED: existingNode.IsOnline = true
		existingNode.LastSeen = time.Now()
		existingNode.Version = verResp.Version
		// REMOVED: existingNode.Status = "online"
		
		if existingNode.Pubkey == "" && pubkey != "" {
			existingNode.Pubkey = pubkey
			existingNode.IsRegistered = nd.registration.IsRegistered(pubkey)
			existingNode.ID = pubkey
			nd.knownNodes[pubkey] = existingNode
			delete(nd.knownNodes, nodeID)
		}
		
		hasRPC := false
		for i := range existingNode.Addresses {
			if existingNode.Addresses[i].Type == "rpc" {
				existingNode.Addresses[i].LastSeen = time.Now()
				existingNode.Addresses[i].IsWorking = true
				hasRPC = true
				break
			}
		}
		if !hasRPC {
			existingNode.Addresses = append(existingNode.Addresses, models.NodeAddress{
				Address:   address,
				IP:        host,
				Port:      port,
				Type:      "rpc",
				LastSeen:  time.Now(),
				IsWorking: true,
			})
		}
		
		// Let DetermineStatus make the final decision
		utils.DetermineStatus(existingNode)
		utils.CalculateScore(existingNode)
		nd.nodesMutex.Unlock()
		
		nd.allNodesMutex.Lock()
		if _, tracked := nd.allNodesByIP[host]; !tracked {
			nd.allNodesByIP[host] = existingNode
		}
		nd.allNodesMutex.Unlock()
		return
	}

	// Create new node
	newNode := &models.Node{
		ID:               nodeID,
		Pubkey:           pubkey,
		Address:          address,
		IP:               host,
		Port:             port,
		Version:          verResp.Version,
		// REMOVED: IsOnline: true,
		IsRegistered:     nd.registration.IsRegistered(pubkey),
		FirstSeen:        time.Now(),
		LastSeen:         time.Now(),
		// REMOVED: Status: "online",
		UptimeScore:      100,
		PerformanceScore: 100,
		CallHistory:      make([]bool, 0, 10),
		SuccessCalls:     1,
		TotalCalls:       1,
		Addresses: []models.NodeAddress{
			{
				Address:   address,
				IP:        host,
				Port:      port,
				Type:      "rpc",
				LastSeen:  time.Now(),
				IsWorking: true,
			},
		},
	}

	versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(verResp.Version, nil)
	newNode.VersionStatus = versionStatus
	newNode.IsUpgradeNeeded = needsUpgrade
	newNode.UpgradeSeverity = severity
	newNode.UpgradeMessage = utils.GetUpgradeMessage(verResp.Version, nil)

	country, city, lat, lon := nd.geo.Lookup(host)
	newNode.Country = country
	newNode.City = city
	newNode.Lat = lat
	newNode.Lon = lon

	// Let DetermineStatus set is_online and status
	utils.DetermineStatus(newNode)
	utils.CalculateScore(newNode)

	nd.nodesMutex.Lock()
	nd.knownNodes[nodeID] = newNode
	nd.nodesMutex.Unlock()

	nd.allNodesMutex.Lock()
	nd.allNodesByIP[host] = newNode
	nd.allNodesMutex.Unlock()

	nd.ipMutex.Lock()
	nd.ipToNodes[host] = append(nd.ipToNodes[host], newNode)
	nd.ipMutex.Unlock()

	if pubkey != "" {
		nd.enrichNodeWithCredits(newNode)
	}

	go nd.discoverPeersFromNode(address)
}




func (nd *NodeDiscovery) createOfflineNode(address, host string, port int, pubkey string) *models.Node {
	nodeID := address
	if pubkey != "" {
		nodeID = pubkey
	}
	
	offlineNode := &models.Node{
		ID:               nodeID,
		Pubkey:           pubkey,
		Address:          address,
		IP:               host,
		Port:             port,
		Version:          "unknown",
		// REMOVED: IsOnline: false,
		IsRegistered:     nd.registration.IsRegistered(pubkey),
		FirstSeen:        time.Now(),
		LastSeen:         time.Now().Add(-10 * time.Minute),
		// REMOVED: Status: "offline",
		UptimeScore:      0,
		PerformanceScore: 0,
		CallHistory:      make([]bool, 0),
		SuccessCalls:     0,
		TotalCalls:       1,
		Addresses: []models.NodeAddress{
			{
				Address:   address,
				IP:        host,
				Port:      port,
				Type:      "rpc",
				LastSeen:  time.Now(),
				IsWorking: false,
			},
		},
	}
	
	country, city, lat, lon := nd.geo.Lookup(host)
	offlineNode.Country = country
	offlineNode.City = city
	offlineNode.Lat = lat
	offlineNode.Lon = lon
	
	offlineNode.VersionStatus = "unknown"
	offlineNode.IsUpgradeNeeded = false
	offlineNode.UpgradeSeverity = "none"
	offlineNode.UpgradeMessage = ""
	
	// Let DetermineStatus make the decision
	utils.DetermineStatus(offlineNode)
	
	nd.nodesMutex.Lock()
	nd.knownNodes[nodeID] = offlineNode
	nd.nodesMutex.Unlock()
	
	nd.ipMutex.Lock()
	nd.ipToNodes[host] = append(nd.ipToNodes[host], offlineNode)
	nd.ipMutex.Unlock()
	
	log.Printf("Tracked offline node: %s (%s, registered: %v) - will retry in health checks", 
		address, country, offlineNode.IsRegistered)
	
	return offlineNode
}




func (nd *NodeDiscovery) findPubkeyForIP(targetIP string) string {
	nd.nodesMutex.RLock()
	nodesToQuery := make([]*models.Node, 0, len(nd.knownNodes))
	for _, node := range nd.knownNodes {
		if node.IsOnline {
			nodesToQuery = append(nodesToQuery, node)
		}
	}
	nd.nodesMutex.RUnlock()

	for i := 0; i < 3 && i < len(nodesToQuery); i++ {
		node := nodesToQuery[i]

		podsResp, err := nd.prpc.GetPods(node.Address)
		if err != nil {
			continue
		}

		for _, pod := range podsResp.Pods {
			podHost, _, err := net.SplitHostPort(pod.Address)
			if err != nil {
				podHost = pod.Address
			}

			if podHost == targetIP && pod.Pubkey != "" {
				return pod.Pubkey
			}
		}
	}

	return ""
}






func (nd *NodeDiscovery) discoverPeers() {
	nodes := nd.GetNodes()
	
	// Get healthy online nodes to query
	onlineNodes := make([]*models.Node, 0)
	for _, node := range nodes {
		if node.IsOnline && node.Status == "online" {
			onlineNodes = append(onlineNodes, node)
		}
	}
	
	log.Printf("Starting peer discovery from %d online nodes", len(onlineNodes))
	
	// Query multiple nodes in parallel to get complete peer list
	maxNodesToQuery := 7
	if len(onlineNodes) > maxNodesToQuery {
		//onlineNodes = onlineNodes[:maxNodesToQuery]
			sort.Slice(onlineNodes, func(i, j int) bool {
			return onlineNodes[i].LastSeen.After(onlineNodes[j].LastSeen)
		})
		onlineNodes = onlineNodes[:maxNodesToQuery]
	}
	
	var wg sync.WaitGroup
	for _, node := range onlineNodes {
		wg.Add(1)
		go func(n *models.Node) {
			defer wg.Done()
			nd.discoverPeersFromNode(n.Address)
		}(node)
		time.Sleep(300 * time.Millisecond) // Stagger queries
	}
	
	wg.Wait()
	
	// Log summary with both counts
	totalIPs, uniquePubkeys := nd.GetNodeCounts()
	
	log.Printf("Peer discovery complete. Total nodes (IPs): %d, Total pods (pubkeys): %d", 
		totalIPs, uniquePubkeys)
}









func (nd *NodeDiscovery) matchPodToNode(pod models.Pod, podIP string) {
	
	nd.ipMutex.RLock()
	nodesWithIP := nd.ipToNodes[podIP]
	nd.ipMutex.RUnlock()

	if len(nodesWithIP) == 0 {
		return
	}

	nd.nodesMutex.Lock()
	defer nd.nodesMutex.Unlock()

	for _, node := range nodesWithIP {
		// Match by IP or by pubkey
		matchByIP := node.IP == podIP
		matchByPubkey := pod.Pubkey != "" && node.Pubkey == pod.Pubkey
		
		if !matchByIP && !matchByPubkey {
			continue
		}

		// Upgrade node with pod data
		oldID := node.ID
	

	if pod.Pubkey != "" && node.Pubkey == "" {
		// Upgrade from unknown to known pubkey
		
		node.ID = pod.Pubkey
		node.Pubkey = pod.Pubkey
		node.IsRegistered = nd.registration.IsRegistered(pod.Pubkey)

			nd.knownNodes[pod.Pubkey] = node
			if oldID != pod.Pubkey {
				delete(nd.knownNodes, oldID)
			}
	
		
		log.Printf("DEBUG: ✓ UPGRADED node %s → pubkey: %s", node.Address, pod.Pubkey)
	}
	
	nd.updateNodeFromPod(node, &pod)
	if node.Pubkey != "" {
			nd.enrichNodeWithCredits(node)
		}

		break // Only upgrade one node per IP
	}
}



func (nd *NodeDiscovery) collectStats() {
    nodes := nd.GetNodes()

    // Use rate limiter to avoid overwhelming the network
    for _, node := range nodes {
        nd.rateLimiter <- struct{}{}
        go func(n *models.Node) {
            defer func() { <-nd.rateLimiter }()

            // CRITICAL FIX: Use the correct RPC address instead of n.Address
            rpcAddr := nd.getRPCAddress(n)

            statsResp, err := nd.prpc.GetStats(rpcAddr)
            if err != nil {
                // Optional: Log failures for debugging (remove in production if too noisy)
                // log.Printf("Failed to get stats from %s (using RPC %s): %v", n.ID, rpcAddr, err)
                return
            }

            nd.nodesMutex.Lock()
            if storedNode, exists := nd.knownNodes[n.ID]; exists {
                nd.updateStats(storedNode, statsResp)

                // Update common fields on success
                storedNode.LastSeen = time.Now()
                storedNode.IsOnline = true

                // Mark the RPC address as working
                for i := range storedNode.Addresses {
                    if storedNode.Addresses[i].Type == "rpc" && storedNode.Addresses[i].Address == rpcAddr {
                        storedNode.Addresses[i].IsWorking = true
                        storedNode.Addresses[i].LastSeen = time.Now()
                        break
                    }
                }

                utils.CalculateScore(storedNode)
                utils.DetermineStatus(storedNode)
            }
            nd.nodesMutex.Unlock()

            // Enrich credits if pubkey exists (no RPC needed)
            if n.Pubkey != "" {
                nd.enrichNodeWithCredits(n)
            }
        }(node)
    }

    // Small sleep to allow goroutines to complete (optional, but helps batching)
    time.Sleep(2 * time.Second)
}


func updateCallHistory(n *models.Node, success bool) {
	if n.CallHistory == nil {
		n.CallHistory = make([]bool, 0, 10)
	}
	if len(n.CallHistory) >= 10 {
		n.CallHistory = n.CallHistory[1:]
	}
	n.CallHistory = append(n.CallHistory, success)
}

func (nd *NodeDiscovery) updateStats(node *models.Node, stats *models.StatsResponse) {
	node.CPUPercent = stats.CPUPercent
	node.RAMUsed = stats.RAMUsed
	node.RAMTotal = stats.RAMTotal
	node.UptimeSeconds = stats.Uptime
	node.PacketsReceived = stats.PacketsReceived
	node.PacketsSent = stats.PacketsSent
	node.StorageCapacity = stats.FileSize
	node.StorageUsed = stats.TotalBytes

	if stats.Uptime > 0 {
		knownDuration := time.Since(node.FirstSeen).Seconds()
		if knownDuration > 0 {
			ratio := float64(stats.Uptime) / knownDuration
			if ratio > 1 {
				ratio = 1
			}
			node.UptimeScore = ratio * 100
		} else {
			node.UptimeScore = 100
		}
	}
}




func (nd *NodeDiscovery) updateNodeFromPod(node *models.Node, pod *models.Pod) {
	podHost, gossipPortStr, err := net.SplitHostPort(pod.Address)
	if err != nil {
		podHost = pod.Address
		gossipPortStr = "0"
	}
	gossipPort, _ := strconv.Atoi(gossipPortStr)
	
	if pod.Pubkey != "" {
		node.Pubkey = pod.Pubkey
		node.IsRegistered = nd.registration.IsRegistered(pod.Pubkey)
	}
	
	node.IsPublic = pod.IsPublic
	
	if pod.Version != "" {
		node.Version = pod.Version
		versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(pod.Version, nil)
		node.VersionStatus = versionStatus
		node.IsUpgradeNeeded = needsUpgrade
		node.UpgradeSeverity = severity
		node.UpgradeMessage = utils.GetUpgradeMessage(pod.Version, nil)
	}
	
	if pod.StorageCommitted > 0 {
		node.StorageCapacity = pod.StorageCommitted
		node.StorageUsed = pod.StorageUsed
		node.StorageUsagePercent = pod.StorageUsagePercent
	}
	
	if pod.Uptime > 0 {
		node.UptimeSeconds = pod.Uptime
		knownDuration := time.Since(node.FirstSeen).Seconds()
		if knownDuration > 0 {
			ratio := float64(pod.Uptime) / knownDuration
			if ratio > 1 {
				ratio = 1
			}
			node.UptimeScore = ratio * 100
		} else {
			node.UptimeScore = 100
		}
	}
	
	if pod.LastSeenTimestamp > 0 {
		podLastSeen := time.Unix(pod.LastSeenTimestamp, 0)
		if podLastSeen.After(node.LastSeen) {
			node.LastSeen = podLastSeen
			// REMOVED: if time.Since(podLastSeen) < 2*time.Minute { node.IsOnline = true }
		}
	}
	
	currentIsGossip := node.Port != 6000 && node.Port > 0
	
	if !currentIsGossip && gossipPort > 0 {
		node.Address = pod.Address
		node.Port = gossipPort
	}
	
	hasGossip := false
	hasRPC := false
	
	for i := range node.Addresses {
		if node.Addresses[i].Type == "gossip" {
			hasGossip = true
			if gossipPort > 0 {
				node.Addresses[i].Address = pod.Address
				node.Addresses[i].Port = gossipPort
				node.Addresses[i].LastSeen = time.Unix(pod.LastSeenTimestamp, 0)
				node.Addresses[i].IsPublic = pod.IsPublic
			}
		}
		if node.Addresses[i].Type == "rpc" {
			hasRPC = true
		}
	}
	
	if !hasGossip && gossipPort > 0 {
		node.Addresses = append([]models.NodeAddress{
			{
				Address:   pod.Address,
				IP:        podHost,
				Port:      gossipPort,
				Type:      "gossip",
				IsPublic:  pod.IsPublic,
				LastSeen:  time.Unix(pod.LastSeenTimestamp, 0),
				IsWorking: true,
			},
		}, node.Addresses...)
	}
	
	if !hasRPC && pod.RpcPort > 0 && pod.RpcPort != gossipPort {
		rpcAddress := net.JoinHostPort(podHost, strconv.Itoa(pod.RpcPort))
		node.Addresses = append(node.Addresses, models.NodeAddress{
			Address:   rpcAddress,
			IP:        podHost,
			Port:      pod.RpcPort,
			Type:      "rpc",
			IsPublic:  pod.IsPublic,
			LastSeen:  time.Unix(pod.LastSeenTimestamp, 0),
			IsWorking: false,
		})
	}
	
	// Recalculate status after updating with pod data
	utils.DetermineStatus(node)
	utils.CalculateScore(node)
}









func (nd *NodeDiscovery) enrichNodeWithCredits(node *models.Node) {
	if nd.credits == nil || node.Pubkey == "" {
		return
	}

	credits, exists := nd.credits.GetCredits(node.Pubkey)
	if exists {
		node.Credits = credits.Credits
		node.CreditsRank = credits.Rank
		node.CreditsChange = credits.CreditsChange
	}
}

func (nd *NodeDiscovery) GetNodes() []*models.Node {
	nd.nodesMutex.RLock()
	defer nd.nodesMutex.RUnlock()

	nodes := make([]*models.Node, 0, len(nd.knownNodes))
	for _, n := range nd.knownNodes {
		nodes = append(nodes, n)
	}
	return nodes
}




func (nd *NodeDiscovery) createNodeFromPod(pod *models.Pod) {
	podHost, gossipPortStr, err := net.SplitHostPort(pod.Address)
	if err != nil {
		podHost = pod.Address
		gossipPortStr = "0"
	}
	gossipPort, _ := strconv.Atoi(gossipPortStr)

	var rpcAddress string
	var rpcPort int
	if pod.RpcPort > 0 {
		rpcPort = pod.RpcPort
		rpcAddress = net.JoinHostPort(podHost, strconv.Itoa(pod.RpcPort))
	} else {
		rpcPort = 6000
		rpcAddress = net.JoinHostPort(podHost, "6000")
	}

	nodeID := pod.Address
	if pod.Pubkey != "" {
		nodeID = pod.Pubkey
	}

	// nd.allNodesMutex.RLock()
	// _, ipExists := nd.allNodesByIP[podHost]
	// nd.allNodesMutex.RUnlock()

		nd.allNodesMutex.RLock()
	existingByIP, ipExists := nd.allNodesByIP[podHost]
	nd.allNodesMutex.RUnlock()

	nd.nodesMutex.Lock()
	existingNode, exists := nd.knownNodes[nodeID]
	nd.nodesMutex.Unlock()

		if ipExists && !exists {
		// Found by IP but not by ID - update the IP-based node
		nd.nodesMutex.Lock()
		nd.updateNodeFromPod(existingByIP, pod)
		nd.nodesMutex.Unlock()
		return
	}

	if exists {
		nd.nodesMutex.Lock()
		nd.updateNodeFromPod(existingNode, pod)
		nd.nodesMutex.Unlock()
		
		if !ipExists {
			nd.allNodesMutex.Lock()
			nd.allNodesByIP[podHost] = existingNode
			nd.allNodesMutex.Unlock()
		}
		return
	}

	now := time.Now()
	podLastSeen := time.Unix(pod.LastSeenTimestamp, 0)
	// REMOVED: isOnline calculation
	// REMOVED: status calculation

	addresses := []models.NodeAddress{}
	
	if gossipPort > 0 {
		addresses = append(addresses, models.NodeAddress{
			Address:   pod.Address,
			IP:        podHost,
			Port:      gossipPort,
			Type:      "gossip",
			IsPublic:  pod.IsPublic,
			LastSeen:  podLastSeen,
			IsWorking: true, // Assume working since it's in peer list
		})
	}
	
	if rpcPort != gossipPort && rpcPort > 0 {
		addresses = append(addresses, models.NodeAddress{
			Address:   rpcAddress,
			IP:        podHost,
			Port:      rpcPort,
			Type:      "rpc",
			IsPublic:  pod.IsPublic,
			LastSeen:  podLastSeen,
			IsWorking: false, // Unknown until we try
		})
	}

	newNode := &models.Node{
		ID:               nodeID,
		Pubkey:           pod.Pubkey,
		Address:          pod.Address,
		IP:               podHost,
		Port:             gossipPort,
		Version:          pod.Version,
		// REMOVED: IsOnline: isOnline,
		IsPublic:         pod.IsPublic,
		IsRegistered:     nd.registration.IsRegistered(pod.Pubkey),
		FirstSeen:        now,
		LastSeen:         podLastSeen,
		// REMOVED: Status: status,
		UptimeScore:      0,
		PerformanceScore: 0,
		CallHistory:      make([]bool, 0),
		StorageCapacity:  pod.StorageCommitted,
		StorageUsed:      pod.StorageUsed,
		UptimeSeconds:    pod.Uptime,
		Addresses:        addresses,
	}

	if pod.Uptime > 0 {
		newNode.UptimeScore = 95.0
	}

	if pod.Version != "" {
		versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(pod.Version, nil)
		newNode.VersionStatus = versionStatus
		newNode.IsUpgradeNeeded = needsUpgrade
		newNode.UpgradeSeverity = severity
		newNode.UpgradeMessage = utils.GetUpgradeMessage(pod.Version, nil)
	}

	country, city, lat, lon := nd.geo.Lookup(podHost)
	newNode.Country = country
	newNode.City = city
	newNode.Lat = lat
	newNode.Lon = lon

	// Let DetermineStatus make the final decision
	utils.CalculateScore(newNode)
	utils.DetermineStatus(newNode)

	nd.nodesMutex.Lock()
	nd.knownNodes[nodeID] = newNode
	nd.nodesMutex.Unlock()

	nd.allNodesMutex.Lock()
	nd.allNodesByIP[podHost] = newNode
	nd.allNodesMutex.Unlock()

	nd.ipMutex.Lock()
	nd.ipToNodes[podHost] = append(nd.ipToNodes[podHost], newNode)
	nd.ipMutex.Unlock()

	if pod.Pubkey != "" {
		nd.enrichNodeWithCredits(newNode)
	}

	log.Printf("Created node from pod: %s (gossip: %s, rpc: %s, %s, %s, public=%v, registered=%v)", 
		nodeID, pod.Address, rpcAddress, country, pod.Version, pod.IsPublic, newNode.IsRegistered)
}




// GetAllNodes returns all nodes tracked by IP (including duplicates)
func (nd *NodeDiscovery) GetAllNodes() []*models.Node {
	nd.allNodesMutex.RLock()
	defer nd.allNodesMutex.RUnlock()

	nodes := make([]*models.Node, 0, len(nd.allNodesByIP))
	for _, n := range nd.allNodesByIP {
		nodes = append(nodes, n)
	}
	return nodes
}



// GetNodeCounts returns both IP count and unique pubkey count
func (nd *NodeDiscovery) GetNodeCounts() (totalIPs int, uniquePubkeys int) {
	nd.allNodesMutex.RLock()
	totalIPs = len(nd.allNodesByIP)
	nd.allNodesMutex.RUnlock()
	
	nd.nodesMutex.RLock()
	uniquePubkeys = len(nd.knownNodes)
	nd.nodesMutex.RUnlock()
	
	return
}



func (nd *NodeDiscovery) discoverPeersFromNode(address string) {
	nd.rateLimiter <- struct{}{}
	defer func() { <-nd.rateLimiter }()

	podsResp, err := nd.prpc.GetPods(address)
	if err != nil {
		return
	}

	log.Printf("DEBUG: Got %d pods from %s", len(podsResp.Pods), address)

	// First pass: Create/update ALL nodes from pod data
	for _, pod := range podsResp.Pods {
		nd.createNodeFromPod(&pod)
	}

	// Second pass: Match pods to existing nodes by pubkey
	for _, pod := range podsResp.Pods {
		if pod.Pubkey == "" {
			continue
		}

		podHost, _, err := net.SplitHostPort(pod.Address)
		if err != nil {
			podHost = pod.Address
		}

		nd.matchPodToNode(pod, podHost)
	}

	// Third pass: Verify connectivity using RPC address
	verificationCount := 0
	for _, pod := range podsResp.Pods {
		podHost, _, err := net.SplitHostPort(pod.Address)
		if err != nil {
			podHost = pod.Address
		}

		// Build RPC address for connectivity check
		var rpcAddress string
		if pod.RpcPort > 0 {
			rpcAddress = net.JoinHostPort(podHost, strconv.Itoa(pod.RpcPort))
		} else {
			rpcAddress = net.JoinHostPort(podHost, "6000")
		}

		// Skip self
		if rpcAddress == address {
			continue
		}

		nodeID := pod.Pubkey
		if nodeID == "" {
			nodeID = pod.Address
		}

		nd.nodesMutex.RLock()
		existingNode, exists := nd.knownNodes[nodeID]
		nd.nodesMutex.RUnlock()

		// Only verify if public or recently seen
		shouldVerify := !exists || 
			existingNode.IsPublic || 
			time.Since(existingNode.LastSeen) < 5*time.Minute

		if shouldVerify && verificationCount < 50 {
			go func(addr string) {
				time.Sleep(100 * time.Millisecond)
				nd.processNodeAddress(addr)  // Uses RPC address for connection
			}(rpcAddress)
			verificationCount++
		}
	}

	log.Printf("DEBUG: Peer discovery from %s - created/updated %d nodes, verifying %d connections", 
		address, len(podsResp.Pods), verificationCount)
}




// ADD THIS HELPER FUNCTION
func (nd *NodeDiscovery) getRPCAddress(node *models.Node) string {
    // Priority 1: Look for working RPC address
    for _, addr := range node.Addresses {
        if addr.Type == "rpc" && addr.IsWorking {
            return addr.Address
        }
    }
    
    // Priority 2: Any RPC address
    for _, addr := range node.Addresses {
        if addr.Type == "rpc" {
            return addr.Address
        }
    }
    
    // Priority 3: Construct default RPC address
    return net.JoinHostPort(node.IP, "6000")
}