package client

import (
	"errors"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ net.Conn = &WrappedConn{}

type WrappedConn struct {
	WrappedConnAdapter
	closed bool
	mtx    sync.Mutex

	stdin  io.WriteCloser
	stdout io.ReadCloser
}

type WrappedConnAdapter struct {
	GetReadPipe  func() io.ReadCloser
	GetWritePipe func() io.WriteCloser
	Start        func() error
	Wait         func() error
	Close        func() error
}

func NewWrappedConn(adapter WrappedConnAdapter) (w *WrappedConn, err error) {
	w = &WrappedConn{
		WrappedConnAdapter: adapter,
	}

	w.stdin = w.GetWritePipe()
	w.stdout = w.GetReadPipe()
	if err = w.Start(); err != nil {
		return
	}

	go w.waitCommand()

	return
}

func (w *WrappedConn) waitCommand() {
	err := w.Wait()
	var stderr *string
	if eerr := (*exec.ExitError)(nil); errors.As(err, &eerr) {
		v := string(eerr.Stderr)
		stderr = &v
	}

	zap.L().With(zap.String("section", "wrappedconn")).Debug("wait done, closing", zap.Error(err), zap.Stringp("stderr", stderr))
	err = w.Close()
	zap.L().With(zap.String("section", "wrappedconn")).Debug("closed", zap.Error(err))
}

func (w *WrappedConn) Read(p []byte) (n int, err error) {
	return w.stdout.Read(p)
}

func (w *WrappedConn) Write(b []byte) (n int, err error) {
	return w.stdin.Write(b)
}

func (w *WrappedConn) Close() (err error) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	if w.closed {
		return
	}
	err = multierr.Append(err, w.stdout.Close())
	err = multierr.Append(err, w.stdin.Close())
	err = multierr.Append(err, w.WrappedConnAdapter.Close())
	w.closed = true
	return
}

func (w *WrappedConn) LocalAddr() net.Addr {
	return nil
}

func (w *WrappedConn) RemoteAddr() net.Addr {
	return nil
}

func (w *WrappedConn) SetDeadline(t time.Time) (err error) {
	return
}

func (w *WrappedConn) SetReadDeadline(t time.Time) (err error) {
	return
}

func (w *WrappedConn) SetWriteDeadline(t time.Time) (err error) {
	return
}
