// Command asanamate-ccmux is asanamate's agent adapter for ccmux. Set
// agents.command = "asanamate-ccmux" to show ccmux's sessions next to their
// tickets.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sadmachine/asanamate/internal/ccmux"
	"github.com/sadmachine/asanamate/internal/config"
)

const usage = `usage:
  asanamate-ccmux              print ccmux's sessions as asanamate agent lines
                               (agents.command = "asanamate-ccmux")
  asanamate-ccmux setup        add the agent status hook to Codex
  asanamate-ccmux hook codex   record a Codex hook event read on stdin
                               (installed by setup)
`

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

func run(args []string, in io.Reader, out io.Writer) int {
	var err error
	switch {
	case len(args) == 0:
		err = list(out)
	case args[0] == "setup" && len(args) == 1:
		err = runSetup(in, out)
	case args[0] == "hook":
		ccmux.RunHook(args[1:], in)
	case args[0] == "help" || args[0] == "-h" || args[0] == "--help":
		fmt.Fprint(out, usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, ccmux.Name+":", err)
		return 1
	}
	return 0
}

// list prints ccmux's sessions, with the states agents reported themselves.
func list(out io.Writer) error {
	stateDir, err := config.StateDir()
	if err != nil {
		return err
	}
	agents, err := ccmux.List(context.Background(), ccmux.HookDir(stateDir))
	if err != nil {
		return err
	}
	return ccmux.Print(out, agents)
}

func runSetup(in io.Reader, out io.Writer) error {
	home, err := ccmux.CodexHome()
	if err != nil {
		return err
	}
	return ccmux.OfferCodexHook(bufio.NewReader(in), out, home, executable())
}

// executable is the adapter path Codex hooks run: the one on PATH, which
// survives upgrades, else this binary.
func executable() string {
	if path, err := exec.LookPath(ccmux.Name); err == nil {
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
	}
	path, _ := os.Executable()
	return path
}
