package filter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
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

func FilterNodes() {
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

	// Filter nodes based on software
	filteredNodes, err := filterNodesBySoftware(nodesList)
	if err != nil {
		fmt.Println("Error filtering nodes:", err)
		return
	}

	// write filtered nodes to a file
	filteredNodesFile, err := os.Create("filtered_nodes.json")
	if err != nil {
		fmt.Println("Error creating filtered_nodes.json:", err)
		return
	}
	defer filteredNodesFile.Close()
	json.NewEncoder(filteredNodesFile).Encode(filteredNodes)
	fmt.Println("Filtered nodes written to filtered_nodes.json")

	fmt.Printf("Filtered %d nodes to %d supported nodes\n", len(nodesList), len(filteredNodes))
}

// filterNodesBySoftware gets nodeinfo for each node and keeps only supported ones
func filterNodesBySoftware(nodes []string) ([]string, error) {
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

	for _, node := range nodes {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire a slot

		go func(node string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release the slot when done

			fmt.Printf("Checking software for node: %s\n", node)

			// First, get the nodeinfo link from the well-known endpoint
			nodeInfoURL, err := getNodeInfoURL(client, node)
			if err != nil {
				fmt.Printf("  Skipping %s: %v\n", node, err)
				return
			}

			// Then fetch the actual nodeinfo
			software, err := getNodeSoftware(client, nodeInfoURL)
			if err != nil {
				fmt.Printf("  Skipping %s: %v\n", node, err)
				return
			}

			// Check if it's one of our supported software types
			if supportedSoftware[software] {
				fmt.Printf("  Found supported software '%s' for %s\n", software, node)
				resultChan <- node
			} else {
				fmt.Printf("  Unsupported software '%s' for %s\n", software, node)
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
