package dea

import (
	"bytes"
	"sync"
	"time"
)

type Container struct {
	ID string
	// Cond is a condition variable to signal new terminal output presence in commands
	Cond     *sync.Cond
	Stopped  bool
	commands []*Command
	buffers  map[LineKind]*bytes.Buffer
}

func NewContainer() *Container {
	cnt := Container{
		Stopped:  false,
		Cond:     sync.NewCond(&sync.Mutex{}),
		commands: []*Command{},
		buffers: map[LineKind]*bytes.Buffer{
			StdOut: bytes.NewBuffer([]byte{}),
			StdErr: bytes.NewBuffer([]byte{}),
		},
	}

	cnt.StartCommand("startup")

	return &cnt
}

func (c *Container) StartCommand(command string) {
	t := time.Now()
	c.Cond.L.Lock()
	cmd := c.commands[len(c.commands)-1]
	cmd.EndedAt = &t

	c.commands = append(c.commands, &Command{
		Command:   command,
		StartedAt: &t,
	})
	c.Cond.L.Unlock()
}

func (c *Container) GetCommands() []*Command {
	return c.commands
}
