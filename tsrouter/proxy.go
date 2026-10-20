package main

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const dialTimeout = 10 * time.Second

// forward dials target and copies bytes both ways until both directions
// have finished. Half-closes are propagated, which matters for raw printing:
// a client finishing a job closes its write side and then waits for the
// printer to finish (and possibly reply) before the connection is torn down.
func forward(ctx context.Context, logf func(string, ...any), client net.Conn, target string, who string) {
	defer client.Close()

	d := net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}
	upstream, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		logf("%s -> %s: %v", who, target, err)
		return
	}
	defer upstream.Close()

	start := time.Now()
	var wg sync.WaitGroup
	var sent, received int64
	wg.Add(2)
	go func() {
		defer wg.Done()
		sent = pipe(upstream, client)
	}()
	go func() {
		defer wg.Done()
		received = pipe(client, upstream)
	}()
	wg.Wait()
	logf("%s -> %s: closed after %s, sent %d bytes, received %d bytes",
		who, target, time.Since(start).Round(time.Millisecond), sent, received)
}

// pipe copies src to dst, then half-closes dst's write side (or fully
// closes it if half-close is unsupported) so the other end sees EOF.
func pipe(dst, src net.Conn) int64 {
	n, err := io.Copy(dst, src)
	if err != nil && !errors.Is(err, net.ErrClosed) {
		// A reset on one side; tear down the other so its copy ends too.
		dst.Close()
		src.Close()
		return n
	}
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	} else {
		dst.Close()
	}
	return n
}

func targetAddr(host string, port uint16) string {
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}
