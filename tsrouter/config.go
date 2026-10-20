package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Options mirrors the add-on's /data/options.json, as written by the
// Supervisor from the schema in config.yaml.
type Options struct {
	AuthKey string   `json:"auth_key"`
	Tags    []string `json:"tags"`
	Verbose bool     `json:"verbose"`
	Proxies []Proxy  `json:"proxies"`
}

// Proxy is one tailnet node forwarding to one LAN device.
type Proxy struct {
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
	// Ports is a comma-separated list of "port" or "listen:target" entries.
	// Empty or "*" forwards every TCP port to the same port on the target.
	Ports string `json:"ports"`

	portMap map[uint16]uint16 // nil means all ports
}

// TargetPort reports which target port a tailnet port forwards to, and
// whether it is forwarded at all.
func (p *Proxy) TargetPort(listen uint16) (uint16, bool) {
	if p.portMap == nil {
		return listen, true
	}
	t, ok := p.portMap[listen]
	return t, ok
}

var hostnameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func loadOptions(path string) (*Options, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var o Options
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := o.validate(); err != nil {
		return nil, err
	}
	return &o, nil
}

func (o *Options) validate() error {
	if len(o.Proxies) == 0 {
		return fmt.Errorf("no proxies configured")
	}
	if strings.HasPrefix(o.AuthKey, "tskey-client-") && len(o.Tags) == 0 {
		return fmt.Errorf("an OAuth client secret requires at least one tag")
	}
	seen := map[string]bool{}
	for i := range o.Proxies {
		p := &o.Proxies[i]
		p.Hostname = strings.ToLower(strings.TrimSpace(p.Hostname))
		p.Target = strings.TrimSpace(p.Target)
		if !hostnameRE.MatchString(p.Hostname) {
			return fmt.Errorf("proxy %d: invalid hostname %q", i+1, p.Hostname)
		}
		if seen[p.Hostname] {
			return fmt.Errorf("proxy %d: duplicate hostname %q", i+1, p.Hostname)
		}
		seen[p.Hostname] = true
		if p.Target == "" {
			return fmt.Errorf("%s: target is required", p.Hostname)
		}
		m, err := parsePorts(p.Ports)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Hostname, err)
		}
		p.portMap = m
	}
	return nil
}

func parsePorts(s string) (map[uint16]uint16, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil, nil
	}
	m := map[uint16]uint16{}
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		listen, target, hasTarget := strings.Cut(f, ":")
		l, err := parsePort(listen)
		if err != nil {
			return nil, err
		}
		t := l
		if hasTarget {
			if t, err = parsePort(target); err != nil {
				return nil, err
			}
		}
		if _, dup := m[l]; dup {
			return nil, fmt.Errorf("port %d listed twice", l)
		}
		m[l] = t
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("no ports in %q", s)
	}
	return m, nil
}

func parsePort(s string) (uint16, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return uint16(n), nil
}
