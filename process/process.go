package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
	db, err := sql.Open("sqlite3", "./node_filter.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create process_nodes table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS process_nodes (
			domain TEXT PRIMARY KEY,
			status TEXT,
			error TEXT,
			last_updated TIMESTAMP
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
		INSERT OR IGNORE INTO process_nodes (domain, status, last_updated) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, node := range nodes {
		_, err := stmt.Exec(node, StatusPending)
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

func ProcessNodes() {
	ctx := context.Background()
	dbUri := "neo4j+s://8bd77a03.databases.neo4j.io"
	dbUser := "neo4j"
	dbPassword := "pcvwVnqKqAgqqxGfPSbQ3kbH6pFtLjNu-Ufrqp33m1E"

	driver, err := neo4j.NewDriverWithContext(
		dbUri,
		neo4j.BasicAuth(dbUser, dbPassword, ""))
	if err != nil {
		panic(err)
	}
	defer driver.Close(ctx)

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("Neo4j connection established.")

	// Initialize SQLite database for process state
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
		Timeout: 10 * time.Second,
	}

	// Set up a worker pool
	numWorkers := 5 // Adjust based on needs
	var wg sync.WaitGroup
	nodeQueue := make(chan string, len(pendingNodes))

	// Launch workers
	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for node := range nodeQueue {
				processNodeWithState(ctx, sqliteDB, driver, client, node, workerID, nodesSet)
			}
		}(i + 1)
	}

	// Add all pending nodes to the queue
	for _, node := range pendingNodes {
		nodeQueue <- node
	}
	close(nodeQueue)

	// Wait for all workers to finish
	wg.Wait()

	// Print final stats
	total, completed, failed, pending, err := getProcessStats(sqliteDB)
	if err != nil {
		fmt.Println("Error getting process stats:", err)
	} else {
		fmt.Printf("Processing complete. Total: %d, Completed: %d, Failed: %d, Pending: %d\n",
			total, completed, failed, pending)
	}
}

// processNodeWithState handles fetching and storing data for a single node with state tracking
func processNodeWithState(
	ctx context.Context,
	sqliteDB *sql.DB,
	driver neo4j.DriverWithContext,
	client *http.Client,
	node string,
	workerID int,
	nodesSet map[string]bool,
) {
	fmt.Printf("Worker %d processing node: %s\n", workerID, node)

	// Update status to processing
	_, err := sqliteDB.Exec(`
		UPDATE process_nodes 
		SET status = ?, last_updated = CURRENT_TIMESTAMP 
		WHERE domain = ?
	`, StatusProcessing, node)
	if err != nil {
		fmt.Printf("Worker %d: Error updating status for %s: %v\n", workerID, node, err)
		return
	}

	// Get peers for this node
	fmt.Printf("Worker %d: Fetching peers for %s\n", workerID, node)
	peers, err := fetchAPIData(client, "https://"+node+"/api/v1/instance/peers")
	if err != nil {
		fmt.Printf("Worker %d: Error fetching peers for %s: %v\n", workerID, node, err)

		// Update status to failed
		_, dbErr := sqliteDB.Exec(`
			UPDATE process_nodes 
			SET status = ?, error = ?, last_updated = CURRENT_TIMESTAMP 
			WHERE domain = ?
		`, StatusFailed, err.Error(), node)
		if dbErr != nil {
			fmt.Printf("Worker %d: Error updating status for %s: %v\n", workerID, node, dbErr)
		}
		return
	}

	// Store data in Neo4j
	fmt.Printf("Worker %d: Storing data for %s\n", workerID, node)
	err = storeDataInNeo4j(ctx, driver, node, peers, nodesSet)
	if err != nil {
		fmt.Printf("Worker %d: Error storing data for %s: %v\n", workerID, node, err)

		// Update status to failed
		_, dbErr := sqliteDB.Exec(`
			UPDATE process_nodes 
			SET status = ?, error = ?, last_updated = CURRENT_TIMESTAMP 
			WHERE domain = ?
		`, StatusFailed, err.Error(), node)
		if dbErr != nil {
			fmt.Printf("Worker %d: Error updating status for %s: %v\n", workerID, node, dbErr)
		}
		return
	}

	// Update status to completed
	_, err = sqliteDB.Exec(`
		UPDATE process_nodes 
		SET status = ?, last_updated = CURRENT_TIMESTAMP 
		WHERE domain = ?
	`, StatusCompleted, node)
	if err != nil {
		fmt.Printf("Worker %d: Error updating status for %s: %v\n", workerID, node, err)
	}

	fmt.Printf("Worker %d: Successfully processed node: %s\n", workerID, node)
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

// storeDataInNeo4j saves the node, its peers, and domain blocks to the Neo4j database
func storeDataInNeo4j(ctx context.Context, driver neo4j.DriverWithContext, nodeURL string, peers []string, nodesSet map[string]bool) error {
	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Create the main node
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Create or merge the main node
		_, err := tx.Run(ctx,
			"MERGE (n:MastodonNode {url: $url})",
			map[string]any{
				"url": nodeURL,
			})
		if err != nil {
			return nil, err
		}

		// Process peers
		for _, peer := range peers {
			if !nodesSet[peer] {
				continue
			}

			_, err = tx.Run(ctx,
				`MERGE (p:MastodonNode {url: $peerURL})
				 WITH p
				 MATCH (n:MastodonNode {url: $nodeURL})
				 MERGE (n)-[r:PEERS_WITH]->(p)`,
				map[string]any{
					"peerURL": peer,
					"nodeURL": nodeURL,
				})
			if err != nil {
				return nil, err
			}
		}

		// Process domain blocks
		// for _, block := range domainBlocks {
		// 	_, err = tx.Run(ctx,
		// 		`MERGE (b:BlockedDomain {domain: $blockedDomain})
		// 		 WITH b
		// 		 MATCH (n:MastodonNode {url: $nodeURL})
		// 		 MERGE (n)-[r:BLOCKS]->(b)`,
		// 		map[string]any{
		// 			"blockedDomain": block,
		// 			"nodeURL":       nodeURL,
		// 		})
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// }

		return nil, nil
	})

	return err
}
