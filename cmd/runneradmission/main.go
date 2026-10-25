// runneradmission is the host-owned ACTIONS_RUNNER_HOOK_JOB_STARTED executable.
package main

import (
	"fmt"
	"os"

	"github.com/araihu/assets/internal/runneradmission"
)

func main() {
	c, err := runneradmission.Load()
	if err == nil {
		err = runneradmission.Admit(c, runneradmission.DefaultPaths())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hostinger admission denied:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "hostinger admission allowed: %s %s\n", c.Event, c.Repository)
}
