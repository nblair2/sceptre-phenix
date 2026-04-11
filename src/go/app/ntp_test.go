package app_test

import (
	"bytes"
	"strings"
	"testing"

	"phenix/app"
	"phenix/tmpl"
)

// TestNTPAppSourceIPAddressDirect verifies that an explicit Address is returned
// directly without consulting the experiment topology.
func TestNTPAppSourceIPAddressDirect(t *testing.T) {
	s := app.NTPAppSource{Address: "192.168.1.1"} //nolint:exhaustruct // test data
	if got := s.IPAddress(nil); got != "192.168.1.1" {
		t.Fatalf("expected 192.168.1.1, got %s", got)
	}
}

// TestNTPAppSourceIPAddressMissingInterface verifies that an empty string is
// returned when Interface is not set (nothing to look up).
func TestNTPAppSourceIPAddressMissingInterface(t *testing.T) {
	s := app.NTPAppSource{Hostname: "server01"} //nolint:exhaustruct // test data
	if got := s.IPAddress(nil); got != "" {
		t.Fatalf("expected empty string, got %s", got)
	}
}

// TestNTPLinuxTemplateClient verifies that ntp_linux.tmpl, when given a server
// address, produces a client config pointing at that address.
func TestNTPLinuxTemplateClient(t *testing.T) {
	var buf bytes.Buffer

	if err := tmpl.GenerateFromTemplate("ntp_linux.tmpl", "10.0.0.1", &buf); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "server 10.0.0.1 iburst prefer") {
		t.Fatalf("expected upstream server line in client config:\n%s", buf.String())
	}
}

// TestNTPLinuxTemplateServer verifies that ntp_linux.tmpl, when given an empty
// address, produces a server config that falls back to the local clock.
func TestNTPLinuxTemplateServer(t *testing.T) {
	var buf bytes.Buffer

	if err := tmpl.GenerateFromTemplate("ntp_linux.tmpl", "", &buf); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "server 127.127.1.1 iburst prefer") {
		t.Fatalf("expected local clock reference in server config:\n%s", buf.String())
	}
}

// TestChronyLinuxTemplateClient verifies that chrony_linux.tmpl, when given a
// server address, produces a client config pointing at that address.
func TestChronyLinuxTemplateClient(t *testing.T) {
	var buf bytes.Buffer

	if err := tmpl.GenerateFromTemplate("chrony_linux.tmpl", "10.0.0.1", &buf); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "server 10.0.0.1 iburst prefer") {
		t.Fatalf("expected upstream server line in client config:\n%s", buf.String())
	}
}

// TestChronyLinuxTemplateServer verifies that chrony_linux.tmpl, when given an
// empty address, produces a server config that falls back to the local clock.
func TestChronyLinuxTemplateServer(t *testing.T) {
	var buf bytes.Buffer

	if err := tmpl.GenerateFromTemplate("chrony_linux.tmpl", "", &buf); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "local stratum") {
		t.Fatalf("expected local stratum reference in server config:\n%s", buf.String())
	}
}
