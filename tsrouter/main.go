// Command tsrouter runs one tsnet node per configured proxy, forwarding TCP
// connections from the tailnet to a device on the local network.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/tsnet"
)

func main() {
	optionsPath := flag.String("options", "/data/options.json", "path to add-on options")
	stateDir := flag.String("state", "/data/nodes", "directory for per-node tailscale state")
	flag.Parse()
	log.SetFlags(0)

	opts, err := loadOptions(*optionsPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var servers []*tsnet.Server
	for i := range opts.Proxies {
		p := &opts.Proxies[i]
		srv, err := startNode(ctx, opts, p, filepath.Join(*stateDir, p.Hostname))
		if err != nil {
			log.Fatalf("[%s] %v", p.Hostname, err)
		}
		servers = append(servers, srv)
	}

	<-ctx.Done()
	log.Printf("shutting down")
	for _, s := range servers {
		s.Close()
	}
}

func startNode(ctx context.Context, opts *Options, p *Proxy, dir string) (*tsnet.Server, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	logf := func(format string, args ...any) {
		log.Printf("["+p.Hostname+"] "+format, args...)
	}

	srv := &tsnet.Server{
		Hostname:      p.Hostname,
		Dir:           dir,
		AdvertiseTags: opts.Tags,
		UserLogf:      logf,
		Logf:          func(string, ...any) {},
	}
	if opts.Verbose {
		srv.Logf = logf
	}
	if strings.HasPrefix(opts.AuthKey, "tskey-client-") {
		srv.ClientSecret = opts.AuthKey
	} else {
		srv.AuthKey = opts.AuthKey
	}

	if err := srv.Start(); err != nil {
		return nil, err
	}
	lc, err := srv.LocalClient()
	if err != nil {
		return nil, err
	}

	srv.RegisterFallbackTCPHandler(func(src, dst netip.AddrPort) (func(net.Conn), bool) {
		port, ok := p.TargetPort(dst.Port())
		if !ok {
			return nil, false
		}
		target := targetAddr(p.Target, port)
		return func(c net.Conn) {
			who := describePeer(ctx, lc, src)
			logf("%s -> %s: connected via :%d", who, target, dst.Port())
			forward(ctx, logf, c, target, who)
		}, true
	})

	// Up blocks until the node is running, which on first start may mean
	// waiting for someone to visit the login URL, so don't hold up the others.
	go func() {
		if _, err := srv.Up(ctx); err != nil {
			if ctx.Err() == nil {
				logf("failed to come up: %v", err)
			}
			return
		}
		if ip4, _ := srv.TailscaleIPs(); ip4.IsValid() {
			logf("up as %s (%s), forwarding %s to %s", p.Hostname, ip4, describePorts(p), p.Target)
		}
	}()
	return srv, nil
}

// describePeer names the tailnet peer behind src for logging, falling back
// to its address if WhoIs fails.
func describePeer(ctx context.Context, lc *local.Client, src netip.AddrPort) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	w, err := lc.WhoIs(ctx, src.String())
	if err != nil || w.Node == nil {
		return src.Addr().String()
	}
	name := strings.TrimSuffix(w.Node.ComputedName, ".")
	if w.UserProfile != nil && w.UserProfile.LoginName != "" && !w.Node.IsTagged() {
		name += " (" + w.UserProfile.LoginName + ")"
	}
	return name
}

func describePorts(p *Proxy) string {
	if p.portMap == nil {
		return "all TCP ports"
	}
	return "ports " + p.Ports
}
