package cli

import (
	"fmt"
	"os"
	"strings"
)

// PrintError prints an error message to stderr
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// PrintSuccess prints a success message
func PrintSuccess(format string, args ...interface{}) {
	fmt.Printf("✅ "+format+"\n", args...)
}

// PrintWarning prints a warning message
func PrintWarning(format string, args ...interface{}) {
	fmt.Printf("⚠️  "+format+"\n", args...)
}

// PrintInfo prints an info message
func PrintInfo(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// PrintHeader prints a section header
func PrintHeader(title string) {
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", len(title)))
}

// PrintDivider prints a visual divider
func PrintDivider() {
	fmt.Println(strings.Repeat("-", 70))
}
