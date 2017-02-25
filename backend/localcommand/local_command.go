package localcommand

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
	"text/template"
	"time"
	"unsafe"

	"github.com/kr/pty"
	"github.com/pkg/errors"
)

const (
	DefaultCloseSignal       = syscall.SIGINT
	DefaultCloseTimeout      = 10 * time.Second
	DefaultWindowTitleFormat = "GoTTY - {{ .Command }} {{ .Hostname }}"
)

var (
	defaultWindowTitleTemplate = template.New(DefaultWindowTitleFormat)
)

type LocalCommand struct {
	closeSignal   syscall.Signal
	closeTimeout  time.Duration
	titleTemplate *template.Template

	cmd       *exec.Cmd
	pty       *os.File
	ptyClosed chan struct{}
}

func New(argv []string, options ...Option) (*LocalCommand, error) {
	cmd := exec.Command(argv[0], argv[1:]...)

	pty, err := pty.Start(cmd)
	if err != nil {
		// todo close cmd?
		return nil, errors.Wrapf(err, "failed to start command `%s`", argv[0])
	}
	ptyClosed := make(chan struct{})

	lcmd := &LocalCommand{
		closeSignal:   DefaultCloseSignal,
		closeTimeout:  DefaultCloseTimeout,
		titleTemplate: defaultWindowTitleTemplate,

		cmd:       cmd,
		pty:       pty,
		ptyClosed: ptyClosed,
	}

	for _, option := range options {
		option(lcmd)
	}

	// When the process is closed by the user,
	// close pty so that Read() on the pty breaks with an EOF.
	go func() {
		defer func() {
			lcmd.pty.Close()
			close(lcmd.ptyClosed)
		}()

		lcmd.cmd.Wait()
	}()

	return lcmd, nil
}

func (lcmd *LocalCommand) Read(p []byte) (n int, err error) {
	return lcmd.pty.Read(p)
}

func (lcmd *LocalCommand) Write(p []byte) (n int, err error) {
	return lcmd.pty.Write(p)
}

func (lcmd *LocalCommand) Close() error {
	if lcmd.cmd != nil && lcmd.cmd.Process != nil {
		lcmd.cmd.Process.Signal(lcmd.closeSignal)
	}
	for {
		select {
		case <-lcmd.ptyClosed:
			return nil
		case <-time.After(lcmd.closeTimeout):
			lcmd.cmd.Process.Signal(syscall.SIGKILL)
		}
	}
}

func (lcmd *LocalCommand) ResizeTerminal(width, height uint16) error {
	window := struct {
		row uint16
		col uint16
		x   uint16
		y   uint16
	}{
		height,
		width,
		0,
		0,
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		lcmd.pty.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(&window)),
	)
	if errno != 0 {
		return errno
	} else {
		return nil
	}
}

func (lcmd *LocalCommand) WindowTitle() (title string, err error) {
	hostname, _ := os.Hostname()

	titleVars := struct {
		Command  string
		Pid      int
		Hostname string
	}{
		Command:  lcmd.cmd.Path,
		Pid:      lcmd.cmd.Process.Pid,
		Hostname: hostname,
	}

	titleBuffer := new(bytes.Buffer)
	if err := lcmd.titleTemplate.Execute(titleBuffer, titleVars); err != nil {
		return "", err
	}
	return titleBuffer.String(), nil
}
