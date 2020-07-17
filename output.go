package dea

import (
	"bytes"
	"io"
	"log"
	"time"
)

type LineKind int

const (
	StdIn LineKind = iota //stdin is not used
	StdOut
	StdErr
)

type OutputLine struct {
	Kind    LineKind
	Content []byte
	Time    time.Time
}

func (c *Container) processOutput(kind LineKind, p []byte) {
	log.Println("container output:", string(p))
	lines := bytes.Split(p, []byte("\n"))
	t := time.Now()

	ret := make([]*OutputLine, 0)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		ret = append(ret, &OutputLine{
			Kind:    kind,
			Content: line,
			Time:    t,
		})
	}
	c.appendLog(ret)
}

func (c *Container) appendLog(lines []*OutputLine) {
	c.cond.L.Lock()
	cmd := c.commands[len(c.commands)-1]
	for _, line := range lines {
		cmd.Output = append(cmd.Output, line)
	}
	c.cond.L.Unlock()
	c.cond.Broadcast()
}

type Writer struct {
	container *Container
	kind      LineKind
}

func (w Writer) Write(p []byte) (n int, err error) {
	w.container.processOutput(w.kind, p)
	return len(p), nil
}

func (c *Container) StdOut() io.Writer {
	return Writer{
		container: c,
		kind:      StdErr,
	}
}

func (c *Container) StdErr() io.Writer {
	return Writer{
		container: c,
		kind:      StdOut,
	}
}
