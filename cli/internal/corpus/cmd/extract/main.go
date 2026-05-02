package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

func main() {
	input := flag.String("input", "", "input markdown file")
	slug := flag.String("slug", "", "document slug for canonical refs")
	output := flag.String("output", "", "output JSON file")
	flag.Parse()

	if *input == "" || *slug == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: extract --input <file.md> --slug <slug> --output <file.json>")
		os.Exit(2)
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *input, err)
		os.Exit(1)
	}

	result := corpus.ExtractMarkdown(string(data), *slug)

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *output, err)
		os.Exit(1)
	}
}
