package process

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

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
	fmt.Println("Connection established.")

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

	nodesSet := make(map[string]bool)
	for _, node := range nodesList {
		nodesSet[node] = true
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Set up a worker pool
	numWorkers := 10 // Adjust based on needs
	var wg sync.WaitGroup
	nodeQueue := make(chan string, len(nodesList))

	// Launch workers
	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for node := range nodeQueue {
				processNode(ctx, driver, client, node, workerID, nodesSet)
			}
		}(i + 1)
	}

	// Add all nodes to the queue
	for _, node := range nodesList {
		nodeQueue <- node
	}
	close(nodeQueue)

	// Wait for all workers to finish
	wg.Wait()
	fmt.Println("Processing complete.")
}

// processNode handles fetching and storing data for a single node
func processNode(ctx context.Context, driver neo4j.DriverWithContext, client *http.Client,
	node string, workerID int, nodesSet map[string]bool) {

	fmt.Printf("Worker %d processing node: %s\n", workerID, node)

	// Get peers for this node
	peers, err := fetchAPIData(client, "https://"+node+"/api/v1/instance/peers")
	if err != nil {
		fmt.Printf("Worker %d: Error fetching peers for %s: %v\n", workerID, node, err)
		return
	}

	// // Get domain blocks for this node
	// domainBlocks, err := fetchAPIData(client, "https://"+node+"/api/v1/instance/domain_blocks")
	// if err != nil {
	// 	fmt.Printf("Worker %d: Error fetching domain blocks for %s: %v\n", workerID, node, err)
	// 	return
	// }

	// Store data in Neo4j
	err = storeDataInNeo4j(ctx, driver, node, peers, nodesSet)
	if err != nil {
		fmt.Printf("Worker %d: Error storing data for %s: %v\n", workerID, node, err)
		return
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
