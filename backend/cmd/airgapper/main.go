// Airgapper - Consensus-based encrypted backup system
package main

import "github.com/lcrostarosa/airgapper/backend/internal/cli"

func main() {
	cli.SetVersion("0.4.0")
	cli.Execute()
}
