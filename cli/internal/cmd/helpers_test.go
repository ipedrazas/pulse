package cmd

import (
	"testing"
)

// --- truncate ---

func TestTruncate_ShorterThanN(t *testing.T) {
	got := truncate("abc", 10)
	if got != "abc" {
		t.Errorf("truncate(abc, 10) = %q, want abc", got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := truncate("abcdef", 6)
	if got != "abcdef" {
		t.Errorf("truncate(abcdef, 6) = %q, want abcdef", got)
	}
}

func TestTruncate_LongerThanN(t *testing.T) {
	got := truncate("abcdefghijklmnop", 12)
	if got != "abcdefghijkl" {
		t.Errorf("truncate = %q, want abcdefghijkl", got)
	}
}

// --- formatUptime ---

func TestFormatUptime_Zero(t *testing.T) {
	if got := formatUptime(0); got != "0s" {
		t.Errorf("formatUptime(0) = %q", got)
	}
}

func TestFormatUptime_Seconds(t *testing.T) {
	if got := formatUptime(45); got != "45s" {
		t.Errorf("formatUptime(45) = %q", got)
	}
}

func TestFormatUptime_Minutes(t *testing.T) {
	if got := formatUptime(120); got != "2m" {
		t.Errorf("formatUptime(120) = %q", got)
	}
}

func TestFormatUptime_Hours(t *testing.T) {
	if got := formatUptime(7200); got != "2h" {
		t.Errorf("formatUptime(7200) = %q", got)
	}
}

func TestFormatUptime_Days(t *testing.T) {
	if got := formatUptime(172800); got != "2d" {
		t.Errorf("formatUptime(172800) = %q", got)
	}
}

// --- parsePort ---

func TestParsePort_Valid(t *testing.T) {
	pm, err := parsePort("8080:80")
	if err != nil {
		t.Fatalf("parsePort returned error: %v", err)
	}
	if pm.HostPort != 8080 {
		t.Errorf("HostPort = %d, want 8080", pm.HostPort)
	}
	if pm.ContainerPort != 80 {
		t.Errorf("ContainerPort = %d, want 80", pm.ContainerPort)
	}
	if pm.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want tcp", pm.Protocol)
	}
}

func TestParsePort_NoColon(t *testing.T) {
	_, err := parsePort("8080")
	if err == nil {
		t.Error("expected error for port without colon")
	}
}

func TestParsePort_NonNumeric(t *testing.T) {
	_, err := parsePort("abc:def")
	if err == nil {
		t.Error("expected error for non-numeric port")
	}
}

func TestParsePort_ValidLargePorts(t *testing.T) {
	pm, err := parsePort("443:443")
	if err != nil {
		t.Fatalf("parsePort returned error: %v", err)
	}
	if pm.HostPort != 443 || pm.ContainerPort != 443 {
		t.Errorf("ports = %d:%d, want 443:443", pm.HostPort, pm.ContainerPort)
	}
}

func TestParsePort_EmptyString(t *testing.T) {
	_, err := parsePort("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}
