package services

import (
	"log"
	"net"
	//"sort"
	"strconv"
	"sync"
	"time"

	"xand/config"
	"xand/models"
	"xand/utils"
)

type NodeDiscovery struct {
	cfg     *config.Config
	prpc    *PRPCClient
	geo     *utils.GeoResolver
	credits *CreditsService

	knownNodes map[string]*models.Node // Key is ALWAYS pubkey once discovered
	nodesMutex sync.RWMutex

	// Track IP->nodes for reverse lookup
	ipToNodes map[string][]*models.Node // One IP can have multiple nodes
	ipMutex   sync.RWMutex

	// Track failed addresses to avoid retry spam
	failedAddresses map[string]time.Time // address -> last failure time
	failedMutex     sync.RWMutex

	stopChan    chan struct{}
	rateLimiter chan struct{}
}

func NewNodeDiscovery(cfg *config.Config, prpc *PRPCClient, geo *utils.GeoResolver, credits *CreditsService) *NodeDiscovery {
	return &NodeDiscovery{
		cfg:             cfg,
		prpc:            prpc,
		geo:             geo,
		credits:         credits,
		knownNodes:      make(map[string]*models.Node),
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

func (nd *NodeDiscovery) Stop() {
	close(nd.stopChan)
}

// func (nd *NodeDiscovery) runDiscoveryLoop() {
// 	ticker := time.NewTicker(time.Duration(nd.cfg.Polling.DiscoveryInterval) * time.Second)
// 	defer ticker.Stop()
// 	for {
// 		select {
// 		case <-ticker.C:
// 			nd.discoverPeers()
// 		case <-nd.stopChan:
// 			return
// 		}
// 	}
// }


func (nd *NodeDiscovery) runDiscoveryLoop() {
	// CRITICAL FIX: Reduce interval to discover nodes faster
	// Initial aggressive discovery, then settle into normal interval
	ticker := time.NewTicker(30 * time.Second) // More frequent than config
	defer ticker.Stop()
	
	discoveryCount := 0
	
	for {
		select {
		case <-ticker.C:
			discoveryCount++
			
			// Log current state
			nd.nodesMutex.RLock()
			totalNodes := len(nd.knownNodes)
			nd.nodesMutex.RUnlock()
			
			log.Printf("Discovery cycle #%d (total nodes: %d)", discoveryCount, totalNodes)
			
			nd.discoverPeers()
			
			// After first 10 cycles (5 minutes), slow down to configured interval
			if discoveryCount == 10 {
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
	for {
		select {
		case <-ticker.C:
			nd.healthCheck()
		case <-nd.stopChan:
			return
		}
	}
}

// func (nd *NodeDiscovery) Bootstrap() {
// 	log.Println("Starting Bootstrap...")

// 	var wg sync.WaitGroup
// 	for _, seed := range nd.cfg.Server.SeedNodes {
// 		wg.Add(1)
// 		go func(seedAddr string) {
// 			defer wg.Done()
// 			log.Printf("Bootstrapping from seed: %s", seedAddr)
// 			nd.processNodeAddress(seedAddr)
// 		}(seed)
// 		time.Sleep(1 * time.Second) // Stagger starts
// 	}

// 	// Wait for initial discovery
// 	wg.Wait()
// 	log.Println("Bootstrap complete, waiting for peer discovery...")

// 	time.Sleep(5 * time.Second)
// 	go nd.discoverPeers()
// 	go nd.healthCheck()
// 	go nd.collectStats()
// }




func (nd *NodeDiscovery) Bootstrap() {
	log.Println("Starting Bootstrap...")
	
	var wg sync.WaitGroup
	for _, seed := range nd.cfg.Server.SeedNodes {
		wg.Add(1)
		go func(seedAddr string) {
			defer wg.Done()
			log.Printf("Bootstrapping from seed: %s", seedAddr)
			nd.processNodeAddress(seedAddr)
		}(seed)
		time.Sleep(500 * time.Millisecond) // Stagger starts
	}
	
	// Wait for seed nodes
	wg.Wait()
	log.Println("Bootstrap complete, starting peer discovery...")
	
	// CRITICAL FIX: Run peer discovery multiple times during bootstrap
	// to ensure we discover the full network
	for i := 0; i < 3; i++ {
		log.Printf("Bootstrap peer discovery round %d/3", i+1)
		nd.discoverPeers()
		
		nd.nodesMutex.RLock()
		nodeCount := len(nd.knownNodes)
		nd.nodesMutex.RUnlock()
		
		log.Printf("After round %d: %d nodes tracked", i+1, nodeCount)
		
		if i < 2 {
			time.Sleep(5 * time.Second) // Wait between rounds
		}
	}
	
	// Initial health check and stats collection
	log.Println("Running initial health check and stats collection...")
	nd.healthCheck()
	time.Sleep(2 * time.Second)
	nd.collectStats()
	
	nd.nodesMutex.RLock()
	finalCount := len(nd.knownNodes)
	nd.nodesMutex.RUnlock()
	
	log.Printf("Bootstrap finished. Total nodes discovered: %d", finalCount)
}



// ============================================
// FIXED: Synchronous pubkey discovery
// ============================================

func (nd *NodeDiscovery) processNodeAddress(address string) {
	// Check if recently failed (skip for 5 minutes)
	nd.failedMutex.RLock()
	lastFailed, failed := nd.failedAddresses[address]
	nd.failedMutex.RUnlock()
	
	if failed && time.Since(lastFailed) < 5*time.Minute {
		return // Skip recently failed addresses
	}

	// Check if IP already has a node
	host, portStr, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portStr)

	// Check if this exact address already exists
	nd.ipMutex.RLock()
	existingNodes := nd.ipToNodes[host]
	for _, node := range existingNodes {
		if node.Address == address {
			nd.ipMutex.RUnlock()
			return // Already processed
		}
	}
	nd.ipMutex.RUnlock()

	// Rate limit
	nd.rateLimiter <- struct{}{}
	defer func() { <-nd.rateLimiter }()

	// Verify connectivity
	verResp, err := nd.prpc.GetVersion(address)
	if err != nil {
		// CRITICAL FIX: Create offline node entry even for failed connections
		nd.failedMutex.Lock()
		nd.failedAddresses[address] = time.Now()
		failCount := len(nd.failedAddresses)
		nd.failedMutex.Unlock()
		
		// Only log every 10th failure to reduce spam
		if failCount%10 == 0 {
			log.Printf("DEBUG: %d addresses currently unreachable", failCount)
		}
		
		// Try to find pubkey for this IP from other nodes
		pubkey := nd.findPubkeyForIP(host)
		nodeID := address
		if pubkey != "" {
			nodeID = pubkey
		}
		
		// Check if we already have this node
		nd.nodesMutex.RLock()
		_, exists := nd.knownNodes[nodeID]
		nd.nodesMutex.RUnlock()
		
		if !exists {
			// Create offline node placeholder
			offlineNode := &models.Node{
				ID:               nodeID,
				Pubkey:           pubkey,
				Address:          address,
				IP:               host,
				Port:             port,
				Version:          "unknown",
				IsOnline:         false,
				FirstSeen:        time.Now(),
				LastSeen:         time.Now().Add(-10 * time.Minute), // Mark as not seen recently
				Status:           "offline",
				UptimeScore:      0,
				PerformanceScore: 0,
				CallHistory:      make([]bool, 0),
				SuccessCalls:     0,
				TotalCalls:       1, // Mark that we tried
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
			
			// GeoIP lookup even for offline nodes
			country, city, lat, lon := nd.geo.Lookup(host)
			offlineNode.Country = country
			offlineNode.City = city
			offlineNode.Lat = lat
			offlineNode.Lon = lon
			
			// Version status
			offlineNode.VersionStatus = "unknown"
			offlineNode.IsUpgradeNeeded = false
			offlineNode.UpgradeSeverity = "none"
			offlineNode.UpgradeMessage = ""
			
			// Store offline node
			nd.nodesMutex.Lock()
			nd.knownNodes[nodeID] = offlineNode
			nd.nodesMutex.Unlock()
			
			// Add to IP index
			nd.ipMutex.Lock()
			nd.ipToNodes[host] = append(nd.ipToNodes[host], offlineNode)
			nd.ipMutex.Unlock()
			
			log.Printf("Tracked offline node: %s (%s) - will retry in health checks", address, country)
		}
		
		return
	}

	// Clear from failed list if it was there
	nd.failedMutex.Lock()
	delete(nd.failedAddresses, address)
	nd.failedMutex.Unlock()

	log.Printf("DEBUG: ✓ Connected to %s, version %s", address, verResp.Version)

	// Try to get pubkey IMMEDIATELY by querying a known node's peer list
	pubkey := nd.findPubkeyForIP(host)

	var nodeID string
	if pubkey != "" {
		nodeID = pubkey
		log.Printf("DEBUG: ✓ Found pubkey for %s: %s", address, pubkey)
	} else {
		nodeID = address // Temporary ID
		log.Printf("DEBUG: No pubkey yet for %s, using address as ID", address)
	}

	// Check if we already have this node (might have been created as offline)
	nd.nodesMutex.RLock()
	existingNode, nodeExists := nd.knownNodes[nodeID]
	nd.nodesMutex.RUnlock()
	
	if nodeExists && existingNode.IsOnline {
		// Node already exists and is online, skip re-creation
		log.Printf("DEBUG: Node %s already exists and is online, skipping", nodeID)
		return
	}

	// Create or update node
	newNode := &models.Node{
		ID:               nodeID,
		Pubkey:           pubkey,
		Address:          address,
		IP:               host,
		Port:             port,
		Version:          verResp.Version,
		IsOnline:         true,
		FirstSeen:        time.Now(),
		LastSeen:         time.Now(),
		Status:           "online",
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
	
	// If node existed as offline, preserve FirstSeen timestamp
	if nodeExists {
		newNode.FirstSeen = existingNode.FirstSeen
		newNode.CallHistory = existingNode.CallHistory
		newNode.TotalCalls = existingNode.TotalCalls + 1
		newNode.SuccessCalls = existingNode.SuccessCalls + 1
	}

	// Version status
	versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(verResp.Version, nil)
	newNode.VersionStatus = versionStatus
	newNode.IsUpgradeNeeded = needsUpgrade
	newNode.UpgradeSeverity = severity
	newNode.UpgradeMessage = utils.GetUpgradeMessage(verResp.Version, nil)

	// Get stats
	statsResp, err := nd.prpc.GetStats(address)
	if err == nil {
		nd.updateStats(newNode, statsResp)
	}

	// GeoIP
	country, city, lat, lon := nd.geo.Lookup(host)
	newNode.Country = country
	newNode.City = city
	newNode.Lat = lat
	newNode.Lon = lon

	// Store
	nd.nodesMutex.Lock()
	nd.knownNodes[nodeID] = newNode
	nd.nodesMutex.Unlock()

	// Add to IP index (or update if it was offline before)
	nd.ipMutex.Lock()
	if !nodeExists {
		nd.ipToNodes[host] = append(nd.ipToNodes[host], newNode)
	} else {
		// Update reference in IP index
		for i, n := range nd.ipToNodes[host] {
			if n.ID == nodeID {
				nd.ipToNodes[host][i] = newNode
				break
			}
		}
	}
	nd.ipMutex.Unlock()

	// Enrich with credits if we have pubkey
	if pubkey != "" {
		nd.enrichNodeWithCredits(newNode)
	}

	log.Printf("Discovered node: %s (%s, %s) [pubkey: %s, status: online → %s]", 
		address, country, verResp.Version, 
		func() string { 
			if pubkey != "" { 
				return pubkey[:8] + "..." 
			} else { 
				return "pending" 
			} 
		}(),
		func() string {
			if nodeExists {
				return "was offline"
			}
			return "new"
		}())

	// Discover peers asynchronously (will help find more pubkeys)
	go nd.discoverPeersFromNode(address)
}
// ============================================
// NEW: Find pubkey by querying existing nodes
// ============================================

func (nd *NodeDiscovery) findPubkeyForIP(targetIP string) string {
	// Get a list of nodes to query
	nd.nodesMutex.RLock()
	nodesToQuery := make([]*models.Node, 0, len(nd.knownNodes))
	for _, node := range nd.knownNodes {
		if node.IsOnline {
			nodesToQuery = append(nodesToQuery, node)
		}
	}
	nd.nodesMutex.RUnlock()

	// Try up to 3 nodes
	for i := 0; i < 3 && i < len(nodesToQuery); i++ {
		node := nodesToQuery[i]

		podsResp, err := nd.prpc.GetPods(node.Address)
		if err != nil {
			continue
		}

		// Search for matching IP in pods
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

	return "" // Not found
}

// ============================================
// Discover peers and match them back
// ============================================

// func (nd *NodeDiscovery) discoverPeersFromNode(address string) {
// 	nd.rateLimiter <- struct{}{}
// 	defer func() { <-nd.rateLimiter }()

// 	podsResp, err := nd.prpc.GetPods(address)
// 	if err != nil {
// 		return
// 	}

// 	log.Printf("DEBUG: Got %d pods from %s", len(podsResp.Pods), address)

// 	// First pass: Update existing nodes with pubkeys
// 	for _, pod := range podsResp.Pods {
// 		if pod.Pubkey == "" {
// 			continue
// 		}

// 		podHost, _, err := net.SplitHostPort(pod.Address)
// 		if err != nil {
// 			podHost = pod.Address
// 		}

// 		nd.matchPodToNode(pod, podHost)
// 	}

// 	// Second pass: Discover new peers (limited and prioritized)
// 	// Prioritize: public nodes, nodes with recent timestamps, nodes with higher uptime
// 	type scoredPod struct {
// 		pod   models.Pod
// 		score int
// 	}

// 	scoredPods := make([]scoredPod, 0, len(podsResp.Pods))
// 	for _, pod := range podsResp.Pods {
// 		score := 0
// 		if pod.IsPublic {
// 			score += 10
// 		}
// 		if time.Since(time.Unix(pod.LastSeenTimestamp, 0)) < 5*time.Minute {
// 			score += 5
// 		}
// 		if pod.Uptime > 86400 { // > 1 day
// 			score += 3
// 		}
// 		scoredPods = append(scoredPods, scoredPod{pod, score})
// 	}

// 	// Sort by score descending
// 	sort.Slice(scoredPods, func(i, j int) bool {
// 		return scoredPods[i].score > scoredPods[j].score
// 	})

// 	count := 0
// 	for _, sp := range scoredPods {
// 		if count >= 30 {
// 			break
// 		}

// 		pod := sp.pod
// 		podHost, _, err := net.SplitHostPort(pod.Address)
// 		if err != nil {
// 			podHost = pod.Address
// 		}

// 		var rpcAddress string
// 		if pod.RpcPort > 0 {
// 			rpcAddress = net.JoinHostPort(podHost, strconv.Itoa(pod.RpcPort))
// 		} else {
// 			rpcAddress = net.JoinHostPort(podHost, "6000")
// 		}

// 		if rpcAddress != address {
// 			go nd.processNodeAddress(rpcAddress)
// 			count++
// 			time.Sleep(100 * time.Millisecond)
// 		}
// 	}
// }


// func (nd *NodeDiscovery) discoverPeersFromNode(address string) {
// 	nd.rateLimiter <- struct{}{}
// 	defer func() { <-nd.rateLimiter }()

// 	podsResp, err := nd.prpc.GetPods(address)
// 	if err != nil {
// 		return
// 	}

// 	log.Printf("DEBUG: Got %d pods from %s", len(podsResp.Pods), address)

// 	// First pass: Update existing nodes with pubkeys
// 	for _, pod := range podsResp.Pods {
// 		if pod.Pubkey == "" {
// 			continue
// 		}

// 		podHost, _, err := net.SplitHostPort(pod.Address)
// 		if err != nil {
// 			podHost = pod.Address
// 		}

// 		nd.matchPodToNode(pod, podHost)
// 	}

// 	// Second pass: Process ALL pods as potential nodes
// 	// CRITICAL FIX: Remove the artificial 30-node limit
// 	processedCount := 0
// 	skippedCount := 0
	
// 	for _, pod := range podsResp.Pods {
// 		podHost, _, err := net.SplitHostPort(pod.Address)
// 		if err != nil {
// 			podHost = pod.Address
// 		}

// 		var rpcAddress string
// 		if pod.RpcPort > 0 {
// 			rpcAddress = net.JoinHostPort(podHost, strconv.Itoa(pod.RpcPort))
// 		} else {
// 			rpcAddress = net.JoinHostPort(podHost, "6000")
// 		}

// 		// Skip if this is the node we're querying (avoid self-loop)
// 		if rpcAddress == address {
// 			continue
// 		}

// 		// Check if this node already exists
// 		nodeID := rpcAddress
// 		if pod.Pubkey != "" {
// 			nodeID = pod.Pubkey
// 		}

// 		nd.nodesMutex.RLock()
// 		_, exists := nd.knownNodes[nodeID]
// 		nd.nodesMutex.RUnlock()

// 		if exists {
// 			skippedCount++
// 			continue // Already tracked
// 		}

// 		// Process this new node
// 		go func(addr string, p models.Pod) {
// 			// Small delay to avoid overwhelming the system
// 			time.Sleep(50 * time.Millisecond)
// 			nd.processNodeAddress(addr)
// 		}(rpcAddress, pod)
		
// 		processedCount++
// 	}
	
// 	log.Printf("DEBUG: Peer discovery from %s - processed %d new nodes, skipped %d existing", 
// 		address, processedCount, skippedCount)
// }






func (nd *NodeDiscovery) discoverPeersFromNode(address string) {
	nd.rateLimiter <- struct{}{}
	defer func() { <-nd.rateLimiter }()

	podsResp, err := nd.prpc.GetPods(address)
	if err != nil {
		return
	}

	log.Printf("DEBUG: Got %d pods from %s", len(podsResp.Pods), address)

	// CRITICAL FIX: First create/update ALL nodes from pod data
	// This ensures we have correct is_public flags and other metadata
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

	// Third pass: Verify connectivity for nodes we haven't connected to yet
	// Only attempt for nodes marked as public or recently seen
	verificationCount := 0
	for _, pod := range podsResp.Pods {
		podHost, _, err := net.SplitHostPort(pod.Address)
		if err != nil {
			podHost = pod.Address
		}

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

		nodeID := rpcAddress
		if pod.Pubkey != "" {
			nodeID = pod.Pubkey
		}

		nd.nodesMutex.RLock()
		existingNode, exists := nd.knownNodes[nodeID]
		nd.nodesMutex.RUnlock()

		// Only verify if:
		// 1. Public node, OR
		// 2. Recently seen (within 5 minutes), OR
		// 3. Node doesn't exist yet
		shouldVerify := !exists || 
			existingNode.IsPublic || 
			time.Since(existingNode.LastSeen) < 5*time.Minute

		if shouldVerify && verificationCount < 50 { // Limit verification attempts
			go func(addr string) {
				time.Sleep(100 * time.Millisecond)
				nd.processNodeAddress(addr)
			}(rpcAddress)
			verificationCount++
		}
	}

	log.Printf("DEBUG: Peer discovery from %s - created/updated %d nodes, verifying %d connections", 
		address, len(podsResp.Pods), verificationCount)
}


// ============================================
// Match pod to existing node
// ============================================

// func (nd *NodeDiscovery) matchPodToNode(pod models.Pod, podIP string) {
// 	nd.ipMutex.RLock()
// 	nodesWithIP := nd.ipToNodes[podIP]
// 	nd.ipMutex.RUnlock()

// 	if len(nodesWithIP) == 0 {
// 		return
// 	}

// 	nd.nodesMutex.Lock()
// 	defer nd.nodesMutex.Unlock()

// 	for _, node := range nodesWithIP {
// 		// If node already has a pubkey, skip
// 		if node.Pubkey != "" {
// 			continue
// 		}

// 		// Upgrade node with pubkey
// 		oldID := node.ID
// 		node.ID = pod.Pubkey
// 		node.Pubkey = pod.Pubkey
// 		nd.updateNodeFromPod(node, &pod)

// 		// Move to new key
// 		nd.knownNodes[pod.Pubkey] = node
// 		if oldID != pod.Pubkey {
// 			delete(nd.knownNodes, oldID)
// 		}

// 		log.Printf("DEBUG: ✓ UPGRADED node %s → pubkey: %s", node.Address, pod.Pubkey)

// 		// Enrich with credits
// 		nd.enrichNodeWithCredits(node)

// 		break // Only upgrade one node per IP
// 	}
// }


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
		
		// Update pubkey if we have it
		if pod.Pubkey != "" && node.Pubkey == "" {
			node.ID = pod.Pubkey
			node.Pubkey = pod.Pubkey
			
			// Move to new key in knownNodes
			nd.knownNodes[pod.Pubkey] = node
			if oldID != pod.Pubkey {
				delete(nd.knownNodes, oldID)
			}
			
			log.Printf("DEBUG: ✓ UPGRADED node %s → pubkey: %s", node.Address, pod.Pubkey)
		}
		
		// CRITICAL: Update node data from pod
		nd.updateNodeFromPod(node, &pod)

		// Enrich with credits
		if node.Pubkey != "" {
			nd.enrichNodeWithCredits(node)
		}
		
		break // Only upgrade one node per IP
	}
}






// ============================================
// Helper methods
// ============================================

// func (nd *NodeDiscovery) discoverPeers() {
// 	nodes := nd.GetNodes()

// 	onlineNodes := make([]*models.Node, 0)
// 	for _, node := range nodes {
// 		if node.IsOnline && node.Status == "online" {
// 			onlineNodes = append(onlineNodes, node)
// 		}
// 	}

// 	if len(onlineNodes) > 30 {
// 		onlineNodes = onlineNodes[:30]
// 	}

// 	for _, node := range onlineNodes {
// 		go nd.discoverPeersFromNode(node.Address)
// 		time.Sleep(500 * time.Millisecond)
// 	}
// }










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
	// CRITICAL FIX: Query more nodes to discover the full network
	maxNodesToQuery := 10 // Query up to 10 nodes to get comprehensive peer lists
	if len(onlineNodes) > maxNodesToQuery {
		onlineNodes = onlineNodes[:maxNodesToQuery]
	}
	
	var wg sync.WaitGroup
	for _, node := range onlineNodes {
		wg.Add(1)
		go func(n *models.Node) {
			defer wg.Done()
			nd.discoverPeersFromNode(n.Address)
		}(node)
		time.Sleep(200 * time.Millisecond) // Stagger queries
	}
	
	wg.Wait()
	
	// Log summary
	nd.nodesMutex.RLock()
	totalNodes := len(nd.knownNodes)
	nd.nodesMutex.RUnlock()
	
	log.Printf("Peer discovery complete. Total nodes tracked: %d", totalNodes)
}

func (nd *NodeDiscovery) collectStats() {
	nodes := nd.GetNodes()

	for _, node := range nodes {
		nd.rateLimiter <- struct{}{}

		go func(n *models.Node) {
			defer func() { <-nd.rateLimiter }()

			statsResp, err := nd.prpc.GetStats(n.Address)
			if err != nil {
				return
			}

			nd.nodesMutex.Lock()
			if storedNode, exists := nd.knownNodes[n.ID]; exists {
				nd.updateStats(storedNode, statsResp)
				storedNode.LastSeen = time.Now()
				storedNode.IsOnline = true
				utils.CalculateScore(storedNode)
				utils.DetermineStatus(storedNode)
			}
			nd.nodesMutex.Unlock()

			if n.Pubkey != "" {
				nd.enrichNodeWithCredits(n)
			}
		}(node)
	}

	time.Sleep(2 * time.Second)
}

// func (nd *NodeDiscovery) healthCheck() {
// 	nodes := nd.GetNodes()

// 	for _, node := range nodes {
// 		nd.rateLimiter <- struct{}{}

// 		go func(n *models.Node) {
// 			defer func() { <-nd.rateLimiter }()

// 			start := time.Now()
// 			verResp, err := nd.prpc.GetVersion(n.Address)
// 			latency := time.Since(start).Milliseconds()

// 			nd.nodesMutex.Lock()
// 			defer nd.nodesMutex.Unlock()

// 			storedNode, exists := nd.knownNodes[n.ID]
// 			if !exists {
// 				return
// 			}

// 			storedNode.ResponseTime = latency
// 			updateCallHistory(storedNode, err == nil)
// 			storedNode.TotalCalls++

// 			if err == nil {
// 				storedNode.SuccessCalls++
// 				storedNode.IsOnline = true
// 				storedNode.LastSeen = time.Now()
// 				storedNode.Version = verResp.Version

// 				versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(verResp.Version, nil)
// 				storedNode.VersionStatus = versionStatus
// 				storedNode.IsUpgradeNeeded = needsUpgrade
// 				storedNode.UpgradeSeverity = severity
// 				storedNode.UpgradeMessage = utils.GetUpgradeMessage(verResp.Version, nil)
// 			} else {
// 				storedNode.IsOnline = false
// 				/*
// 					if time.Since(storedNode.LastSeen) > time.Duration(nd.cfg.Polling.StaleThreshold)*time.Minute {
// 						// Remove from knownNodes
// 						delete(nd.knownNodes, storedNode.ID)

// 						// Remove from IP index
// 						nd.ipMutex.Lock()
// 						nodes := nd.ipToNodes[storedNode.IP]
// 						for i, n := range nodes {
// 							if n.ID == storedNode.ID {
// 								nd.ipToNodes[storedNode.IP] = append(nodes[:i], nodes[i+1:]...)
// 								break
// 							}
// 						}
// 						nd.ipMutex.Unlock()
// 					}
// 				*/

// 				if time.Since(storedNode.LastSeen) > time.Duration(nd.cfg.Polling.StaleThreshold)*time.Minute {
// 					storedNode.Status = "offline"
// 					storedNode.IsOnline = false
// 					// Node remains in knownNodes for tracking
// 					log.Printf("Node %s marked as offline (last seen: %v ago)",
// 						storedNode.ID, time.Since(storedNode.LastSeen))
// 				}
// 			}

// 			utils.DetermineStatus(storedNode)
// 			utils.CalculateScore(storedNode)
// 		}(node)
// 	}
// }


// FIND the healthCheck() function (around line 420-480)
// COMPLETELY REPLACE the section that handles failed health checks:

func (nd *NodeDiscovery) healthCheck() {
	nodes := nd.GetNodes()
	
	for _, node := range nodes {
		nd.rateLimiter <- struct{}{}
		
		go func(n *models.Node) {
			defer func() { <-nd.rateLimiter }()
			
			start := time.Now()
			verResp, err := nd.prpc.GetVersion(n.Address)
			latency := time.Since(start).Milliseconds()

			nd.nodesMutex.Lock()
			defer nd.nodesMutex.Unlock()

			storedNode, exists := nd.knownNodes[n.ID]
			if !exists {
				return
			}

			storedNode.ResponseTime = latency
			updateCallHistory(storedNode, err == nil)
			storedNode.TotalCalls++
			
			if err == nil {
				// Node is ONLINE
				storedNode.SuccessCalls++
				storedNode.IsOnline = true
				storedNode.LastSeen = time.Now()
				storedNode.Version = verResp.Version
				
				versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(verResp.Version, nil)
				storedNode.VersionStatus = versionStatus
				storedNode.IsUpgradeNeeded = needsUpgrade
				storedNode.UpgradeSeverity = severity
				storedNode.UpgradeMessage = utils.GetUpgradeMessage(verResp.Version, nil)
			} else {
				// Node is OFFLINE - CRITICAL FIX: DO NOT DELETE IT
				storedNode.IsOnline = false
				
				// Calculate how long it's been offline
				offlineDuration := time.Since(storedNode.LastSeen)
				
				// Only log occasionally to reduce spam
				if storedNode.TotalCalls%10 == 0 {
					log.Printf("Node %s offline for %v (keeping in database)", 
						storedNode.ID, offlineDuration.Round(time.Minute))
				}
				
				// REMOVED: The code that deleted nodes
				// We now KEEP all nodes regardless of offline duration
				// This allows proper tracking of offline/dead nodes
			}
			
			// Always recalculate status and score for ALL nodes
			utils.DetermineStatus(storedNode)
			utils.CalculateScore(storedNode)
		}(node)
	}
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

// func (nd *NodeDiscovery) updateNodeFromPod(node *models.Node, pod *models.Pod) {
// 	node.Pubkey = pod.Pubkey
// 	node.IsPublic = pod.IsPublic

// 	if pod.Version != "" {
// 		node.Version = pod.Version
// 	}

// 	if pod.StorageCommitted > 0 {
// 		node.StorageCapacity = pod.StorageCommitted
// 		node.StorageUsed = pod.StorageUsed
// 		node.StorageUsagePercent = pod.StorageUsagePercent
// 	}

// 	if pod.Uptime > 0 {
// 		node.UptimeSeconds = pod.Uptime
// 	}

// 	if pod.LastSeenTimestamp > 0 {
// 		node.LastSeen = time.Unix(pod.LastSeenTimestamp, 0)
// 	}

// 	if pod.Version != "" {
// 		versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(pod.Version, nil)
// 		node.VersionStatus = versionStatus
// 		node.IsUpgradeNeeded = needsUpgrade
// 		node.UpgradeSeverity = severity
// 		node.UpgradeMessage = utils.GetUpgradeMessage(pod.Version, nil)
// 	}
// }



func (nd *NodeDiscovery) updateNodeFromPod(node *models.Node, pod *models.Pod) {
	// Update pubkey
	if pod.Pubkey != "" {
		node.Pubkey = pod.Pubkey
	}
	
	// CRITICAL: Update is_public flag
	node.IsPublic = pod.IsPublic
	
	// Update version
	if pod.Version != "" {
		node.Version = pod.Version
		
		// Update version status
		versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(pod.Version, nil)
		node.VersionStatus = versionStatus
		node.IsUpgradeNeeded = needsUpgrade
		node.UpgradeSeverity = severity
		node.UpgradeMessage = utils.GetUpgradeMessage(pod.Version, nil)
	}
	
	// Update storage info
	if pod.StorageCommitted > 0 {
		node.StorageCapacity = pod.StorageCommitted
		node.StorageUsed = pod.StorageUsed
		node.StorageUsagePercent = pod.StorageUsagePercent
	}
	
	// Update uptime
	if pod.Uptime > 0 {
		node.UptimeSeconds = pod.Uptime
		
		// Recalculate uptime score based on pod data
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
	
	// Update last seen from pod timestamp
	if pod.LastSeenTimestamp > 0 {
		podLastSeen := time.Unix(pod.LastSeenTimestamp, 0)
		
		// Only update if pod's timestamp is more recent
		if podLastSeen.After(node.LastSeen) {
			node.LastSeen = podLastSeen
			
			// If last seen is recent (within 2 minutes), mark as online
			if time.Since(podLastSeen) < 2*time.Minute {
				node.IsOnline = true
			}
		}
	}
	
	// Update RPC port in addresses if different
	if pod.RpcPort > 0 && pod.RpcPort != node.Port {
		node.Port = pod.RpcPort
		node.Address = net.JoinHostPort(node.IP, strconv.Itoa(pod.RpcPort))
		
		// Update or add address entry
		found := false
		for i := range node.Addresses {
			if node.Addresses[i].IP == node.IP {
				node.Addresses[i].Port = pod.RpcPort
				node.Addresses[i].Address = node.Address
				node.Addresses[i].LastSeen = time.Now()
				found = true
				break
			}
		}
		if !found {
			node.Addresses = append(node.Addresses, models.NodeAddress{
				Address:   node.Address,
				IP:        node.IP,
				Port:      pod.RpcPort,
				Type:      "rpc",
				LastSeen:  time.Now(),
				IsWorking: node.IsOnline,
				IsPublic:  pod.IsPublic,
			})
		}
	}
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













// createNodeFromPod creates or updates a node from pod information
func (nd *NodeDiscovery) createNodeFromPod(pod *models.Pod) {
	podHost, _, err := net.SplitHostPort(pod.Address)
	if err != nil {
		podHost = pod.Address
	}

	var rpcAddress string
	if pod.RpcPort > 0 {
		rpcAddress = net.JoinHostPort(podHost, strconv.Itoa(pod.RpcPort))
	} else {
		rpcAddress = net.JoinHostPort(podHost, "6000")
	}

	nodeID := rpcAddress
	if pod.Pubkey != "" {
		nodeID = pod.Pubkey
	}

	nd.nodesMutex.Lock()
	existingNode, exists := nd.knownNodes[nodeID]
	nd.nodesMutex.Unlock()

	if exists {
		// Update existing node with pod data
		nd.nodesMutex.Lock()
		nd.updateNodeFromPod(existingNode, pod)
		nd.nodesMutex.Unlock()
		return
	}

	// Create new node from pod data
	now := time.Now()
	podLastSeen := time.Unix(pod.LastSeenTimestamp, 0)
	
	// Determine if node is likely online based on last seen
	isOnline := time.Since(podLastSeen) < 5*time.Minute
	status := "offline"
	if isOnline {
		status = "online"
	}

	newNode := &models.Node{
		ID:               nodeID,
		Pubkey:           pod.Pubkey,
		Address:          rpcAddress,
		IP:               podHost,
		Port:             pod.RpcPort,
		Version:          pod.Version,
		IsOnline:         isOnline,
		IsPublic:         pod.IsPublic, // CRITICAL: Set from pod data
		FirstSeen:        now,
		LastSeen:         podLastSeen,
		Status:           status,
		UptimeScore:      0,
		PerformanceScore: 0,
		CallHistory:      make([]bool, 0),
		StorageCapacity:  pod.StorageCommitted,
		StorageUsed:      pod.StorageUsed,
		UptimeSeconds:    pod.Uptime,
		Addresses: []models.NodeAddress{
			{
				Address:   rpcAddress,
				IP:        podHost,
				Port:      pod.RpcPort,
				Type:      "rpc",
				IsPublic:  pod.IsPublic,
				LastSeen:  podLastSeen,
				IsWorking: isOnline,
			},
		},
	}

	// Calculate uptime score from pod data
	if pod.Uptime > 0 {
		// Assume pod has been running for at least its uptime
		newNode.UptimeScore = 95.0 // Default to high score for nodes in peer list
	}

	// Version status
	if pod.Version != "" {
		versionStatus, needsUpgrade, severity := utils.CheckVersionStatus(pod.Version, nil)
		newNode.VersionStatus = versionStatus
		newNode.IsUpgradeNeeded = needsUpgrade
		newNode.UpgradeSeverity = severity
		newNode.UpgradeMessage = utils.GetUpgradeMessage(pod.Version, nil)
	}

	// GeoIP
	country, city, lat, lon := nd.geo.Lookup(podHost)
	newNode.Country = country
	newNode.City = city
	newNode.Lat = lat
	newNode.Lon = lon

	// Calculate initial scores
	utils.CalculateScore(newNode)
	utils.DetermineStatus(newNode)

	// Store node
	nd.nodesMutex.Lock()
	nd.knownNodes[nodeID] = newNode
	nd.nodesMutex.Unlock()

	// Add to IP index
	nd.ipMutex.Lock()
	nd.ipToNodes[podHost] = append(nd.ipToNodes[podHost], newNode)
	nd.ipMutex.Unlock()

	// Enrich with credits
	if pod.Pubkey != "" {
		nd.enrichNodeWithCredits(newNode)
	}

	log.Printf("Created node from pod: %s (%s, %s, public=%v, status=%s)", 
		rpcAddress, country, pod.Version, pod.IsPublic, status)
}