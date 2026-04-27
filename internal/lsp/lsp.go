// Package lsp implements a minimal Language Server Protocol server for
// PureLang. It speaks JSON-RPC 2.0 over the LSP framing protocol on
// stdin/stdout and supports:
//
//   - initialize / initialized / shutdown / exit
//   - textDocument/didOpen, didChange, didSave, didClose
//   - publishDiagnostics produced by the PureLang parser and checker
//   - textDocument/formatting via internal/fmtter
//   - textDocument/hover for built-in std functions
//
// This is intentionally a small but useful starting point. It is enough
// to drive linting and formatting in any LSP-capable editor.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"purelang/internal/checker"
	"purelang/internal/fmtter"
	"purelang/internal/parser"
)

// Server holds the LSP server state.
type Server struct {
	in     *bufio.Reader
	out    io.Writer
	logw   io.Writer
	mu     sync.Mutex
	docs   map[string]string
	closed bool
}

// NewServer creates a new LSP server bound to the given streams.
// `logw` may be nil; if set, internal log lines are written there.
func NewServer(in io.Reader, out io.Writer, logw io.Writer) *Server {
	return &Server{
		in:   bufio.NewReader(in),
		out:  out,
		logw: logw,
		docs: map[string]string{},
	}
}

// Serve runs the LSP loop until shutdown/exit.
func (s *Server) Serve() error {
	for !s.closed {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := s.handle(msg); err != nil {
			s.log("handle: %v", err)
		}
	}
	return nil
}

func (s *Server) log(format string, args ...interface{}) {
	if s.logw == nil {
		return
	}
	fmt.Fprintf(s.logw, "[lsp] "+format+"\n", args...)
}

type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (s *Server) readMessage() (rawMessage, error) {
	contentLength := -1
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			return rawMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n := 0
			fmt.Sscanf(line, "Content-Length: %d", &n)
			contentLength = n
		}
	}
	if contentLength < 0 {
		return rawMessage{}, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.in, body); err != nil {
		return rawMessage{}, err
	}
	var m rawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return rawMessage{}, err
	}
	return m, nil
}

func (s *Server) writeMessage(v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.out.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := s.out.Write(body); err != nil {
		return err
	}
	return nil
}

func (s *Server) reply(id json.RawMessage, result interface{}) error {
	return s.writeMessage(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func (s *Server) replyError(id json.RawMessage, code int, message string) error {
	return s.writeMessage(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func (s *Server) notify(method string, params interface{}) error {
	return s.writeMessage(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (s *Server) handle(m rawMessage) error {
	switch m.Method {
	case "initialize":
		return s.handleInitialize(m)
	case "initialized":
		return nil
	case "shutdown":
		return s.reply(m.ID, nil)
	case "exit":
		s.closed = true
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(m)
	case "textDocument/didChange":
		return s.handleDidChange(m)
	case "textDocument/didSave":
		return s.handleDidSave(m)
	case "textDocument/didClose":
		return s.handleDidClose(m)
	case "textDocument/formatting":
		return s.handleFormatting(m)
	case "textDocument/hover":
		return s.handleHover(m)
	}
	if len(m.ID) > 0 {
		return s.replyError(m.ID, -32601, "method not found: "+m.Method)
	}
	return nil
}

type initParams struct {
	Capabilities map[string]interface{} `json:"capabilities"`
}

func (s *Server) handleInitialize(m rawMessage) error {
	caps := map[string]interface{}{
		"textDocumentSync": 1, // full sync
		"hoverProvider":    true,
		"documentFormattingProvider": true,
	}
	return s.reply(m.ID, map[string]interface{}{
		"capabilities": caps,
		"serverInfo": map[string]string{
			"name":    "purelang-lsp",
			"version": "0.1.0",
		},
	})
}

type didOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

func (s *Server) handleDidOpen(m rawMessage) error {
	var p didOpenParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return err
	}
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	return s.diagnose(p.TextDocument.URI)
}

type didChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

func (s *Server) handleDidChange(m rawMessage) error {
	var p didChangeParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return err
	}
	if len(p.ContentChanges) > 0 {
		s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
	}
	return s.diagnose(p.TextDocument.URI)
}

type didSaveParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
	Text *string `json:"text,omitempty"`
}

func (s *Server) handleDidSave(m rawMessage) error {
	var p didSaveParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return err
	}
	if p.Text != nil {
		s.docs[p.TextDocument.URI] = *p.Text
	}
	return s.diagnose(p.TextDocument.URI)
}

type didCloseParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func (s *Server) handleDidClose(m rawMessage) error {
	var p didCloseParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return err
	}
	delete(s.docs, p.TextDocument.URI)
	return s.notify("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         p.TextDocument.URI,
		"diagnostics": []interface{}{},
	})
}

func (s *Server) diagnose(uri string) error {
	src := s.docs[uri]
	var diags []map[string]interface{}
	prog, err := parser.Parse(src)
	if err != nil {
		line, col, msg := parsePosError(err.Error())
		diags = append(diags, diagnostic(line, col, msg, 1 /* error */))
	} else {
		c := checker.New()
		errs := c.Check(prog)
		for _, e := range errs {
			line, col, msg := parsePosError(e)
			diags = append(diags, diagnostic(line, col, msg, 2 /* warning */))
		}
	}
	if diags == nil {
		diags = []map[string]interface{}{}
	}
	return s.notify("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": diags,
	})
}

// parsePosError parses messages that begin with "[line:col] message".
func parsePosError(msg string) (int, int, string) {
	if !strings.HasPrefix(msg, "[") {
		return 0, 0, msg
	}
	end := strings.Index(msg, "]")
	if end < 0 {
		return 0, 0, msg
	}
	hdr := msg[1:end]
	rest := strings.TrimSpace(msg[end+1:])
	parts := strings.SplitN(hdr, ":", 2)
	if len(parts) != 2 {
		return 0, 0, msg
	}
	var line, col int
	fmt.Sscanf(parts[0], "%d", &line)
	fmt.Sscanf(parts[1], "%d", &col)
	if line > 0 {
		line--
	}
	if col > 0 {
		col--
	}
	return line, col, rest
}

func diagnostic(line, col int, msg string, severity int) map[string]interface{} {
	return map[string]interface{}{
		"range": map[string]interface{}{
			"start": map[string]int{"line": line, "character": col},
			"end":   map[string]int{"line": line, "character": col + 1},
		},
		"severity": severity,
		"source":   "purelang",
		"message":  msg,
	}
}

type formattingParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func (s *Server) handleFormatting(m rawMessage) error {
	var p formattingParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return s.replyError(m.ID, -32602, err.Error())
	}
	src, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return s.reply(m.ID, []interface{}{})
	}
	formatted, err := fmtter.Format(src)
	if err != nil {
		return s.replyError(m.ID, -32603, err.Error())
	}
	if formatted == src {
		return s.reply(m.ID, []interface{}{})
	}
	lines := strings.Count(src, "\n")
	edit := map[string]interface{}{
		"range": map[string]interface{}{
			"start": map[string]int{"line": 0, "character": 0},
			"end":   map[string]int{"line": lines + 1, "character": 0},
		},
		"newText": formatted,
	}
	return s.reply(m.ID, []interface{}{edit})
}

var hoverDocs = map[string]string{
	"print":      "print(args...): writes a space-separated representation of args followed by a newline.",
	"println":    "println(args...): same as print; included for ergonomics.",
	"len":        "len(value): length of a list or string.",
	"first":      "first(list): the first element of a list, or null if empty.",
	"last":       "last(list): the last element of a list, or null if empty.",
	"push":       "push(list, value): returns a new list with value appended.",
	"range":      "range(start?, end, step?): integer list from start to end exclusive.",
	"reverse":    "reverse(list): reversed copy of a list.",
	"sort_ints":  "sort_ints(list): sorted copy of a list of integers.",
	"upper":      "upper(s): uppercase string.",
	"lower":      "lower(s): lowercase string.",
	"trim":       "trim(s): trims leading and trailing whitespace.",
	"split":      "split(s, sep): splits a string by a separator.",
	"join":       "join(list, sep): joins a list of values into a string.",
	"contains":   "contains(s, sub): true if sub is a substring of s.",
	"to_int":     "to_int(s): parses an integer from a string, or null on failure.",
	"to_string":  "to_string(v): converts a value to its string representation.",
	"abs":        "abs(n): absolute value.",
	"min":        "min(...): minimum of arguments.",
	"max":        "max(...): maximum of arguments.",
	"sqrt":       "sqrt(x): square root.",
	"pow":        "pow(x, y): x raised to the y.",
	"floor":      "floor(x): largest integer not greater than x.",
	"ceil":       "ceil(x): smallest integer not less than x.",
	"read_file":  "read_file(path): contents of a file as a string.",
	"write_file": "write_file(path, contents): write a string to a file.",
	"exists":     "exists(path): true if a file or directory exists at path.",
}

type hoverParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
}

func (s *Server) handleHover(m rawMessage) error {
	var p hoverParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return s.replyError(m.ID, -32602, err.Error())
	}
	src, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return s.reply(m.ID, nil)
	}
	word := wordAt(src, p.Position.Line, p.Position.Character)
	if word == "" {
		return s.reply(m.ID, nil)
	}
	doc, ok := hoverDocs[word]
	if !ok {
		return s.reply(m.ID, nil)
	}
	return s.reply(m.ID, map[string]interface{}{
		"contents": map[string]string{
			"kind":  "markdown",
			"value": "**" + word + "**\n\n" + doc,
		},
	})
}

func wordAt(src string, line, col int) string {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	l := lines[line]
	if col < 0 || col >= len(l) {
		return ""
	}
	isIdent := func(r byte) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_'
	}
	start := col
	for start > 0 && isIdent(l[start-1]) {
		start--
	}
	end := col
	for end < len(l) && isIdent(l[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return l[start:end]
}
