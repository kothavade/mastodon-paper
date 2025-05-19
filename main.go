package main

import (
	"fmt"
	"os"

	"github.com/kothavade/mastodon-paper/filter"
	"github.com/kothavade/mastodon-paper/graph"
	"github.com/kothavade/mastodon-paper/injest"
	"github.com/kothavade/mastodon-paper/process"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: go run main.go <filter|process>")
		return
	}

	switch args[0] {
	// Filter nodes.json to software that supports the peers API
	case "filter":
		filter.FilterNodes()
	// Create nodes in neo4j for all nodes
	case "graph-init":
		graph.Init()
	// Download the peers for all nodes
	case "process":
		process.ProcessNodes()
	// Injest the relationships into neo4j
	case "injest":
		injest.Injest()
	default:
		fmt.Println("Usage: go run main.go <filter|process>")
	}
}
