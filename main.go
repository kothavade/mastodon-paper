package main

import (
	"fmt"
	"os"

	"github.com/kothavade/mastodon-paper/filter"
	"github.com/kothavade/mastodon-paper/process"
)

func main() {

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: go run main.go <filter|process>")
		return
	}

	switch args[0] {
	case "filter":
		filter.FilterNodes()
	case "process":
		process.ProcessNodes()
	default:
		fmt.Println("Usage: go run main.go <filter|process>")
	}
}
