package webtty

// Option is an option for WebTTY.
type Option func(*WebTTY)

// WithPermitWrite sets a WebTTY to accept input from slaves.
func WithPermitWrite() Option {
	return func(wt *WebTTY) {
		wt.permitWrite = true
	}
}

// WithFixedSize sets a fixed size to TTY master.
func WithFixedSize(width uint16, height uint16) Option {
	return func(wt *WebTTY) {
		wt.width = width
		wt.height = height
	}
}
