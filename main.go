package main

import (
	"fmt"
	"os"

	"github.com/kothavade/mastodon-paper/collect_data"
	"github.com/kothavade/mastodon-paper/filter"
	"github.com/kothavade/mastodon-paper/graph"
	"github.com/kothavade/mastodon-paper/injest"
	"github.com/kothavade/mastodon-paper/injest_data"
	"github.com/kothavade/mastodon-paper/process"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: go run main.go <filter|process|collect_data>")
		return
	}

	switch args[0] {
	// Filter nodes.json to software that supports the peers API
	case "filter":
		filter.FilterNodes()
	// Create nodes in neo4j for all nodes
	case "graph-init":
		graph.Init()

	case "collect_data":
		collect_data.CollectData()

	case "process":
		process.ProcessNodes()
	// Injest the relationships into neo4j
	case "injest":
		injest.Injest()
	// Injest the data for nodes into neo4j
	case "injest_data":
		injest_data.InjestData()
	default:
		fmt.Println("Usage: go run main.go <filter|process|collect_data>")
	}
}
