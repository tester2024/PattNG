package transport

import (
	"cmp"
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"sync"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/common/socks5"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
	sf "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/client/lib"
	"golang.org/x/net/proxy"
)

const managedSocksAddr = "127.0.0.1:0"

var ErrNoManagedTransports = errors.New("no managed pluggable transports were launched")

// IsManagedPTEnv reports whether the current process was started by Tor/Arti as a managed PT binary.
func IsManagedPTEnv() bool {
	return os.Getenv("TOR_PT_MANAGED_TRANSPORT_VER") != ""
}

// RunManagedClient executes the Tor Managed Pluggable Transport v1 client protocol for reg.
func RunManagedClient(ctx context.Context, reg *Registry) error {
	stateDir, err := pt.MakeStateDir()
	if err != nil {
		return err
	}

	ptInfo, err := pt.ClientSetup(reg.Methods())
	if err != nil {
		return err
	}

	if ptInfo.ProxyURL != nil {
		pt.ProxyDone()
	}

	var listeners []net.Listener
	for _, name := range ptInfo.MethodNames {
		t := reg.Get(name)
		if t == nil {
			pt.CmethodError(name, "no such transport is enabled")
			continue
		}

		factory, err := t.ClientFactory(stateDir)
		if err != nil {
			pt.CmethodError(name, err.Error())
			continue
		}

		factory.OnEvent(func(e base.TransportEvent) {
			pt.Log(pt.LogSeverityNotice, e.String())
		})

		ln, err := net.Listen("tcp", managedSocksAddr)
		if err != nil {
			pt.CmethodError(name, err.Error())
			continue
		}

		listeners = append(listeners, ln)
		go acceptLoop(ctx, factory, ln, ptInfo.ProxyURL)
		pt.Cmethod(name, socks5.Version(), ln.Addr())
	}

	pt.CmethodsDone()
	if len(listeners) == 0 {
		return ErrNoManagedTransports
	}

	stopOnStdinClose(ctx, listeners)
	<-ctx.Done()

	for _, ln := range listeners {
		_ = ln.Close()
	}

	return nil
}

func stopOnStdinClose(ctx context.Context, listeners []net.Listener) {
	if os.Getenv("TOR_PT_EXIT_ON_STDIN_CLOSE") != "1" {
		return
	}

	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}()

	context.AfterFunc(ctx, func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	})
}

func acceptLoop(ctx context.Context, factory base.ClientFactory, ln net.Listener, proxyURL *url.URL) {
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go handleClientConn(factory, conn, proxyURL)
	}
}

func handleClientConn(factory base.ClientFactory, conn net.Conn, proxyURI *url.URL) {
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

	dialFn := proxy.Direct.Dial
	if proxyURI != nil {
		dialer, err := proxy.FromURL(proxyURI, proxy.Direct)
		if err != nil {
			_ = req.Reply(socks5.ReplyGeneralFailure)
			return
		}
		dialFn = dialer.Dial

		if factory.Transport().Name() == "snowflake" {
			if sfCfg, ok := args.(sf.ClientConfig); ok {
				sfCfg.CommunicationProxy = proxyURI
				args = sfCfg
			}
		}
	}

	remote, err := factory.Dial("tcp", req.Target, dialFn, args)
	if err != nil {
		_ = req.Reply(socks5.ErrorToReplyCode(err))
		return
	}
	defer remote.Close()

	if err := req.Reply(socks5.ReplySucceeded); err != nil {
		return
	}

	_ = relayStream(conn, remote)
}

func relayStream(a, b net.Conn) error {
	var wg sync.WaitGroup
	var errA, errB error

	wg.Go(func() {
		_, errA = io.Copy(b, a)
		_ = b.Close()
	})
	wg.Go(func() {
		_, errB = io.Copy(a, b)
		_ = a.Close()
	})

	wg.Wait()
	return cmp.Or(errA, errB)
}
