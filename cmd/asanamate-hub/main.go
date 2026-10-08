// Command asanamate-hub is asanamate's project links provider for imarc's
// Hub. It serves one request of the project links protocol
// (docs/extensions.md) on stdin, read-only, with a Hub service account's
// client credentials from HUB_CLIENT_ID and HUB_CLIENT_SECRET.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

const usage = `usage:
  asanamate-hub [--url <url>]   serve one project links request on stdin
                                (set as project_links.command)
  asanamate-hub check           check that HUB_CLIENT_ID and HUB_CLIENT_SECRET are set
`

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	err := serve(args, in, out)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintln(errOut, "asanamate-hub:", err)
		return 1
	}
	return 0
}

func serve(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 1 && args[0] == "check" {
		if err := checkHub(); err != nil {
			return err
		}
		fmt.Fprintln(out, "ok: Hub credentials set")
		return nil
	}
	fs := flag.NewFlagSet("asanamate-hub", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	url := fs.String("url", hubURL, "Hub base URL")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n%s", err, usage)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v\n%s", fs.Args(), usage)
	}
	return runHub(context.Background(), in, out, *url, os.Getenv(HubClientIDEnv), os.Getenv(HubClientSecretEnv), time.Now())
}
