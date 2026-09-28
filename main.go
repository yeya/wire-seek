package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/yeya/wire-seek/mtu"
	"github.com/yeya/wire-seek/output"
)

const usageText = `Usage: wire-seek [options] <host>
       wire-seek [options] -target <host>

Discovers the path MTU to <host> and recommends a WireGuard interface MTU.

Options:
`

type config struct {
	target   string
	ipv6     bool
	verbose  bool
	quiet    bool
	tunnel   bool
	minMTU   int
	maxMTU   int
	timeout  time.Duration
	attempts int
}

func main() {
	cfg, err := parseFlags(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if err != nil {
		os.Exit(2)
	}
	os.Exit(run(cfg))
}

func newFlagSet(cfg *config, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("wire-seek", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), usageText)
		fs.PrintDefaults()
	}

	fs.StringVar(&cfg.target, "target", "", "Target host or IP address")
	fs.BoolVar(&cfg.ipv6, "6", false, "Use IPv6 instead of IPv4")
	fs.BoolVar(&cfg.verbose, "v", false, "Verbose output (debug diagnostics)")
	fs.BoolVar(&cfg.quiet, "q", false, "Quiet output (only print MTU value, for scripting)")
	fs.BoolVar(&cfg.tunnel, "tunnel", false, "Testing through an existing tunnel (skip WireGuard overhead calculation)")
	fs.IntVar(&cfg.minMTU, "min-mtu", 0, "Minimum MTU to test (default: 576 for IPv4, 1280 for IPv6)")
	fs.IntVar(&cfg.maxMTU, "max-mtu", 0, "Maximum MTU to test (default: 1500)")
	fs.DurationVar(&cfg.timeout, "timeout", mtu.DefaultTimeout, "Time to wait for each echo reply")
	fs.IntVar(&cfg.attempts, "attempts", mtu.DefaultAttempts, "Probes per MTU size before treating it as too large (tolerates packet loss)")
	return fs
}

// parseFlags parses command-line arguments. Flags may appear before or
// after the positional host argument (e.g. "wire-seek example.com -v").
// Errors and usage are written to errOut.
func parseFlags(args []string, errOut io.Writer) (*config, error) {
	cfg := &config{}
	fs := newFlagSet(cfg, errOut)

	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err // already reported by the flag package
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}

	if err := cfg.validate(positional); err != nil {
		_, _ = fmt.Fprintf(errOut, "Error: %v\n\n", err)
		fs.Usage()
		return nil, err
	}
	return cfg, nil
}

// validate applies the positional host argument and checks
// the configuration
func (cfg *config) validate(positional []string) error {
	switch {
	case len(positional) > 1:
		return fmt.Errorf("unexpected arguments: %v", positional[1:])
	case len(positional) == 1 && cfg.target != "":
		return errors.New("specify the host either with -target or as an argument, not both")
	case len(positional) == 1:
		cfg.target = positional[0]
	case cfg.target == "":
		return errors.New("target host is required")
	}

	if cfg.minMTU < 0 || cfg.maxMTU < 0 {
		return errors.New("MTU values must be positive")
	}
	if cfg.maxMTU > mtu.MaxPacketSize {
		return fmt.Errorf("max-mtu cannot exceed %d", mtu.MaxPacketSize)
	}
	if cfg.timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if cfg.attempts < 1 {
		return errors.New("attempts must be at least 1")
	}
	return nil
}

func run(cfg *config) int {
	// Determine output level (quiet takes precedence if both are specified)
	level := output.LevelNormal
	if cfg.quiet {
		level = output.LevelQuiet
	} else if cfg.verbose {
		level = output.LevelVerbose
	}
	log := output.New(level)

	ip, err := resolveTarget(cfg.target, cfg.ipv6)
	if err != nil {
		log.Error("Error resolving target: %v\n", err)
		return 1
	}

	isIPv6 := ip.To4() == nil
	protocol, protocolMin := "IPv4", mtu.MinMTU_IPv4
	if isIPv6 {
		protocol, protocolMin = "IPv6", mtu.MinMTU_IPv6
	}

	log.Info("Wire-Seek: WireGuard MTU Optimizer\n")
	log.Info("Target: %s (%s)\n", cfg.target, ip)
	log.Info("Protocol: %s\n", protocol)
	log.Info("\n")

	minMTU, maxMTU := cfg.minMTU, cfg.maxMTU
	if minMTU == 0 {
		minMTU = protocolMin
	}
	if maxMTU == 0 {
		maxMTU = mtu.MaxMTU
	}

	if minMTU > maxMTU {
		log.Error("Error: min-mtu (%d) cannot be greater than max-mtu (%d)\n", minMTU, maxMTU)
		return 2
	}
	if headers := mtu.ProbeOverhead(isIPv6); minMTU < headers {
		log.Error("Error: min-mtu (%d) must be at least %d (%s + ICMP headers)\n", minMTU, headers, protocol)
		return 2
	}
	if minMTU < protocolMin {
		log.Info("Warning: min-mtu %d is below the protocol minimum (%d for %s)\n", minMTU, protocolMin, protocol)
	}

	discoverer := mtu.NewDiscoverer(ip, log, mtu.Options{
		MinMTU:   minMTU,
		MaxMTU:   maxMTU,
		Timeout:  cfg.timeout,
		Attempts: cfg.attempts,
	})
	pathMTU, err := discoverer.FindPathMTU()
	if err != nil {
		log.Error("Error discovering MTU: %v\n", err)
		return 1
	}

	resultMTU := mtu.CalculateMTU(pathMTU, isIPv6, cfg.tunnel)

	log.Info("\n")
	log.Info("Results:\n")
	log.Info("  Path MTU: %d bytes\n", pathMTU)

	if pathMTU == maxMTU {
		log.Info("  (Path MTU reached the top of the search range; use -max-mtu to test larger sizes)\n")
	}

	if cfg.tunnel {
		log.Info("\n")
		log.Info("Tunnel mode: the path MTU is the largest packet that fits through the tunnel.\n")
		log.Info("It should match the MTU configured on your WireGuard interface.\n")
	} else {
		log.Info("  WireGuard MTU: %d bytes\n", resultMTU)
		log.Info("\n")
		log.Info("Add to your WireGuard config:\n")
		log.Info("  MTU = %d\n", resultMTU)
		if resultMTU < mtu.MinMTU_IPv6 {
			log.Info("\nWarning: an MTU below %d cannot carry IPv6 traffic inside the tunnel.\n", mtu.MinMTU_IPv6)
		}
	}

	// In quiet mode, output only the MTU value
	if cfg.quiet {
		log.Result("%d\n", resultMTU)
	}
	return 0
}

// resolveTarget resolves a hostname or IP string to a net.IP. When
// requireIPv6 is set, only IPv6 addresses are accepted; otherwise IPv4 is
// preferred with IPv6 as a fallback.
func resolveTarget(target string, requireIPv6 bool) (net.IP, error) {
	var ips []net.IP
	if ip := net.ParseIP(target); ip != nil {
		ips = []net.IP{ip}
	} else {
		var err error
		ips, err = net.LookupIP(target)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", target, err)
		}
	}

	var fallback net.IP
	for _, ip := range ips {
		isV6 := ip.To4() == nil
		if isV6 == requireIPv6 {
			return ip, nil
		}
		if fallback == nil {
			fallback = ip
		}
	}

	if requireIPv6 {
		return nil, fmt.Errorf("no IPv6 address found for %s", target)
	}
	if fallback == nil {
		return nil, fmt.Errorf("no IP addresses found for %s", target)
	}
	return fallback, nil
}
