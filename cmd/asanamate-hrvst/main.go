// Command asanamate-hrvst is asanamate's Harvest time tracking provider and
// feed. It serves one request of the time tracking or feed protocol
// (docs/extensions.md) on stdin, using the login of hrvst
// (https://github.com/kgajera/hrvst-cli).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const usage = `usage:
  asanamate-hrvst --task-id <id>   serve one time tracking or feed request on
                                   stdin (set as time_tracking.command, and
                                   add to today.feeds; feeds need no task ID)
  asanamate-hrvst check            check that hrvst is installed and logged in
`

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	err := serve(args, in, out)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintln(errOut, "asanamate-hrvst:", err)
		return 1
	}
	return 0
}

func serve(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 1 && args[0] == "check" {
		if err := checkHarvest(); err != nil {
			return err
		}
		fmt.Fprintln(out, "ok: hrvst installed and logged in")
		return nil
	}
	fs := flag.NewFlagSet("asanamate-hrvst", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	taskID := fs.String("task-id", "", "default Harvest task ID")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n%s", err, usage)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v\n%s", fs.Args(), usage)
	}
	configPath, err := harvestConfigPath()
	if err != nil {
		return err
	}
	return runHarvest(context.Background(), in, out, *taskID, configPath, harvestEndpoint)
}
