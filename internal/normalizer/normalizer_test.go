package normalizer

import "testing"

func TestNormalizeUUID(t *testing.T) {
	result := Normalize("550e8400-e29b-41d4-a716-446655440000")
	if result != "<uuid>" {
		t.Errorf("got %q, want <uuid>", result)
	}
}

func TestNormalizeIPv4(t *testing.T) {
	result := Normalize("connection from 192.168.1.1")
	if result != "connection from <ip>" {
		t.Errorf("got %q, want %q", result, "connection from <ip>")
	}
}

func TestNormalizeNumber(t *testing.T) {
	result := Normalize("timeout after 30 seconds")
	if result != "timeout after <n> seconds" {
		t.Errorf("got %q, want %q", result, "timeout after <n> seconds")
	}
}

func TestNormalizePath(t *testing.T) {
	result := Normalize("failed to open /var/log/app.log")
	if result != "failed to open <path>" {
		t.Errorf("got %q, want %q", result, "failed to open <path>")
	}
}

func TestNormalizeMultiplePatterns(t *testing.T) {
	result := Normalize("request 550e8400-e29b-41d4-a716-446655440000 from 10.0.0.1 failed after 5 retries")
	if result != "request <uuid> from <ip> failed after <n> retries" {
		t.Errorf("got %q", result)
	}
}

func TestNormalizeAlreadyNormalized(t *testing.T) {
	result := Normalize("timeout after <n> seconds on host <ip>")
	if result != "timeout after <n> seconds on host <ip>" {
		t.Errorf("normalizer should not double-replace: %q", result)
	}
}

func TestNormalizeHex(t *testing.T) {
	result := Normalize("error code 0xdeadbeef occurred")
	if result != "error code <hex> occurred" {
		t.Errorf("got %q, want %q", result, "error code <hex> occurred")
	}
}

func TestNormalizeRequestID(t *testing.T) {
	result := Normalize("req-abc12345: timeout")
	if result != "<req>: timeout" {
		t.Errorf("got %q, want %q", result, "<req>: timeout")
	}
}

func TestNormalizeNoChange(t *testing.T) {
	input := "plain text message without any patterns"
	result := Normalize(input)
	if result != input {
		t.Errorf("got %q, want %q", result, input)
	}
}

func TestNormalizeHash(t *testing.T) {
	result := Normalize("commit abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	if result != "commit <hash>" {
		t.Errorf("got %q, want %q", result, "commit <hash>")
	}
}
