package exec

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"time"
)

var (
	now = time.Now
)

// Result is the result of executing a command.
type Result struct {
	Args []string

	Start, End     time.Time
	Stdout, Stderr []byte

	ExitCode int
	Err      error
}

// Run the command immediately, and wait for it to complete.
//
// The command's stdout and stderr will be plumbed through to the current stdout and stderr.
func Run(name string, args ...string) Result {
	c := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	c.Stdin = os.Stdin
	c.Stdout = io.MultiWriter(os.Stdout, &stdout)
	c.Stderr = io.MultiWriter(os.Stderr, &stderr)

	var res Result
	res.Args = append([]string{name}, args...)

	res.Start = now()
	err := c.Run()
	res.End = now()

	res.Stdout = stdout.Bytes()
	res.Stderr = stderr.Bytes()
	res.ExitCode = c.ProcessState.ExitCode()
	if _, ok := err.(*exec.ExitError); !ok {
		res.Err = err
	}

	return res
}
