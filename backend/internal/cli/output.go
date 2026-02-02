package cli

import (
	"fmt"
	"strings"
)

func printHeader(format string, args ...interface{}) {
	fmt.Println()
	fmt.Printf("=== "+format+" ===\n", args...)
	fmt.Println()
}

func printInfo(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func printSuccess(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func printWarning(format string, args ...interface{}) {
	fmt.Printf("Warning: "+format+"\n", args...)
}

func printError(format string, args ...interface{}) {
	fmt.Printf("Error: "+format+"\n", args...)
}

func printDivider() {
	fmt.Println(strings.Repeat("-", 50))
}
