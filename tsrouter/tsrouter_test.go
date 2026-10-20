package main

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		in      string
		want    map[uint16]uint16
		wantErr bool
	}{
		{in: "", want: nil},
		{in: "*", want: nil},
		{in: "9100", want: map[uint16]uint16{9100: 9100}},
		{in: " 9100, 80:8080 ,", want: map[uint16]uint16{9100: 9100, 80: 8080}},
		{in: "0", wantErr: true},
		{in: "70000", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "80,80:81", wantErr: true},
		{in: ",", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parsePorts(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePorts(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if len(got) != len(tt.want) || (got == nil) != (tt.want == nil) {
			t.Errorf("parsePorts(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("parsePorts(%q)[%d] = %d, want %d", tt.in, k, got[k], v)
			}
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Options{Proxies: []Proxy{
		{Hostname: " Receipt ", Target: "192.168.1.30", Ports: "9100"},
		{Hostname: "hp", Target: "192.168.1.136"},
	}}
	if err := ok.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if ok.Proxies[0].Hostname != "receipt" {
		t.Errorf("hostname not normalised: %q", ok.Proxies[0].Hostname)
	}
	if _, fwd := ok.Proxies[0].TargetPort(80); fwd {
		t.Error("receipt should not forward port 80")
	}
	if p, fwd := ok.Proxies[1].TargetPort(631); !fwd || p != 631 {
		t.Errorf("hp TargetPort(631) = %d, %v", p, fwd)
	}

	bad := map[string]Options{
		"no proxies":     {},
		"bad hostname":   {Proxies: []Proxy{{Hostname: "my_printer", Target: "x"}}},
		"duplicate":      {Proxies: []Proxy{{Hostname: "a", Target: "x"}, {Hostname: "A", Target: "y"}}},
		"no target":      {Proxies: []Proxy{{Hostname: "a"}}},
		"oauth no tags":  {AuthKey: "tskey-client-abc", Proxies: []Proxy{{Hostname: "a", Target: "x"}}},
		"bad port value": {Proxies: []Proxy{{Hostname: "a", Target: "x", Ports: "nine"}}},
	}
	for name, o := range bad {
		if err := o.validate(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// TestForwardHalfClose models a raw (port 9100) print job: the client sends
// the job and closes its write side, the printer reads to EOF and only then
// replies. The reply must still reach the client.
func TestForwardHalfClose(t *testing.T) {
	printer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	gotJob := make(chan string, 1)
	go func() {
		c, err := printer.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		b, _ := io.ReadAll(c)
		gotJob <- string(b)
		c.Write([]byte("printed"))
	}()

	front, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer front.Close()
	forwarded := make(chan struct{})
	go func() {
		defer close(forwarded)
		c, err := front.Accept()
		if err != nil {
			return
		}
		forward(context.Background(), t.Logf, c, printer.Addr().String(), "test")
	}()

	c, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	job := "^XA^FDhello^FS^XZ"
	c.Write([]byte(job))
	c.(*net.TCPConn).CloseWrite()

	reply, err := io.ReadAll(c)
	if err != nil {
		t.Fatalf("reading reply: %v", err)
	}
	if got := <-gotJob; got != job {
		t.Errorf("printer got %q, want %q", got, job)
	}
	if string(reply) != "printed" {
		t.Errorf("client got %q, want %q", reply, "printed")
	}
	<-forwarded
}

func TestForwardDialFailure(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := l.Addr().String()
	l.Close()

	a, b := net.Pipe()
	defer a.Close()
	var logs []string
	logf := func(f string, args ...any) { logs = append(logs, f) }
	done := make(chan struct{})
	go func() { forward(context.Background(), logf, b, dead, "test"); close(done) }()
	a.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := a.Read(make([]byte, 1)); err != io.EOF {
		t.Errorf("client read err = %v, want EOF", err)
	}
	<-done
	if len(logs) != 1 || !strings.Contains(logs[0], "%v") {
		t.Errorf("expected one dial-error log, got %v", logs)
	}
}

func TestOAuthSecret(t *testing.T) {
	tests := map[string]string{
		"tskey-client-abc":                                    "tskey-client-abc?ephemeral=false&preauthorized=true",
		"tskey-client-abc?ephemeral=true":                     "tskey-client-abc?ephemeral=true&preauthorized=true",
		"tskey-client-abc?preauthorized=false&ephemeral=true": "tskey-client-abc?preauthorized=false&ephemeral=true",
	}
	for in, want := range tests {
		if got := oauthSecret(in); got != want {
			t.Errorf("oauthSecret(%q) = %q, want %q", in, got, want)
		}
	}
}
