// Package mcp is kete's stdio MCP server.
//
// Wire: JSON-RPC 2.0, one message per newline-delimited line on stdin/
// stdout (per the MCP spec's stdio transport). Logs go to a file under
// ~/.kete/, never to stdout/stderr — those belong to the protocol.
// Hand-rolled per ADR 0012.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/dreamware-nz/kete/internal/store"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "kete"
)

// rpcRequest is the JSON-RPC 2.0 request shape we accept. ID is
// json.RawMessage so we round-trip whatever the client sent.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// Server holds the dispatch state. One per process; safe for concurrent
// dispatch on a single connection (we serialise writes).
type Server struct {
	store   *store.DB
	version string
	cache   *previewCache
	logger  *log.Logger

	writeMu sync.Mutex
}

// NewServer wires a server against an open store. version is the kete
// binary version string surfaced in `initialize`.
func NewServer(db *store.DB, version string, logOut io.Writer) *Server {
	return &Server{
		store:   db,
		version: version,
		cache:   newPreviewCache(),
		logger:  log.New(logOut, "", log.LstdFlags),
	}
}

// Serve runs the JSON-RPC loop until in is closed or returns an error.
// One request at a time per the stdio transport contract.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// MCP messages can be larger than the 64 KiB default.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		s.handleLine(ctx, line, out)
	}
	return scanner.Err()
}

func (s *Server) handleLine(ctx context.Context, line []byte, out io.Writer) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeErr(out, nil, errParse, "parse error: "+err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		s.writeErr(out, req.ID, errInvalidRequest, "jsonrpc must be \"2.0\"")
		return
	}
	// Notifications (no id) get no response.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	result, rpcErr := s.dispatch(ctx, req.Method, req.Params)
	if isNotification {
		return
	}
	if rpcErr != nil {
		s.writeErr(out, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}
	s.writeOK(out, req.ID, result)
}

func (s *Server) dispatch(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return s.handleInitialize(params)
	case "initialized", "notifications/initialized":
		return nil, nil
	case "ping":
		return struct{}{}, nil
	case "tools/list":
		return s.handleToolsList()
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	default:
		return nil, &rpcError{Code: errMethodNotFound, Message: "method not found: " + method}
	}
}

func (s *Server) writeOK(out io.Writer, id json.RawMessage, result any) {
	s.write(out, rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) writeErr(out io.Writer, id json.RawMessage, code int, msg string) {
	s.write(out, rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) write(out io.Writer, resp rpcResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		s.logger.Printf("marshal response: %v", err)
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := out.Write(append(b, '\n')); err != nil {
		s.logger.Printf("write response: %v", err)
	}
}

// initialize handshake.

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      map[string]any `json:"clientInfo"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      map[string]any `json:"serverInfo"`
}

func (s *Server) handleInitialize(_ json.RawMessage) (any, *rpcError) {
	return initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: map[string]any{
			"tools": map[string]any{},
		},
		ServerInfo: map[string]any{
			"name":    serverName,
			"version": s.version,
		},
	}, nil
}

// Helpers shared by tool dispatch.

func parseParams[T any](raw json.RawMessage) (T, *rpcError) {
	var v T
	if len(raw) == 0 {
		return v, nil
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, &rpcError{Code: errInvalidParams, Message: "invalid params: " + err.Error()}
	}
	return v, nil
}

// errToRPC wraps a non-RPC error for return as a tool result. MCP's
// convention: tool errors go in result.isError, not RPC error.
func errToRPC(err error) (any, *rpcError) {
	if errors.Is(err, store.ErrNotFound) {
		return toolError(err.Error()), nil
	}
	return toolError(fmt.Sprintf("internal error: %s", err)), nil
}
