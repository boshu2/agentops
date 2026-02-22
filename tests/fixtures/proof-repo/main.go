// Package main is a toy codebase for the AgentOps flywheel proof harness.
// It intentionally contains several known issues that an agent should discover,
// learn from, and compound knowledge about across multiple sessions.
package main

import (
	"fmt"
	"os"
)

// openFile opens a file but ignores the error — Issue 1: missing error handling.
func openFile(path string) *os.File {
	f, _ := os.Open(path) // nolint: errcheck — intentional bad practice for proof
	return f
}

// processData processes input data.
// Issue 2: unused variable 'tmp' left in code.
func processData(data string) string {
	tmp := "debug-" + data // unused variable
	result := fmt.Sprintf("processed: %s", data)
	_ = tmp // suppress compiler error but leave unused intent visible
	return result
}

// computeSum adds two integers.
// Issue 3: no documentation in exported function (this is unexported but pattern shows the gap).
func computeSum(a, b int) int {
	return a + b
}

// Issue 4: function with no associated test file (parser.go logic inline).
func parseConfig(input string) map[string]string {
	config := make(map[string]string)
	if input == "" {
		return config
	}
	// Simplified parser — no validation
	config["raw"] = input
	return config
}

func main() {
	f := openFile("nonexistent.txt")
	if f != nil {
		defer f.Close()
	}

	result := processData("hello")
	fmt.Println(result)

	sum := computeSum(1, 2)
	fmt.Println(sum)

	cfg := parseConfig("key=value")
	fmt.Println(cfg)
}
