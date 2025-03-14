package main

import (
	"os"
)

func main() {
	rootCmd := getRootCmd()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
