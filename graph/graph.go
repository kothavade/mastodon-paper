package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func Init() {
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

	// for each node in filtered_processed_nodes.json, create a node in the graph
	nodes, err := os.ReadFile("filtered_processed_nodes.json")
	if err != nil {
		panic(err)
	}

	var nodesList []string
	json.Unmarshal(nodes, &nodesList)

	for _, node := range nodesList {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, "CREATE (n:MastodonNode {url: $url})", map[string]any{"url": node})
			return nil, err
		})
		if err != nil {
			panic(err)
		}
		fmt.Println("Created node", node)
	}
}

func ImportPeerRelationships() {
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

	// Create constraint for unique domain names
	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			"CREATE CONSTRAINT domain_name IF NOT EXISTS FOR (d:MastodonNode) REQUIRE d.url IS UNIQUE;",
			nil)
		return nil, err
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Created unique constraint on MastodonNode urls")

	// Import CSV and create relationships
	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, `
			LOAD CSV WITH HEADERS FROM 'file:///domain_peers.csv' AS row
			MERGE (domain:MastodonNode {url: row.domain})
			MERGE (peer:MastodonNode {url: row.peer})
			MERGE (domain)-[:PEERS_WITH]->(peer)
			RETURN count(*) as relationships`,
			nil)
		if err != nil {
			return nil, err
		}

		record, err := result.Single(ctx)
		if err != nil {
			return nil, err
		}

		count := record.Values[0].(int64)
		fmt.Printf("Created %d peer relationships\n", count)
		return nil, nil
	})
	if err != nil {
		panic(err)
	}
}
