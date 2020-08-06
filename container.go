package dea

import (
	"bytes"
	"context"
	"sync"
	"time"
)

type Container struct {
	ID string `json:"id"`
	// Cond is a condition variable to signal new terminal output presence in commands
	Cond *sync.Cond `json:"-"`
	// StdinCond is a condition variable to signal that command is finished
	StdinCond *sync.Cond `json:"-"`
	StoppedAt *time.Time `json:"stopped_at"`
	Error     error      `json:"error"`
	commands  []*Command
	buffers   map[LineKind]*bytes.Buffer
	Ctx       context.Context `json:"-"`
}

func NewContainer() *Container {
	cnt := Container{
		Cond:      sync.NewCond(&sync.Mutex{}),
		StdinCond: sync.NewCond(&sync.Mutex{}),
		commands:  []*Command{},
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
	if len(c.commands) > 0 {
		cmd := c.commands[len(c.commands)-1]
		cmd.EndedAt = &t
	}

	c.commands = append(c.commands, &Command{
		Command:   command,
		StartedAt: &t,
	})
	c.Cond.L.Unlock()
}

func (c *Container) GetCommands() []*Command {
	return c.commands
}
