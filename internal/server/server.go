// Package server accepts Redis client connections and dispatches commands.
package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/M0d3v1/hafez/internal/resp"
)

// Handler executes one command. args[0] is the command name. It is called
// from the connection's goroutine and may run concurrently for different
// connections.
type Handler func(ctx context.Context, args []string) resp.Value

// Server is a RESP2 TCP server. Each accepted connection is served by its
// own goroutine. Cancel the context passed to Serve to stop accepting and
// unblock in-flight reads.
type Server struct {
	// Addr is the TCP address to listen on. An empty Addr means ":6379".
	// Listen rewrites Addr to the address the kernel actually bound, which
	// matters when the port is 0.
	Addr    string
	Handler Handler
	Logger  *slog.Logger

	ln net.Listener
}

// Listen opens the TCP listener. It is optional; Serve listens if needed.
func (s *Server) Listen() error {
	if s.ln != nil {
		return errors.New("server: already listening")
	}
	addr := s.Addr
	if addr == "" {
		addr = ":6379"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ln = ln
	s.Addr = ln.Addr().String()
	return nil
}

// ListenAndServe listens and then serves until ctx is cancelled or Accept fails.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	return s.Serve(ctx)
}

// Serve accepts connections until ctx is cancelled. The listener is closed
// and in-flight connection handlers return before Serve returns.
func (s *Server) Serve(ctx context.Context) error {
	if s.Handler == nil {
		if s.ln != nil {
			_ = s.ln.Close()
		}
		return errors.New("server: nil handler")
	}
	if s.ln == nil {
		if err := s.Listen(); err != nil {
			return err
		}
	}
	ln := s.ln
	log := s.logger()
	log.Info("listening", "addr", ln.Addr().String())

	var wg sync.WaitGroup
	defer func() {
		_ = ln.Close()
		wg.Wait()
	}()
	stop := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stop()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			s.serveConn(ctx, c, log)
		}(conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn, log *slog.Logger) {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	remote := conn.RemoteAddr().String()
	log.Debug("client connected", "remote", remote)
	defer log.Debug("client disconnected", "remote", remote)

	rd := resp.NewReader(conn)
	wr := resp.NewWriter(conn)
	for {
		if ctx.Err() != nil {
			return
		}
		v, err := rd.Read()
		if err != nil {
			s.handleReadError(wr, log, remote, ctx, err)
			return
		}
		args, err := commandArgs(v)
		if err != nil {
			s.handleReadError(wr, log, remote, ctx, err)
			return
		}
		if len(args) == 0 {
			continue
		}
		reply := s.Handler(ctx, args)
		if err := wr.Write(reply); err != nil {
			return
		}
		if err := wr.Flush(); err != nil {
			return
		}
	}
}

func (s *Server) handleReadError(wr *resp.Writer, log *slog.Logger, remote string, ctx context.Context, err error) {
	if ctx.Err() != nil || isClosedConn(err) {
		return
	}
	var pe *resp.ProtocolError
	if errors.As(err, &pe) {
		log.Warn("protocol error", "remote", remote, "err", pe.Reason)
		_ = wr.Write(resp.Error("ERR Protocol error: " + pe.Reason))
		_ = wr.Flush()
		return
	}
	log.Debug("read error", "remote", remote, "err", err)
}

func (s *Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func commandArgs(v resp.Value) ([]string, error) {
	if v.Type != resp.TypeArray || v.Null {
		return nil, &resp.ProtocolError{Reason: "expected array of bulk strings"}
	}
	args := make([]string, len(v.Array))
	for i, el := range v.Array {
		if el.Type != resp.TypeBulkString || el.Null {
			return nil, &resp.ProtocolError{Reason: "expected array of bulk strings"}
		}
		args[i] = el.Str
	}
	return args, nil
}

func isClosedConn(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed)
}
