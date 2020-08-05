// Package main represents the main entrypoint of the seeshell application.
package main

import (
	"log"

	"github.com/antoniomika/seeshell/cmd"
)

// main will start the seeshell command lifecycle and spawn the seeshell services.
func main() {
	err := cmd.Execute()
	if err != nil {
		log.Println("Unable to execute root command:", err)
	}
}
