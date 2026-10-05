package main

import (
	"os"

	"github.com/x7ssss/tf-unlock/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
