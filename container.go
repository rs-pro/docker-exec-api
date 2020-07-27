package dea

import (
	"bytes"
	"sync"
)

type Container struct {
	ID       string
	cond     *sync.Cond
	commands []*Command
	buffers  map[LineKind]*bytes.Buffer
}

func NewContainer() *Container {
	cnt := Container{
		cond: sync.NewCond(&sync.Mutex{}),
		commands: []*Command{
			{Command: "startup"},
		},
		buffers: map[LineKind]*bytes.Buffer{
			StdOut: bytes.NewBuffer([]byte{}),
			StdErr: bytes.NewBuffer([]byte{}),
		},
	}

	return &cnt
}

func (c *Container) GetCond() *sync.Cond {
	return c.cond
}

func (c *Container) GetCommands() []*Command {
	return c.commands
}
