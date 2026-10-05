package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/M0d3v1/hafez/internal/command"
	"github.com/M0d3v1/hafez/internal/resp"
)

func TestCommandsOverTCP(t *testing.T) {
	srv := startServer(t)
	conn := dial(t, srv.Addr)
	rd, wr := resp.NewReader(conn), resp.NewWriter(conn)

	exchange := func(req resp.Value, want resp.Value) {
		t.Helper()
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		if err := wr.Write(req); err != nil {
			t.Fatal(err)
		}
		if err := wr.Flush(); err != nil {
			t.Fatal(err)
		}
		got, err := rd.Read()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("reply %#v, want %#v", got, want)
		}
	}

	exchange(resp.Array(resp.BulkString("PING")), resp.SimpleString("PONG"))
	exchange(resp.Array(resp.BulkString("ping"), resp.BulkString("hi")), resp.BulkString("hi"))
	exchange(resp.Array(resp.BulkString("ECHO"), resp.BulkString("hello")), resp.BulkString("hello"))
	exchange(resp.Array(resp.BulkString("COMMAND")), resp.Array())
	exchange(resp.Array(resp.BulkString("COMMAND"), resp.BulkString("COUNT")), resp.Integer(0))
	exchange(resp.Array(resp.BulkString("COMMAND"), resp.BulkString("DOCS")), resp.Array())
	exchange(
		resp.Array(resp.BulkString("GET"), resp.BulkString("foo")),
		resp.Error("ERR unknown command 'GET'"),
	)
	exchange(
		resp.Array(resp.BulkString("ECHO")),
		resp.Error("ERR wrong number of arguments for 'echo' command"),
	)
}

func TestInlineAndPipeline(t *testing.T) {
	srv := startServer(t)
	conn := dial(t, srv.Addr)
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn, "\r\nECHO \"hello world\"\r\nPING\r\n"); err != nil {
		t.Fatal(err)
	}
	rd := resp.NewReader(conn)
	got, err := rd.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, resp.BulkString("hello world")) {
		t.Fatalf("inline reply %#v", got)
	}
	got, err = rd.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, resp.SimpleString("PONG")) {
		t.Fatalf("second reply %#v", got)
	}
}

func TestProtocolErrorClosesConnection(t *testing.T) {
	srv := startServer(t)
	conn := dial(t, srv.Addr)
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn, "$-2\r\n"); err != nil {
		t.Fatal(err)
	}
	rd := resp.NewReader(conn)
	got, err := rd.Read()
	if err != nil {
		t.Fatal(err)
	}
	want := resp.Error("ERR Protocol error: invalid bulk length")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reply %#v, want %#v", got, want)
	}
	if _, err := rd.Read(); !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("after protocol error, Read() = %v, want EOF", err)
	}
	// A bad client must not take down the listener.
	conn2 := dial(t, srv.Addr)
	conn2.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn2, "*1\r\n$4\r\nPING\r\n"); err != nil {
		t.Fatal(err)
	}
	got, err = resp.NewReader(conn2).Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, resp.SimpleString("PONG")) {
		t.Fatalf("server did not survive a bad client: %#v", got)
	}
}

func TestRejectsNonCommandValues(t *testing.T) {
	srv := startServer(t)
	conn := dial(t, srv.Addr)
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
		t.Fatal(err)
	}
	got, err := resp.NewReader(conn).Read()
	if err != nil {
		t.Fatal(err)
	}
	want := resp.Error("ERR Protocol error: expected array of bulk strings")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reply %#v, want %#v", got, want)
	}
}

func TestConcurrentClients(t *testing.T) {
	srv := startServer(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", srv.Addr, 2*time.Second)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			rd, wr := resp.NewReader(conn), resp.NewWriter(conn)
			for n := 0; n < 20; n++ {
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				if err := wr.Write(resp.Array(resp.BulkString("PING"))); err != nil {
					t.Error(err)
					return
				}
				if err := wr.Flush(); err != nil {
					t.Error(err)
					return
				}
				got, err := rd.Read()
				if err != nil {
					t.Error(err)
					return
				}
				if !reflect.DeepEqual(got, resp.SimpleString("PONG")) {
					t.Errorf("reply %#v", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestShutdownClosesClients(t *testing.T) {
	srv, cancel := startServerCancel(t)
	conn := dial(t, srv.Addr)
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	cancel()
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected the connection to close on shutdown")
	}
}

func TestListenAndNilHandler(t *testing.T) {
	srv := &Server{Addr: "127.0.0.1:99999"}
	if err := srv.Listen(); err == nil {
		t.Fatal("expected listen error")
	}
	srv = &Server{Addr: "127.0.0.1:0"}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	if err := srv.Serve(context.Background()); err == nil {
		t.Fatal("expected nil handler error")
	}
}

func startServer(t *testing.T) *Server {
	t.Helper()
	srv, _ := startServerCancel(t)
	return srv
}

func startServerCancel(t *testing.T) (*Server, context.CancelFunc) {
	t.Helper()
	d := command.New()
	if err := command.RegisterConn(d); err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Addr:    "127.0.0.1:0",
		Handler: d.Dispatch,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})
	return srv, cancel
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
