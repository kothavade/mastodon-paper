package injest_data

import (
	"context"
	"database/sql"
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

func InjestData() {
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

	db, err := sql.Open("sqlite3", "node_filter.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Get total count of nodes to process
	var totalNodes int
	err = db.QueryRow("SELECT COUNT(*) FROM node_info WHERE status = 'success'").Scan(&totalNodes)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d nodes to process\n", totalNodes)

	rows, err := db.Query("SELECT domain, ip, asn, country_code, user_count, post_count, cloud_provider FROM node_info WHERE status = 'success'")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	// Create a CSV file for Neo4j import
	csvPath := "data.csv"
	csvFile, err := os.Create(csvPath)
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()

	// Write CSV header
	_, err = csvFile.WriteString("domain,ip,asn,country_code,user_count,post_count,cloud_provider\n")
	if err != nil {
		panic(err)
	}

	i := 0
	for rows.Next() {
		var domain string
		var ip string
		var asn string
		var country_code string
		var user_count int
		var post_count int
		var cloud_provider string
		if err := rows.Scan(&domain, &ip, &asn, &country_code, &user_count, &post_count, &cloud_provider); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			continue
		}

		// Write each domain-peer relationship to CSV
		_, err = csvFile.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d,%d,%s\n", domain, ip, asn, country_code, user_count, post_count, cloud_provider))
		if err != nil {
			fmt.Printf("Error writing to CSV: %v\n", err)
			continue
		}

		if i%100 == 0 {
			fmt.Printf("Wrote %d/%d to CSV\n", i+1, totalNodes)
		}
		i++
	}

	absPath, err := filepath.Abs(csvPath)
	if err != nil {
		fmt.Printf("Error getting absolute path: %v\n", err)
		absPath = csvPath
	}
	fmt.Printf("CSV file created at: %s\n", absPath)
}
