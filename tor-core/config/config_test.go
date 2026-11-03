package config

import (
	"errors"
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
		check   func(t *testing.T, cfg Config)
	}{
		{
			name: "defaults are valid",
			args: nil,
			check: func(t *testing.T, cfg Config) {
				if cfg.ListenAddr != "127.0.0.1:10829" {
					t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
				}
				if !cfg.Snowflake || !cfg.Lyrebird || !cfg.Conjure || !cfg.AutoFetch {
					t.Fatalf("expected all toggles enabled by default: %+v", cfg)
				}
			},
		},
		{
			name: "custom flags and repeatable bridges",
			args: []string{
				"--bind", "127.0.0.1:19050",
				"--snowflake=false",
				"--lyrebird=true",
				"--conjure=true",
				"--auto-fetch=false",
				"--bridge", "obfs4 192.0.2.1:443 AAAA cert=xyz iat-mode=0",
				"--bridge", "conjure 192.0.2.2:80 BBBB url=https://example.org",
				"--log-level", "debug",
			},
			check: func(t *testing.T, cfg Config) {
				if cfg.ListenAddr != "127.0.0.1:19050" {
					t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
				}
				if cfg.Snowflake {
					t.Fatal("expected snowflake to be false")
				}
				if !cfg.Lyrebird || !cfg.Conjure {
					t.Fatal("expected lyrebird and conjure to be true")
				}
				if cfg.AutoFetch {
					t.Fatal("expected auto-fetch to be false")
				}
				if len(cfg.Bridges) != 2 {
					t.Fatalf("expected 2 bridges, got %d", len(cfg.Bridges))
				}
				if cfg.LogLevel != "debug" {
					t.Fatalf("unexpected log level: %s", cfg.LogLevel)
				}
			},
		},
		{
			name:    "invalid listen address fails",
			args:    []string{"--bind", "not-an-address"},
			wantErr: ErrInvalidListenAddr,
		},
		{
			name:    "port zero fails",
			args:    []string{"--bind", "127.0.0.1:0"},
			wantErr: ErrInvalidListenAddr,
		},
		{
			name:    "invalid log level fails",
			args:    []string{"--log-level", "verbose"},
			wantErr: ErrInvalidLogLevel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseFlags(tc.args)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}
