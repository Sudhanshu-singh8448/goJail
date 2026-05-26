package sandbox

import (
	"io"
	"strings"
	"testing"
)

func TestCompareOutput_Accepted(t *testing.T) {
	status := CompareOutput("hello\n", "hello\n")
	if status != StatusAccepted {
		t.Errorf("expected 'accepted', got %q", status)
	}
}

func TestCompareOutput_ExactEmpty(t *testing.T) {
	status := CompareOutput("", "")
	if status != StatusAccepted {
		t.Errorf("expected 'accepted', got %q", status)
	}
}

func TestCompareOutput_WhitespaceMismatch(t *testing.T) {
	tests := []struct {
		actual   string
		expected string
	}{
		{"hello\n", "hello"},
		{"  hello  \n", "hello"},
		{"hello", "hello\n"},
	}

	for _, tc := range tests {
		status := CompareOutput(tc.actual, tc.expected)
		if status != StatusOutputWhitespaceMismatch {
			t.Errorf("CompareOutput(%q, %q) = %q, want 'output_whitespace_mismatch'",
				tc.actual, tc.expected, status)
		}
	}
}

func TestCompareOutput_WrongOutput(t *testing.T) {
	status := CompareOutput("hello", "world")
	if status != StatusWrongOutput {
		t.Errorf("expected 'wrong_output', got %q", status)
	}
}

func TestComputeOverallStatus_AllAccepted(t *testing.T) {
	status := ComputeOverallStatus("ok", []string{"accepted", "accepted", "accepted"})
	if status != StatusOverallAccepted {
		t.Errorf("expected 'accepted', got %q", status)
	}
}

func TestComputeOverallStatus_BuildFailed(t *testing.T) {
	status := ComputeOverallStatus("failed", []string{})
	if status != StatusOverallBuildFailed {
		t.Errorf("expected 'build_failed', got %q", status)
	}
}

func TestComputeOverallStatus_FirstNonAccepted(t *testing.T) {
	status := ComputeOverallStatus("ok", []string{
		"accepted",
		"time_exceeded",
		"wrong_output",
	})
	if status != StatusTimeExceeded {
		t.Errorf("expected 'time_exceeded', got %q", status)
	}
}

func TestComputeOverallStatus_RuntimeError(t *testing.T) {
	status := ComputeOverallStatus("ok", []string{"accepted", "runtime_error"})
	if status != StatusRuntimeError {
		t.Errorf("expected 'runtime_error', got %q", status)
	}
}

func TestParseBuildStatus_Success(t *testing.T) {
	status := ParseBuildStatus(nil, "")
	if status != StatusBuildOK {
		t.Errorf("expected 'ok', got %q", status)
	}
}

func TestParseTestStatus_Success(t *testing.T) {
	status := ParseTestStatus(nil, "")
	if status != StatusAccepted {
		t.Errorf("expected 'accepted', got %q", status)
	}
}

func TestReadBounded_Short(t *testing.T) {
	r := strings.NewReader("short")
	data := readBounded(r, 100)
	if string(data) != "short" {
		t.Errorf("expected 'short', got %q", string(data))
	}
}

func TestReadBounded_Truncated(t *testing.T) {
	long := strings.Repeat("x", 200)
	r := strings.NewReader(long)
	data := readBounded(r, 50)
	if len(data) != 50 {
		t.Errorf("expected truncated to 50 bytes, got %d", len(data))
	}
	suffix := string(data[len(data)-len("[truncated]"):])
	if suffix != "[truncated]" {
		t.Errorf("expected truncation marker at end, got suffix %q", suffix)
	}
}

func TestReadBounded_ExactSize(t *testing.T) {
	exact := strings.Repeat("a", 50)
	r := strings.NewReader(exact)
	data := readBounded(r, 50)
	if string(data) != exact {
		t.Errorf("expected exact 50 bytes, got %d", len(data))
	}
}

func TestReadBounded_Empty(t *testing.T) {
	r := strings.NewReader("")
	data := readBounded(r, 100)
	if len(data) != 0 {
		t.Errorf("expected empty, got %d bytes", len(data))
	}
}

// Verify io.EOF is handled
func TestReadBounded_EOF(t *testing.T) {
	r := io.LimitReader(strings.NewReader("hello"), 5)
	data := readBounded(r, 100)
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}
