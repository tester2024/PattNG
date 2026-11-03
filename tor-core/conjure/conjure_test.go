package conjure

import (
	"testing"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
)

func TestConjureTransportParseArgs(t *testing.T) {
	tr := NewTransport()
	if tr.Name() != MethodName {
		t.Fatalf("unexpected transport name: %s", tr.Name())
	}

	cf, err := tr.ClientFactory(t.TempDir())
	if err != nil {
		t.Fatalf("ClientFactory failed: %v", err)
	}

	args := make(pt.Args)
	args.Add("url", "https://1657793272.rsc.cdn77.org")
	args.Add("fronts", "cdn.zk.mk,www.cdn77.com")
	args.Add("transport", "prefix")
	args.Add("registrar", "bdapi")
	args.Add("utls-nosni", "true")

	parsed, err := cf.ParseArgs(&args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil parsed config")
	}
}
