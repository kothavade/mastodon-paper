package main

import (
	"fmt"
	"os"

	"github.com/kothavade/mastodon-paper/collect_data"
	"github.com/kothavade/mastodon-paper/filter"
	"github.com/kothavade/mastodon-paper/graph"
	"github.com/kothavade/mastodon-paper/process"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: go run main.go <filter|process|collect_data>")
		return
	}

	switch args[0] {
	case "filter":
		filter.FilterNodes()
	case "process":
		process.ProcessNodes()
	case "graph-init":
		graph.Init()
	case "collect_data":
		collect_data.CollectData()

	default:
		fmt.Println("Usage: go run main.go <filter|process|collect_data>")
	}
}
