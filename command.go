package dea

import (
	"bytes"
)

type Command struct {
	Command string
	Args    []string
	Output  []*OutputLine
}

func (c *Command) GetOutput() string {
	var out bytes.Buffer
	for _, line := range c.Output {
		out.Write([]byte("< "))
		out.Write(line.Content)
		out.Write([]byte("\n"))
	}
	return out.String()
}
