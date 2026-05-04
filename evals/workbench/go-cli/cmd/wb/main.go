// Command wb is a minimal CLI with calc and store subcommands.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/agentops/eval-workbench/go-cli/internal/calc"
	"github.com/agentops/eval-workbench/go-cli/internal/store"
)

const usage = `Usage: wb <command> [args]

Commands:
  calc <op> <a> <b>          Arithmetic: add, subtract, multiply, divide
  store set <key> <value>    Set a key-value pair
  store get <key>            Get a value by key
  store delete <key>         Delete a key
  store list                 List all key-value pairs`

func main() {
	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "calc":
		runCalc(os.Args[2:])
	case "store":
		runStore(os.Args[2:])
	default:
		fmt.Println(usage)
		os.Exit(1)
	}
}

func runCalc(args []string) {
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: wb calc <op> <a> <b>")
		os.Exit(1)
	}
	a, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid number %q: %v\n", args[1], err)
		os.Exit(1)
	}
	b, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid number %q: %v\n", args[2], err)
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		fmt.Println(calc.Add(a, b))
	case "subtract":
		fmt.Println(calc.Subtract(a, b))
	case "multiply":
		fmt.Println(calc.Multiply(a, b))
	case "divide":
		result, err := calc.Divide(a, b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
	default:
		fmt.Fprintf(os.Stderr, "unknown op %q (add, subtract, multiply, divide)\n", args[0])
		os.Exit(1)
	}
}

func storePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".wb-store.json")
}

func loadStore() *store.Store {
	s := store.New(storePath())
	if err := s.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "load store: %v\n", err)
		os.Exit(1)
	}
	return s
}

func runStore(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: wb store <set|get|delete|list> [args]")
		os.Exit(1)
	}
	switch args[0] {
	case "set":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: wb store set <key> <value>")
			os.Exit(1)
		}
		s := loadStore()
		if err := s.Set(args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("set %s = %s\n", args[1], args[2])
	case "get":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: wb store get <key>")
			os.Exit(1)
		}
		s := loadStore()
		v, err := s.Get(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(v)
	case "delete":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: wb store delete <key>")
			os.Exit(1)
		}
		s := loadStore()
		if err := s.Delete(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("deleted %s\n", args[1])
	case "list":
		s := loadStore()
		for k, v := range s.List() {
			fmt.Printf("%s = %s\n", k, v)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown store command %q\n", args[0])
		os.Exit(1)
	}
}
