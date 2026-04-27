package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// pipeStream is an io.ReadWriter for both directions; not used in tests.
// Tests construct messages by directly writing to a buffer.

func writeRPC(buf *bytes.Buffer, payload map[string]interface{}) {
	body, _ := json.Marshal(payload)
	fmt.Fprintf(buf, "Content-Length: %d\r\n\r\n", len(body))
	buf.Write(body)
}

// Drain reads server output messages.
type splitWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *splitWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *splitWriter) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestLSPInitialize(t *testing.T) {
	var in bytes.Buffer
	var out splitWriter
	writeRPC(&in, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	writeRPC(&in, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "exit",
	})
	srv := NewServer(&in, &out, io.Discard)
	if err := srv.Serve(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, `"capabilities"`) {
		t.Errorf("expected capabilities in: %s", got)
	}
	if !strings.Contains(got, `"purelang-lsp"`) {
		t.Errorf("expected serverInfo: %s", got)
	}
}

func TestLSPDiagnostics(t *testing.T) {
	var in bytes.Buffer
	var out splitWriter
	writeRPC(&in, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	writeRPC(&in, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":  "file:///t.pure",
				"text": "x = ", // syntax error
			},
		},
	})
	writeRPC(&in, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "exit",
	})
	srv := NewServer(&in, &out, io.Discard)
	if err := srv.Serve(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "publishDiagnostics") {
		t.Errorf("expected diagnostics, got: %s", got)
	}
	if !strings.Contains(got, "file:///t.pure") {
		t.Errorf("expected uri in diagnostics: %s", got)
	}
}

func TestWordAt(t *testing.T) {
	src := "use std.io\nprint x"
	if w := wordAt(src, 1, 0); w != "print" {
		t.Errorf("got %q", w)
	}
	if w := wordAt(src, 1, 6); w != "x" {
		t.Errorf("got %q", w)
	}
}
