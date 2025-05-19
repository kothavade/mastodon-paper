package filter

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

// NodeInfo represents the relevant parts of the nodeinfo response
type NodeInfo struct {
	Software struct {
		Name string `json:"name"`
	} `json:"software"`
}

// NodeInfoWellKnown represents the well-known nodeinfo response
type NodeInfoWellKnown struct {
	Links []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

// Node status constants
const (
	StatusPending  = "pending"
	StatusChecking = "checking"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
)

// initDB initializes the SQLite database
func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./node_filter.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create nodes table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			domain TEXT PRIMARY KEY,
			status TEXT,
			software TEXT,
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

func FilterNodes() {
	// Open nodes.json file
	nodes, err := os.ReadFile("nodes.json")
	if err != nil {
		fmt.Println("Error reading nodes.json:", err)
		return
	}

	var nodesList []string
	err = json.Unmarshal(nodes, &nodesList)
	if err != nil {
		fmt.Println("Error unmarshalling nodes.json:", err)
		return
	}

	// Initialize the database
	db, err := initDB()
	if err != nil {
		fmt.Println("Error initializing database:", err)
		return
	}
	defer db.Close()

	// Insert nodes that aren't in the database yet
	err = initializeNodes(db, nodesList)
	if err != nil {
		fmt.Println("Error initializing nodes in database:", err)
		return
	}

	// Filter nodes based on software
	filteredNodes, err := filterNodesBySoftware(db, nodesList)
	if err != nil {
		fmt.Println("Error filtering nodes:", err)
		return
	}

	// Write filtered nodes to a file
	filteredNodesFile, err := os.Create("filtered_nodes.json")
	if err != nil {
		fmt.Println("Error creating filtered_nodes.json:", err)
		return
	}
	defer filteredNodesFile.Close()

	err = json.NewEncoder(filteredNodesFile).Encode(filteredNodes)
	if err != nil {
		fmt.Println("Error writing to filtered_nodes.json:", err)
		return
	}

	fmt.Println("Filtered nodes written to filtered_nodes.json")

	// Get stats
	total, checked, supported, err := getNodeStats(db)
	if err != nil {
		fmt.Println("Error getting node stats:", err)
		return
	}

	fmt.Printf("Total nodes: %d, Checked: %d, Supported: %d\n", total, checked, supported)
}

// initializeNodes inserts nodes into the database if they don't exist
func initializeNodes(db *sql.DB, nodes []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO nodes (domain, status, last_updated) 
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

// getNodeStats returns statistics about the nodes in the database
func getNodeStats(db *sql.DB) (total int, checked int, supported int, err error) {
	err = db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&total)
	if err != nil {
		return
	}

	err = db.QueryRow("SELECT COUNT(*) FROM nodes WHERE status != ?", StatusPending).Scan(&checked)
	if err != nil {
		return
	}

	err = db.QueryRow("SELECT COUNT(*) FROM nodes WHERE status = ? AND software IN ('mastodon', 'pleroma', 'misskey', 'bookwyrm', 'smithereen')", StatusSuccess).Scan(&supported)
	if err != nil {
		return
	}

	return
}

// filterNodesBySoftware gets nodeinfo for each node and keeps only supported ones
func filterNodesBySoftware(db *sql.DB, nodes []string) ([]string, error) {
	supportedSoftware := map[string]bool{
		"mastodon":   true,
		"pleroma":    true,
		"misskey":    true,
		"bookwyrm":   true,
		"smithereen": true,
	}

	// Create a channel to receive results from goroutines
	resultChan := make(chan string)

	// Use a wait group to know when all goroutines are done
	var wg sync.WaitGroup

	// Limit the number of concurrent goroutines to avoid overwhelming the system
	concurrencyLimit := 10
	semaphore := make(chan struct{}, concurrencyLimit)

	// Start collecting results in the background
	var filteredNodes []string
	done := make(chan struct{})
	go func() {
		for node := range resultChan {
			filteredNodes = append(filteredNodes, node)
		}
		done <- struct{}{}
	}()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// First, collect already processed supported nodes
	rows, err := db.Query(`
		SELECT domain FROM nodes 
		WHERE status = ? AND software IN ('mastodon', 'pleroma', 'misskey', 'bookwyrm', 'smithereen')
	`, StatusSuccess)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var node string
		if err := rows.Scan(&node); err != nil {
			return nil, err
		}
		resultChan <- node
	}

	// Then process pending nodes
	pendingRows, err := db.Query(`
		SELECT domain FROM nodes 
		WHERE status = ?
	`, StatusPending)
	if err != nil {
		return nil, err
	}
	defer pendingRows.Close()

	var pendingNodes []string
	for pendingRows.Next() {
		var node string
		if err := pendingRows.Scan(&node); err != nil {
			return nil, err
		}
		pendingNodes = append(pendingNodes, node)
	}

	// Process each pending node
	for _, node := range pendingNodes {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire a slot

		go func(node string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release the slot when done

			fmt.Printf("Checking software for node: %s\n", node)

			// Update status to checking
			_, err := db.Exec(`
				UPDATE nodes 
				SET status = ?, last_updated = CURRENT_TIMESTAMP 
				WHERE domain = ?
			`, StatusChecking, node)
			if err != nil {
				fmt.Printf("  Error updating status for %s: %v\n", node, err)
				return
			}

			// First, get the nodeinfo link from the well-known endpoint
			nodeInfoURL, err := getNodeInfoURL(client, node)
			if err != nil {
				fmt.Printf("  Skipping %s: %v\n", node, err)
				// Update status to failed
				_, dbErr := db.Exec(`
					UPDATE nodes 
					SET status = ?, error = ?, last_updated = CURRENT_TIMESTAMP 
					WHERE domain = ?
				`, StatusFailed, err.Error(), node)
				if dbErr != nil {
					fmt.Printf("  Error updating status for %s: %v\n", node, dbErr)
				}
				return
			}

			// Then fetch the actual nodeinfo
			software, err := getNodeSoftware(client, nodeInfoURL)
			if err != nil {
				fmt.Printf("  Skipping %s: %v\n", node, err)
				// Update status to failed
				_, dbErr := db.Exec(`
					UPDATE nodes 
					SET status = ?, error = ?, last_updated = CURRENT_TIMESTAMP 
					WHERE domain = ?
				`, StatusFailed, err.Error(), node)
				if dbErr != nil {
					fmt.Printf("  Error updating status for %s: %v\n", node, dbErr)
				}
				return
			}

			// Check if it's one of our supported software types
			if supportedSoftware[software] {
				fmt.Printf("  Found supported software '%s' for %s\n", software, node)
				// Update status to success
				_, dbErr := db.Exec(`
					UPDATE nodes 
					SET status = ?, software = ?, last_updated = CURRENT_TIMESTAMP 
					WHERE domain = ?
				`, StatusSuccess, software, node)
				if dbErr != nil {
					fmt.Printf("  Error updating status for %s: %v\n", node, dbErr)
				}
				resultChan <- node
			} else {
				fmt.Printf("  Unsupported software '%s' for %s\n", software, node)
				// Update status to success (we successfully checked it, but it's not supported)
				_, dbErr := db.Exec(`
					UPDATE nodes 
					SET status = ?, software = ?, last_updated = CURRENT_TIMESTAMP 
					WHERE domain = ?
				`, StatusSuccess, software, node)
				if dbErr != nil {
					fmt.Printf("  Error updating status for %s: %v\n", node, dbErr)
				}
			}
		}(node)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(resultChan)

	// Wait for the result collection goroutine to finish
	<-done

	return filteredNodes, nil
}

// getNodeInfoURL retrieves the nodeinfo URL from the well-known endpoint
func getNodeInfoURL(client *http.Client, node string) (string, error) {
	wellKnownURL := fmt.Sprintf("https://%s/.well-known/nodeinfo", node)

	resp, err := client.Get(wellKnownURL)
	if err != nil {
		return "", fmt.Errorf("failed to access well-known endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("well-known endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read well-known response: %w", err)
	}

	var wellKnown NodeInfoWellKnown
	if err := json.Unmarshal(body, &wellKnown); err != nil {
		return "", fmt.Errorf("failed to parse well-known response: %w", err)
	}

	// Find the nodeinfo 2.0 or 2.1 link
	for _, link := range wellKnown.Links {
		if link.Rel == "http://nodeinfo.diaspora.software/ns/schema/2.0" ||
			link.Rel == "http://nodeinfo.diaspora.software/ns/schema/2.1" {
			return link.Href, nil
		}
	}

	return "", fmt.Errorf("no nodeinfo link found")
}

// getNodeSoftware retrieves the software name from nodeinfo
func getNodeSoftware(client *http.Client, nodeInfoURL string) (string, error) {
	resp, err := client.Get(nodeInfoURL)
	if err != nil {
		return "", fmt.Errorf("failed to access nodeinfo endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nodeinfo endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read nodeinfo response: %w", err)
	}

	var nodeInfo NodeInfo
	if err := json.Unmarshal(body, &nodeInfo); err != nil {
		return "", fmt.Errorf("failed to parse nodeinfo response: %w", err)
	}

	if nodeInfo.Software.Name == "" {
		return "", fmt.Errorf("software name not found in nodeinfo")
	}

	return nodeInfo.Software.Name, nil
}
