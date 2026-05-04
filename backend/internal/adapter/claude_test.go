package adapter

import "testing"

func TestClaudeAdapter_Protocol(t *testing.T) {
	a := &ClaudeAdapter{}
	if got := a.Protocol(); got != "claude" {
		t.Fatalf("Protocol() = %q, want %q", got, "claude")
	}
}
