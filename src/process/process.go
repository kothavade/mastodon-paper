package process

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Node processing status constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// initProcessDB initializes the SQLite database for processing state
func initProcessDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./node_process.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create process_nodes table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS process_nodes (
			domain TEXT PRIMARY KEY,
			status TEXT,
			error TEXT,
			last_updated TIMESTAMP,
			peers TEXT
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

// initializeProcessNodes inserts nodes into the database if they don't exist
func initializeProcessNodes(db *sql.DB, nodes []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO process_nodes (domain, status, last_updated, peers) 
		VALUES (?, ?, CURRENT_TIMESTAMP, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, node := range nodes {
		_, err := stmt.Exec(node, StatusPending, "")
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// getPendingNodes retrieves nodes that still need to be processed
func getPendingNodes(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT domain FROM process_nodes 
		WHERE status = ?
	`, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pendingNodes []string
	for rows.Next() {
		var node string
		if err := rows.Scan(&node); err != nil {
			return nil, err
		}
		pendingNodes = append(pendingNodes, node)
	}

	return pendingNodes, nil
}

// getProcessStats returns statistics about node processing
func getProcessStats(db *sql.DB) (total int, completed int, failed int, pending int, err error) {
	err = db.QueryRow("SELECT COUNT(*) FROM process_nodes").Scan(&total)
	if err != nil {
		return
	}

	err = db.QueryRow("SELECT COUNT(*) FROM process_nodes WHERE status = ?", StatusCompleted).Scan(&completed)
	if err != nil {
		return
	}

	err = db.QueryRow("SELECT COUNT(*) FROM process_nodes WHERE status = ?", StatusFailed).Scan(&failed)
	if err != nil {
		return
	}

	err = db.QueryRow("SELECT COUNT(*) FROM process_nodes WHERE status = ?", StatusPending).Scan(&pending)
	if err != nil {
		return
	}

	return
}

// NodeResult represents the result of fetching peers for a node
type NodeResult struct {
	Node  string
	Peers []string
	Error error
}

func ProcessNodes() {
	sqliteDB, err := initProcessDB()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize SQLite database: %v", err))
	}
	defer sqliteDB.Close()
	fmt.Println("SQLite database initialized for process state tracking.")

	nodes, err := os.ReadFile("filtered_nodes.json")
	if err != nil {
		fmt.Println("Error reading filtered_nodes.json:", err)
		return
	}

	var nodesList []string
	err = json.Unmarshal(nodes, &nodesList)
	if err != nil {
		fmt.Println("Error unmarshalling filtered_nodes.json:", err)
		return
	}

	// Initialize nodes in the SQLite database
	err = initializeProcessNodes(sqliteDB, nodesList)
	if err != nil {
		fmt.Println("Error initializing nodes in database:", err)
		return
	}

	// Get pending nodes
	pendingNodes, err := getPendingNodes(sqliteDB)
	if err != nil {
		fmt.Println("Error retrieving pending nodes:", err)
		return
	}
	fmt.Printf("Found %d pending nodes to process\n", len(pendingNodes))

	nodesSet := make(map[string]bool)
	for _, node := range nodesList {
		nodesSet[node] = true
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Create channels for worker pool
	jobs := make(chan string, len(pendingNodes))
	results := make(chan NodeResult, len(pendingNodes))
	var wg sync.WaitGroup

	// Start workers
	numWorkers := 10
	fmt.Printf("Starting %d workers to process nodes\n", numWorkers)
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func() {
			worker(client, sqliteDB, jobs, results, &wg, nodesSet)
		}()
	}

	// Send jobs to workers
	for _, node := range pendingNodes {
		jobs <- node
	}
	close(jobs)

	// Wait for all jobs to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	processedCount := 0
	for range results {
		processedCount++
		if processedCount%10 == 0 || processedCount == len(pendingNodes) {
			fmt.Printf("Processed %d/%d nodes\n", processedCount, len(pendingNodes))
		}
	}

	// Display final stats
	total, completed, failed, pending, err := getProcessStats(sqliteDB)
	if err != nil {
		fmt.Println("Error getting process stats:", err)
	} else {
		fmt.Printf("\nProcessing complete. Stats:\n")
		fmt.Printf("Total nodes: %d\n", total)
		fmt.Printf("Completed: %d\n", completed)
		fmt.Printf("Failed: %d\n", failed)
		fmt.Printf("Pending: %d\n", pending)
	}
}

// worker processes jobs from the jobs channel
func worker(client *http.Client, db *sql.DB, jobs <-chan string, results chan<- NodeResult, wg *sync.WaitGroup, nodesSet map[string]bool) {
	defer wg.Done()

	for node := range jobs {
		// Update status to processing
		updateNodeStatus(db, node, StatusProcessing, "")

		// Construct API endpoint
		endpoint := fmt.Sprintf("https://%s/api/v1/instance/peers", node)

		// Fetch peers
		peers, err := fetchAPIData(client, endpoint)

		// Update database with result
		if err != nil {
			updateNodeStatus(db, node, StatusFailed, err.Error())
		} else {
			// Convert peers to JSON string
			var filteredPeers []string
			for _, peer := range peers {
				if nodesSet[peer] {
					filteredPeers = append(filteredPeers, peer)
				}
			}
			peersJSON, err := json.Marshal(filteredPeers)
			if err != nil {
				updateNodeStatus(db, node, StatusFailed, fmt.Sprintf("Error marshalling peers: %v", err))
			} else {
				updateNodeWithPeers(db, node, string(peersJSON))
			}
		}

		// Send minimal result to the results channel - we don't use the full data in the result handler
		results <- NodeResult{Node: node}
	}
}

// updateNodeStatus updates the status and error message for a node
func updateNodeStatus(db *sql.DB, node string, status string, errorMsg string) error {
	_, err := db.Exec(`
		UPDATE process_nodes 
		SET status = ?, error = ?, last_updated = CURRENT_TIMESTAMP 
		WHERE domain = ?
	`, status, errorMsg, node)

	return err
}

// updateNodeWithPeers updates a node with its peers and marks it as completed
func updateNodeWithPeers(db *sql.DB, node string, peersJSON string) error {
	_, err := db.Exec(`
		UPDATE process_nodes 
		SET status = ?, peers = ?, last_updated = CURRENT_TIMESTAMP, error = NULL
		WHERE domain = ?
	`, StatusCompleted, peersJSON, node)

	return err
}

// fetchAPIData retrieves JSON data from the given endpoint
func fetchAPIData(client *http.Client, endpoint string) ([]string, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-OK status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data []string
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
