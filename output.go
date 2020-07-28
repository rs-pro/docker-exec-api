package dea

import (
	"bytes"
	"io"
	"log"
	"strings"
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
	c.cond.L.Lock()
	log.Println(">", strings.TrimSpace(string(p)))
	c.buffers[kind].Write(p)
	lines := bytes.Split(c.buffers[kind].Bytes(), []byte("\n"))
	if len(lines) > 1 {
		c.buffers[kind] = bytes.NewBuffer([]byte{})
	} else {
		c.cond.L.Unlock()
		return
	}
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
	c.cond.L.Unlock()
}

func (c *Container) appendLog(lines []*OutputLine) {
	cmd := c.commands[len(c.commands)-1]
	for _, line := range lines {
		cmd.Output = append(cmd.Output, line)
	}
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
