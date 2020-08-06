package dea

import (
	"bytes"
	"io"
	"log"
	"strconv"
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
	Kind    LineKind  `json:"kind"`
	Content []byte    `json:"content"`
	Time    time.Time `json:"time"`
}

func (c *Container) processOutput(kind LineKind, p []byte) {
	c.Cond.L.Lock()
	log.Println(">", strings.TrimSpace(string(p)))
	c.buffers[kind].Write(p)
	lines := bytes.Split(c.buffers[kind].Bytes(), []byte("\n"))
	if len(lines) > 1 {
		c.buffers[kind] = bytes.NewBuffer([]byte{})
	} else {
		c.Cond.L.Unlock()
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
	lastLine := ret[len(ret)-1]
	if bytes.HasPrefix(lastLine.Content, []byte(" "+deaPrefix)) {
		parts := bytes.Split(lastLine.Content, []byte("||"))
		exitCode, err := strconv.Atoi(string(parts[1]))
		if err != nil {
			log.Println("failed to parse exit code", err)
		}
		cmd := c.LastCommand()
		cmd.ExitCode = &exitCode
		log.Println("stdin is free")
		c.StdinCond.L.Lock()
		c.StdinCond.Broadcast()
		c.StdinCond.L.Unlock()
	}
	c.appendLog(ret)
	c.Cond.Broadcast()
	c.Cond.L.Unlock()
}

func (c *Container) LastCommand() *Command {
	return c.commands[len(c.commands)-1]
}

func (c *Container) appendLog(lines []*OutputLine) {
	cmd := c.LastCommand()
	for _, line := range lines {
		cmd.Output = append(cmd.Output, line)
	}
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
