package injest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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

	db, err := sql.Open("sqlite3", "node_process.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// add a new column to the process_nodes table that tracks injest status
	_, err = db.Exec("ALTER TABLE process_nodes ADD COLUMN injest_status TEXT")
	if err != nil {
		panic(err)
	}

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

	processedNodes := 0
	totalRelationships := 0

	for rows.Next() {
		var domain string
		var peersJSON string
		if err := rows.Scan(&domain, &peersJSON); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			continue
		}

		// Update injest status to processing
		_, err = db.Exec("UPDATE process_nodes SET injest_status = ? WHERE domain = ?", StatusProcessing, domain)
		if err != nil {
			fmt.Printf("Error updating injest status: %v\n", err)
			continue
		}

		var peers []string
		if err := json.Unmarshal([]byte(peersJSON), &peers); err != nil {
			fmt.Printf("Error unmarshalling peers for %s: %v\n", domain, err)
			_, _ = db.Exec("UPDATE process_nodes SET injest_status = ? WHERE domain = ?", StatusFailed, domain)
			continue
		}

		// Create relationships in Neo4j
		_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			for _, peer := range peers {
				_, err := tx.Run(ctx, `
					MATCH (a:MastodonNode {url: $domain})
					MATCH (b:MastodonNode {url: $peer})
					MERGE (a)-[:PEERS_WITH]->(b)
				`, map[string]any{
					"domain": domain,
					"peer":   peer,
				})
				if err != nil {
					return nil, err
				}
			}
			return nil, nil
		})

		if err != nil {
			fmt.Printf("Error creating relationships for %s: %v\n", domain, err)
			_, _ = db.Exec("UPDATE process_nodes SET injest_status = ? WHERE domain = ?", StatusFailed, domain)
			continue
		}

		// Update injest status to completed
		_, err = db.Exec("UPDATE process_nodes SET injest_status = ? WHERE domain = ?", StatusCompleted, domain)
		if err != nil {
			fmt.Printf("Error updating final injest status: %v\n", err)
			continue
		}

		processedNodes++
		totalRelationships += len(peers)
		fmt.Printf("Progress: %d/%d nodes processed (%d relationships created) - Latest: %s\n",
			processedNodes, totalNodes, totalRelationships, domain)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("Error iterating rows: %v\n", err)
	}

	fmt.Printf("\nProcessing complete!\n")
	fmt.Printf("Total nodes processed: %d\n", processedNodes)
	fmt.Printf("Total relationships created: %d\n", totalRelationships)
}
