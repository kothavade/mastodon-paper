package injest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

func Injest() {
	ctx := context.Background()

	dbUri := "neo4j://localhost:7687"
	dbUser := "neo4j"
	dbPassword := "mastodonpaper"

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

	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	db, err := sql.Open("sqlite3", "node_process.db?cache=shared")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Get total count of nodes to process
	var totalNodes int
	err = db.QueryRow("SELECT COUNT(*) FROM process_nodes WHERE status = 'completed'").Scan(&totalNodes)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d nodes to process\n", totalNodes)

	rows, err := db.Query("SELECT domain, peers FROM process_nodes WHERE status = 'completed'")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	// Create a CSV file for Neo4j import
	csvPath := "domain_peers.csv"
	csvFile, err := os.Create(csvPath)
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()

	// Write CSV header
	_, err = csvFile.WriteString("domain,peer\n")
	if err != nil {
		panic(err)
	}

	i := 0
	for rows.Next() {
		var domain string
		var peersJSON string
		if err := rows.Scan(&domain, &peersJSON); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			continue
		}

		var peers []string
		if err := json.Unmarshal([]byte(peersJSON), &peers); err != nil {
			fmt.Printf("Error unmarshalling peers for %s: %v\n", domain, err)
			continue
		}

		// Write each domain-peer relationship to CSV
		for _, peer := range peers {
			_, err = csvFile.WriteString(fmt.Sprintf("%s,%s\n", domain, peer))
			if err != nil {
				fmt.Printf("Error writing to CSV: %v\n", err)
				continue
			}
		}

		fmt.Printf("Wrote %s (%d/%d) to CSV\n", domain, i+1, totalNodes)
		i++
	}

	absPath, err := filepath.Abs(csvPath)
	if err != nil {
		fmt.Printf("Error getting absolute path: %v\n", err)
		absPath = csvPath
	}
	fmt.Printf("CSV file created at: %s\n", absPath)
}
