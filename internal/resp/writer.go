package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Writer encodes RESP2 values to an io.Writer. Writes are buffered until
// Flush.
type Writer struct {
	w *bufio.Writer
}

// NewWriter returns a Writer that buffers encoded values into w.
func NewWriter(w io.Writer) *Writer {
	bw, ok := w.(*bufio.Writer)
	if !ok {
		bw = bufio.NewWriter(w)
	}
	return &Writer{w: bw}
}

// Write encodes v. It does not flush the underlying buffer.
func (w *Writer) Write(v Value) error {
	switch v.Type {
	case TypeSimpleString, TypeError:
		if strings.ContainsAny(v.Str, "\r\n") {
			return fmt.Errorf("resp: simple string or error contains CR or LF")
		}
		if err := w.w.WriteByte(byte(v.Type)); err != nil {
			return err
		}
		if _, err := w.w.WriteString(v.Str); err != nil {
			return err
		}
		_, err := w.w.WriteString("\r\n")
		return err
	case TypeInteger:
		if _, err := w.w.WriteString(":"); err != nil {
			return err
		}
		if _, err := w.w.WriteString(strconv.FormatInt(v.Int, 10)); err != nil {
			return err
		}
		_, err := w.w.WriteString("\r\n")
		return err
	case TypeBulkString:
		if v.Null {
			_, err := w.w.WriteString("$-1\r\n")
			return err
		}
		if _, err := w.w.WriteString("$"); err != nil {
			return err
		}
		if _, err := w.w.WriteString(strconv.Itoa(len(v.Str))); err != nil {
			return err
		}
		if _, err := w.w.WriteString("\r\n"); err != nil {
			return err
		}
		if _, err := w.w.WriteString(v.Str); err != nil {
			return err
		}
		_, err := w.w.WriteString("\r\n")
		return err
	case TypeArray:
		if v.Null {
			_, err := w.w.WriteString("*-1\r\n")
			return err
		}
		if _, err := w.w.WriteString("*"); err != nil {
			return err
		}
		if _, err := w.w.WriteString(strconv.Itoa(len(v.Array))); err != nil {
			return err
		}
		if _, err := w.w.WriteString("\r\n"); err != nil {
			return err
		}
		for _, el := range v.Array {
			if err := w.Write(el); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("resp: unknown type %q", byte(v.Type))
	}
}

// Flush writes any buffered data to the underlying writer.
func (w *Writer) Flush() error {
	return w.w.Flush()
}
