package backend

import (
	"github.com/yudai/gotty/webtty"
)

// Factory creates instances of webtty.Slave.
type Factory interface {
	Name() string
	New(params map[string][]string) (webtty.Slave, error)
}
