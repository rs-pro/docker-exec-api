package dea

import (
	"sync"
)

type Command struct {
	Command string
	Args    []string
	Output  []*OutputLine
}

type Container struct {
	ID       string
	cond     *sync.Cond
	commands []*Command
}

func NewContainer() *Container {
	cnt := Container{
		cond: sync.NewCond(&sync.Mutex{}),
		commands: []*Command{
			{Command: "startup"},
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
