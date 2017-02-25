package webtty

import (
	"io"
)

// Slave represents a PTY slave, typically it's a local command.
type Slave interface {
	io.ReadWriteCloser

	WindowTitle() (string, error)
	ResizeTerminal(columns uint16, rows uint16) error
}
