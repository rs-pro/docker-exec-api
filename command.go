package dea

import (
	"bytes"
	"time"
)

type Command struct {
	Command   string        `json:"command"`
	Args      []string      `json:"args"`
	Output    []*OutputLine `json:"-"`
	StartedAt *time.Time    `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at"`
	ExitCode  *int          `json:"exit_code"`
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
