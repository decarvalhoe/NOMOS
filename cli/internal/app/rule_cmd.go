package app

// VRC-42 (#578) — `nomos rule exec`: run computable atoms through a BORROWED
// substrate.
//
// A formula atom is declared, never guessed: a fenced block tagged `formula` is
// one, and nothing else is. Inferring which prose "looks computable" would be
// the first step towards the rule engine the plan's anti-goal §10.3 forbids.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"github.com/RBOKproject/Nomos/cli/internal/ruleexec"
)

// FormulaBlockLanguage is the fence tag that declares a computable atom.
const FormulaBlockLanguage = "formula"

func ruleCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	subcommands := map[string]commandFunc{
		"exec": ruleExec,
	}
	if len(args) == 0 {
		ruleHelp(stdout)
		return 0
	}
	cmd, ok := subcommands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "unknown rule subcommand %q\n\n", args[0])
		ruleHelp(stderr)
		return 2
	}
	return cmd(args[1:], stdout, stderr)
}

func ruleHelp(w io.Writer) {
	fmt.Fprintln(w, "nomos rule <subcommand> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  exec   Execute ```formula atoms through an external substrate")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "NOMOS computes nothing itself: --substrate-cmd is required, and a")
	fmt.Fprintln(w, "substrate that fails produces no values at all.")
}

// collectFormulas turns a document into the formula atoms it declares.
func collectFormulas(source string, path string, docRef string) ([]ruleexec.Formula, error) {
	ast := atomization.ParseMarkdown(source)
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef: docRef,
		SourceFile:  path,
	})

	formulaBlocks := map[string]atomization.Block{}
	for _, block := range ast.Blocks {
		if block.Props == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(block.Props["language"]), FormulaBlockLanguage) {
			formulaBlocks[block.ID] = block
		}
	}

	var formulas []ruleexec.Formula
	for _, atom := range set.Atoms {
		block, ok := formulaBlocks[atom.BlockID]
		if !ok {
			continue
		}
		formulas = append(formulas, ruleexec.Formula{
			AtomID: atom.ID,
			// The expression is the block's own text, verbatim. NOMOS does not
			// rewrite, normalise or interpret it on the way out.
			Expression: strings.TrimSpace(block.Text),
			Trace: ruleexec.SourceTrace{
				CanonicalRef: atom.CanonicalRef,
				File:         atom.SourceSpan.File,
				StartLine:    atom.SourceSpan.StartLine,
				EndLine:      atom.SourceSpan.EndLine,
			},
		})
	}
	return formulas, nil
}

func parseParameters(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, pair := range pairs {
		name, value, found := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, fmt.Errorf("parameter %q is not name=value", pair)
		}
		out[name] = value
	}
	return out, nil
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func ruleExec(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("rule exec", flag.ContinueOnError)
	flags.SetOutput(stderr)
	substrateCmd := flags.String("substrate-cmd", "",
		"external substrate command (required) — NOMOS computes nothing itself")
	docRef := flags.String("doc-ref", "", "document reference slug")
	timeout := flags.Duration("timeout", ruleexec.DefaultTimeout, "substrate timeout")
	var params stringList
	flags.Var(&params, "param", "parameter name=value, repeatable")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos rule exec --substrate-cmd CMD [--param k=v] <file.md>")
		return 2
	}

	// Fail-closed at the door: without a substrate there is nothing to run, and
	// NOMOS will not stand in for one.
	if strings.TrimSpace(*substrateCmd) == "" {
		fmt.Fprintf(stderr,
			"%s: --substrate-cmd is required; NOMOS borrows a rule engine and never substitutes for one\n",
			ruleexec.FindingSubstrateFailed)
		return 2
	}

	path := flags.Arg(0)
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	formulas, err := collectFormulas(string(source), path, *docRef)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", ruleexec.FindingSubstrateFailed, err)
		return 1
	}
	if len(formulas) == 0 {
		fmt.Fprintf(stderr,
			"%s: %s declares no ```%s block — nothing computable was found, which is not a result\n",
			ruleexec.FindingSubstrateFailed, path, FormulaBlockLanguage)
		return 2
	}

	parameters, err := parseParameters(params)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	for i := range formulas {
		formulas[i].Parameters = parameters
	}

	command := strings.Fields(*substrateCmd)
	record, err := ruleexec.Execute(
		ruleexec.External{Command: command, Timeout: *timeout},
		command,
		formulas,
	)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	// The engine verifies its own record before emitting it.
	if err := ruleexec.VerifyRecord(record, formulas); err != nil {
		fmt.Fprintf(stderr, "%s: refusing to emit a record that does not verify: %v\n",
			ruleexec.FindingSubstrateFailed, err)
		return 1
	}

	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

var _ = time.Second // keep the duration flag's package referenced across refactors
