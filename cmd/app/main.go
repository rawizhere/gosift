package main

import (
	"fmt"
	"os"

	"github.com/rawizhere/gosift/internal/config"
)

func main() {
	if _, err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
