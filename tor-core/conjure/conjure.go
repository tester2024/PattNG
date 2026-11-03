package conjure

import (
	"cmp"
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/refraction-networking/conjure/pkg/client/assets"
	upstreamconjure "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/conjure/client/conjure"
	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/common/socks5"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
)

const (
	MethodName      = "conjure"
	defaultReg      = "bdapi"
	defaultMode     = "min"
	defaultSTUN     = "stun.antisip.com:3478"
	listenSocksAddr = "127.0.0.1:0"
)

var (
	ErrServerUnsupported = errors.New("server factory is not supported for conjure transport")
	ErrInvalidArgs       = errors.New("invalid conjure arguments")
	ErrNoListeners       = errors.New("conjure managed client launched no listeners")
)

// Transport implements base.Transport using the official Tor Project Conjure client library.
type Transport struct {
	assetsOnce sync.Once
}

// NewTransport returns a Conjure pluggable transport instance.
func NewTransport() *Transport {
	return &Transport{}
}

// Name returns the pluggable transport method identifier.
func (t *Transport) Name() string {
	return MethodName
}

// ClientFactory initializes the Conjure asset directory and returns a base.ClientFactory.
func (t *Transport) ClientFactory(stateDir string) (base.ClientFactory, error) {
	if strings.TrimSpace(stateDir) != "" {
		assetDir := filepath.Join(stateDir, MethodName)
		if err := os.MkdirAll(assetDir, 0o700); err != nil {
			return nil, err
		}
		t.assetsOnce.Do(func() {
			_, _ = assets.AssetsSetDir(assetDir)
		})
	}

	return &clientFactory{transport: t}, nil
}

// ServerFactory is unsupported on the client.
func (t *Transport) ServerFactory(_ string, _ *pt.Args) (base.ServerFactory, error) {
	return nil, ErrServerUnsupported
}

// RunManagedClient runs the Tor Managed Pluggable Transport v1 client loop for Conjure.
func RunManagedClient(ctx context.Context) error {
	stateDir, err := pt.MakeStateDir()
	if err != nil {
		return err
	}

	ptInfo, err := pt.ClientSetup([]string{MethodName})
	if err != nil {
		return err
	}

	if ptInfo.ProxyURL != nil {
		pt.ProxyDone()
	}

	tr := NewTransport()
	factory, err := tr.ClientFactory(stateDir)
	if err != nil {
		return err
	}

	var listeners []net.Listener
	for _, method := range ptInfo.MethodNames {
		if method != MethodName {
			pt.CmethodError(method, "no such method")
			continue
		}

		ln, err := net.Listen("tcp", listenSocksAddr)
		if err != nil {
			pt.CmethodError(method, err.Error())
			continue
		}

		listeners = append(listeners, ln)
		go acceptLoop(ctx, factory, ln, ptInfo.ProxyURL)
		pt.Cmethod(method, socks5.Version(), ln.Addr())
	}

	pt.CmethodsDone()
	if len(listeners) == 0 {
		return ErrNoListeners
	}

	<-ctx.Done()
	for _, ln := range listeners {
		_ = ln.Close()
	}
	return nil
}

type clientFactory struct {
	transport base.Transport
	onEvent   func(base.TransportEvent)
}

func (cf *clientFactory) Transport() base.Transport {
	return cf.transport
}

func (cf *clientFactory) OnEvent(f func(base.TransportEvent)) {
	cf.onEvent = f
}

func (cf *clientFactory) ParseArgs(args *pt.Args) (any, error) {
	cfg := upstreamconjure.ConjureConfig{
		Registrar: defaultReg,
		Transport: defaultMode,
		STUNAddr:  defaultSTUN,
	}
	if args == nil {
		return cfg, nil
	}

	if v, ok := args.Get("registrar"); ok && strings.TrimSpace(v) != "" {
		cfg.Registrar = strings.TrimSpace(v)
	}
	if v, ok := args.Get("ampcache"); ok {
		cfg.AMPCacheURL = strings.TrimSpace(v)
	}
	if v, ok := args.Get("url"); ok {
		cfg.RegisterURL = strings.TrimSpace(v)
	}
	if v, ok := args.Get("fronts"); ok && strings.TrimSpace(v) != "" {
		cfg.Fronts = splitCSV(v)
	} else if v, ok := args.Get("front"); ok && strings.TrimSpace(v) != "" {
		cfg.Fronts = splitCSV(v)
	}
	if v, ok := args.Get("utls-nosni"); ok {
		lower := strings.ToLower(strings.TrimSpace(v))
		cfg.UTLSRemoveSNI = lower == "true" || lower == "yes" || lower == "1"
	}
	if v, ok := args.Get("utls-imitate"); ok {
		cfg.UTLSClientID = strings.TrimSpace(v)
	}
	if v, ok := args.Get("transport"); ok && strings.TrimSpace(v) != "" {
		cfg.Transport = strings.TrimSpace(v)
	}
	if v, ok := args.Get("stun"); ok && strings.TrimSpace(v) != "" {
		cfg.STUNAddr = strings.TrimSpace(v)
	}

	return cfg, nil
}

func (cf *clientFactory) Dial(_, address string, _ base.DialFunc, rawArgs any) (net.Conn, error) {
	cfg, ok := rawArgs.(upstreamconjure.ConjureConfig)
	if !ok {
		return nil, ErrInvalidArgs
	}

	cfg.BridgeAddress = cmp.Or(strings.TrimSpace(address), cfg.BridgeAddress)
	return upstreamconjure.Register(&cfg)
}

func acceptLoop(ctx context.Context, factory base.ClientFactory, ln net.Listener, _ *url.URL) {
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go handleConn(factory, conn)
	}
}

func handleConn(factory base.ClientFactory, conn net.Conn) {
	defer conn.Close()

	req, err := socks5.Handshake(conn)
	if err != nil {
		return
	}

	args, err := factory.ParseArgs(&req.Args)
	if err != nil {
		_ = req.Reply(socks5.ReplyGeneralFailure)
		return
	}

	remote, err := factory.Dial("tcp", req.Target, nil, args)
	if err != nil {
		_ = req.Reply(socks5.ErrorToReplyCode(err))
		return
	}
	defer remote.Close()

	if err := req.Reply(socks5.ReplySucceeded); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = io.Copy(remote, conn)
		_ = remote.Close()
	})
	wg.Go(func() {
		_, _ = io.Copy(conn, remote)
		_ = conn.Close()
	})
	wg.Wait()
}

func splitCSV(raw string) []string {
	var out []string
	for item := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
