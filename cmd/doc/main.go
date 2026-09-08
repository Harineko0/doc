package main

import (
	"fmt"
	"os"

	"github.com/Harineko0/doc/internal/app"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}

	switch args[0] {
	case "lint":
		staged := false
		switch len(args) {
		case 1:
		case 2:
			if args[1] != "--staged" {
				fmt.Fprintf(os.Stderr, "doc lint: unknown argument %q\n", args[1])
				return 2
			}
			staged = true
		default:
			fmt.Fprintln(os.Stderr, "usage: doc lint [--staged]")
			return 2
		}
		findings, err := app.Lint(staged)
		if err != nil {
			fmt.Fprintf(os.Stderr, "doc lint: %v\n", err)
			return 2
		}
		for _, finding := range findings {
			fmt.Fprintln(os.Stdout, finding)
		}
		if len(findings) != 0 {
			return 1
		}
		return 0

	case "mv":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: doc mv <source> <destination>")
			return 2
		}
		result, err := app.Move(args[1], args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "doc mv: %v\n", err)
			if app.IsInternalError(err) {
				return 2
			}
			return 1
		}
		fmt.Fprintf(os.Stdout, "moved %s -> %s; updated %d Markdown file(s)\n", result.Source, result.Destination, result.Updated)
		return 0

	case "help", "--help", "-h":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "doc: unknown command %q\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  doc lint [--staged]")
	fmt.Fprintln(os.Stderr, "  doc mv <source> <destination>")
}
