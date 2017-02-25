package localcommand

import (
	"syscall"
	"text/template"

	"github.com/pkg/errors"

	"github.com/yudai/gotty/webtty"
)

type Options struct {
	CloseSignal int    `hcl:"close_signal" flagName:"close-signal" flagSName:"" flagDescribe:"Signal sent to the command process when gotty close it (default: SIGHUP)" default:"1"`
	TitleFormat string `hcl:"title_format" flagName:"title-format" flagSName:"" flagDescribe:"Title format of browser window" default:"GoTTY - {{ .Command }} ({{ .Hostname }})"`
}

type Factory struct {
	command       []string
	options       *Options
	titleTemplate *template.Template
}

func NewFactory(command []string, options *Options) (*Factory, error) {
	titleTemplate, err := template.New("title").Parse(options.TitleFormat)
	if err != nil {
		return nil, errors.Wrapf(err, "title format string syntax error: %v", options.TitleFormat)
	}
	return &Factory{
		command:       command,
		options:       options,
		titleTemplate: titleTemplate,
	}, nil
}

func (factory *Factory) Name() string {
	return "local command"
}

func (factory *Factory) New(params map[string][]string) (webtty.Slave, error) {
	argv := factory.command
	// todo conststant?
	if params["args"] != nil && len(params["args"]) > 0 {
		argv = append(argv, params["args"]...)
	}
	return New(
		argv,
		WithCloseSignal(syscall.Signal(factory.options.CloseSignal)),
		WithTitleTemplate(factory.titleTemplate),
	)
}
