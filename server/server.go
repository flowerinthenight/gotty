package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/yudai/gotty/backend"
	"github.com/yudai/gotty/pkg/homedir"
	"github.com/yudai/gotty/pkg/randomstring"
	"github.com/yudai/gotty/webtty"
)

// Server provides a webtty HTTP endpoint.
type Server struct {
	factory backend.Factory
	options *Options

	srv *http.Server

	upgrader *websocket.Upgrader

	timer       *time.Timer
	wsWG        sync.WaitGroup
	url         *url.URL // use URL()
	connections *int64   // Use atomic operations
	once        *int64   // use atomic operations
}

// New creates a new instance of Server.
// Server will use the New() of the factory provided to handle each request.
func New(factory backend.Factory, options *Options) (*Server, error) {
	connections := int64(0)
	once := int64(0)
	return &Server{
		factory: factory,
		options: options,

		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			Subprotocols:    webtty.Protocols,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		connections: &connections,
		once:        &once,
	}, nil
}

// Run starts the main process of the Server.
// The cancelation of ctx will shutdown the server immediately with aborting
// existing connections. Use WithGracefullContext() to support gracefull shutdown.
func (server *Server) Run(ctx context.Context, options ...RunOption) error {
	cctx, cancel := context.WithCancel(ctx)
	opts := &RunOptions{gracefullCtx: context.Background()}
	for _, opt := range options {
		opt(opts)
	}

	handlers := server.setupHandlers(cctx, cancel)
	srv, err := server.setupHTTPServer(handlers)
	if err != nil {
		return errors.Wrapf(err, "failed to setup an HTTP server")
	}

	if server.options.PermitWrite {
		log.Printf("Permitting clients to write input to the PTY.")
	}

	if server.options.Once {
		log.Printf("Once option is provided, accepting only one client")
	}

	server.srv = srv

	if server.options.Timeout > 0 {
		server.timer = time.NewTimer(time.Duration(server.options.Timeout) * time.Second)
		go func() {
			select {
			case <-server.timer.C:
				cancel()
			case <-cctx.Done():
			}
		}()
	}

	listenErr := make(chan error, 1)
	go func() {
		if server.options.EnableTLS {
			crtFile := homedir.Expand(server.options.TLSCrtFile)
			keyFile := homedir.Expand(server.options.TLSKeyFile)
			log.Printf("TLS crt file: " + crtFile)
			log.Printf("TLS key file: " + keyFile)

			err = srv.ListenAndServeTLS(crtFile, keyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil {
			listenErr <- err
		}
	}()

	go func() {
		select {
		case <-opts.gracefullCtx.Done():
			srv.Shutdown(context.Background())
		case <-cctx.Done():
		}
	}()

	select {
	case err = <-listenErr:
		if err == http.ErrServerClosed { // by gracefull ctx
			err = nil
		} else {
			cancel()
		}
	case <-cctx.Done():
		srv.Close()
		err = cctx.Err()
	}

	conn := atomic.LoadInt64(server.connections)
	if conn > 0 {
		log.Printf("Waiting for %d connections to be closed", conn)
	}
	server.wsWG.Wait()

	return err
}

func (server *Server) setupHandlers(ctx context.Context, cancel context.CancelFunc) http.Handler {
	staticFileHandler := http.FileServer(
		&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, Prefix: "static"},
	)

	url := server.URL()
	var siteMux = http.NewServeMux()
	if server.options.IndexFile != "" {
		log.Printf("Using index file at " + server.options.IndexFile)
		siteMux.HandleFunc(url.Path, server.handleCustomIndex)
	} else {
		siteMux.Handle(url.Path, http.StripPrefix(url.Path, staticFileHandler))
	}
	siteMux.Handle(url.Path+"js/", http.StripPrefix(url.Path, staticFileHandler))
	siteMux.Handle(url.Path+"favicon.png", http.StripPrefix(url.Path, staticFileHandler))
	siteMux.HandleFunc(url.Path+"auth_token.js", server.handleAuthToken)

	siteHandler := http.Handler(siteMux)

	if server.options.EnableBasicAuth {
		log.Printf("Using Basic Authentication")
		siteHandler = server.wrapBasicAuth(siteHandler, server.options.Credential)
	}

	siteHandler = server.wrapHeaders(siteHandler)

	wsMux := http.NewServeMux()
	wsMux.Handle("/", siteHandler)
	wsMux.HandleFunc(url.Path+"ws", server.generateHandleWS(ctx, cancel))
	siteHandler = http.Handler(wsMux)

	return server.wrapLogger(siteHandler)
}

func (server *Server) setupHTTPServer(handler http.Handler) (*http.Server, error) {
	url := server.URL()
	log.Printf("URL: %s", url.String())

	srv := &http.Server{
		Addr:    url.Host,
		Handler: handler,
	}

	if server.options.EnableTLSClientAuth {
		tlsConfig, err := server.tlsConfig()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to setup TLS configuration")
		}
		srv.TLSConfig = tlsConfig
	}

	return srv, nil
}

func (server *Server) URL() *url.URL {
	if server.url == nil {
		host := net.JoinHostPort(server.options.Address, server.options.Port)
		path := ""
		fmt.Printf("%s %d\n", server.options.EnableRandomUrl, server.options.RandomUrlLength)
		if server.options.EnableRandomUrl {
			path += "/" + randomstring.Generate(server.options.RandomUrlLength)
		}
		scheme := "http"
		if server.options.EnableTLS {
			scheme = "https"
		}
		server.url = &url.URL{Scheme: scheme, Host: host, Path: path + "/"}
	}
	return server.url
}

func (server *Server) tlsConfig() (*tls.Config, error) {
	caFile := homedir.Expand(server.options.TLSCACrtFile)
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, errors.New("Could not open CA crt file " + caFile)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("Could not parse CA crt file data in " + caFile)
	}
	tlsConfig := &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}
	return tlsConfig, nil
}
