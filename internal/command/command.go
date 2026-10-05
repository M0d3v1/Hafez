// Package command registers Redis commands and turns argument lists into
// RESP replies. Handlers do not touch the network.
package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/M0d3v1/hafez/internal/resp"
)

// Handler runs one command. args[0] is the command name as the client sent it.
type Handler func(ctx context.Context, args []string) resp.Value

type spec struct {
	name    string
	minArgs int
	maxArgs int // -1 means no upper bound
	fn      Handler
}

// Dispatcher looks up commands by name. Register must finish before the
// first Dispatch, and the registry is not mutated after that.
type Dispatcher struct {
	commands map[string]spec
}

// New returns an empty dispatcher.
func New() *Dispatcher {
	return &Dispatcher{commands: make(map[string]spec)}
}

// Register adds a command. Names are matched case-insensitively. minArgs and
// maxArgs count arguments after the command name. maxArgs -1 means unlimited.
func (d *Dispatcher) Register(name string, minArgs, maxArgs int, fn Handler) error {
	if name == "" || fn == nil {
		return errors.New("command: name and handler are required")
	}
	if minArgs < 0 || (maxArgs < -1) || (maxArgs >= 0 && maxArgs < minArgs) {
		return fmt.Errorf("command: invalid arity for %s", name)
	}
	key := strings.ToUpper(name)
	if _, ok := d.commands[key]; ok {
		return fmt.Errorf("command: %s already registered", name)
	}
	d.commands[key] = spec{
		name:    strings.ToLower(name),
		minArgs: minArgs,
		maxArgs: maxArgs,
		fn:      fn,
	}
	return nil
}

// Dispatch runs the command in args. Unknown commands and wrong arity produce
// Redis error values; they are not Go errors.
func (d *Dispatcher) Dispatch(ctx context.Context, args []string) resp.Value {
	if len(args) == 0 {
		return resp.Error("ERR unknown command ''")
	}
	cmd, ok := d.commands[strings.ToUpper(args[0])]
	if !ok {
		return resp.Error("ERR unknown command '" + safeToken(args[0]) + "'")
	}
	n := len(args) - 1
	if n < cmd.minArgs || (cmd.maxArgs >= 0 && n > cmd.maxArgs) {
		return resp.Error("ERR wrong number of arguments for '" + cmd.name + "' command")
	}
	return cmd.fn(ctx, args)
}

// safeToken keeps error replies on one RESP line.
func safeToken(s string) string {
	if strings.ContainsAny(s, "\r\n") {
		return "?"
	}
	return s
}
