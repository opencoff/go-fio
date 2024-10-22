// tparse.go -- lex and parse test harness

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/opencoff/shlex"
)

// a command is an executor of one of the test-harness commands.
type Cmd interface {
	Run(e *TestEnv, args []string) error
	Name() string
	Reset()
}

// singleton struct to register test harness commands.
type commands struct {
	sync.Mutex
	once sync.Once
	cmds map[string]Cmd
}

// TestSuite captures the parsed contents of a test file (.t file)
type TestSuite struct {
	Cmd  Cmd
	Args []string
}

func ReadTest(fn string) ([]TestSuite, error) {
	var line string

	fd, err := os.Open(fn)
	if err != nil {
		return nil, err
	}

	defer fd.Close()

	tests := make([]TestSuite, 0, 4)
	b := bufio.NewScanner(fd)
	for n := 1; b.Scan(); n++ {
		part := strings.TrimSpace(b.Text())
		if len(part) == 0 || part[0] == '#' {
			continue
		}

		if part[len(part)-1] == '\\' {
			line += part[:len(part)-1]
			continue
		}

		line += part
		args, err := shlex.Split(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", fn, n, err)
		}

		line = ""
		nm := args[0]
		c, ok := Commands.cmds[nm]
		if !ok {
			return nil, fmt.Errorf("%s:%d: unknown command %s", fn, n, nm)
		}

		t := TestSuite{
			Cmd:  c,
			Args: args,
		}
		tests = append(tests, t)
	}
	if err = b.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	return tests, nil
}

// global list of all registered commands
var Commands commands

// Register a test-harness command; called by init() functions
// in the cmd_xxx.go files.
func RegisterCommand(cmd Cmd) {
	c := &Commands

	c.Lock()
	defer c.Unlock()

	c.once.Do(func() {
		c.cmds = make(map[string]Cmd)
	})

	nm := cmd.Name()
	if _, ok := c.cmds[nm]; ok {
		err := fmt.Sprintf("%s: command already registered", nm)
		panic(err)
	}

	c.cmds[nm] = cmd
}
