package main

import (
	"errors"
	"flag"
	"io"
	"testing"
	"time"

	"github.com/yeya/wire-seek/mtu"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    config
		wantErr bool
	}{
		{
			name: "positional host",
			args: []string{"example.com"},
			want: config{target: "example.com", timeout: mtu.DefaultTimeout, attempts: mtu.DefaultAttempts},
		},
		{
			name: "target flag",
			args: []string{"-target", "example.com", "-v"},
			want: config{target: "example.com", verbose: true, timeout: mtu.DefaultTimeout, attempts: mtu.DefaultAttempts},
		},
		{
			name: "flags after host",
			args: []string{"example.com", "-tunnel", "-max-mtu", "1420"},
			want: config{target: "example.com", tunnel: true, maxMTU: 1420, timeout: mtu.DefaultTimeout, attempts: mtu.DefaultAttempts},
		},
		{
			name: "flags on both sides",
			args: []string{"-6", "2001:db8::1", "-q", "-timeout", "500ms", "-attempts", "3"},
			want: config{target: "2001:db8::1", ipv6: true, quiet: true, timeout: 500 * time.Millisecond, attempts: 3},
		},
		{name: "missing host", args: []string{"-v"}, wantErr: true},
		{name: "two hosts", args: []string{"a.example", "b.example"}, wantErr: true},
		{name: "host twice", args: []string{"-target", "a.example", "b.example"}, wantErr: true},
		{name: "unknown flag", args: []string{"-bogus", "a.example"}, wantErr: true},
		{name: "negative mtu", args: []string{"-min-mtu", "-1", "a.example"}, wantErr: true},
		{name: "max mtu too large", args: []string{"-max-mtu", "70000", "a.example"}, wantErr: true},
		{name: "zero timeout", args: []string{"-timeout", "0s", "a.example"}, wantErr: true},
		{name: "zero attempts", args: []string{"-attempts", "0", "a.example"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFlags(tt.args, io.Discard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFlags(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if !tt.wantErr && *got != tt.want {
				t.Errorf("parseFlags(%v) = %+v, want %+v", tt.args, *got, tt.want)
			}
		})
	}
}

func TestParseFlagsHelp(t *testing.T) {
	if _, err := parseFlags([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("parseFlags(-h) error = %v, want flag.ErrHelp", err)
	}
}

func TestResolveTargetLiterals(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		requireIPv6 bool
		want        string
		wantErr     bool
	}{
		{"IPv4 literal", "192.0.2.1", false, "192.0.2.1", false},
		{"IPv6 literal", "2001:db8::1", false, "2001:db8::1", false},
		{"IPv6 literal with -6", "2001:db8::1", true, "2001:db8::1", false},
		{"IPv4 literal with -6 is rejected", "192.0.2.1", true, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTarget(tt.target, tt.requireIPv6)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveTarget(%q, %v) error = %v, wantErr %v", tt.target, tt.requireIPv6, err, tt.wantErr)
			}
			if !tt.wantErr && got.String() != tt.want {
				t.Errorf("resolveTarget(%q, %v) = %v, want %v", tt.target, tt.requireIPv6, got, tt.want)
			}
		})
	}
}
