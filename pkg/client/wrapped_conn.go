package client

import (
	"io"
	"net"
	"sync"
	"time"

	"go.uber.org/multierr"
)

var _ net.Conn = &WrappedConn{}

type WrappedConn struct {
	closed bool
	mtx    sync.Mutex

	stdin  io.WriteCloser
	stdout io.ReadCloser

	getReadPipe  func() io.ReadCloser
	getWritePipe func() io.WriteCloser
	start        func() error
	wait         func() error
	close        func() error
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
		getReadPipe:  adapter.GetReadPipe,
		getWritePipe: adapter.GetWritePipe,
		start:        adapter.Start,
		wait:         adapter.Wait,
		close:        adapter.Close,
	}

	w.stdin = w.getWritePipe()
	w.stdout = w.getReadPipe()
	if err = w.start(); err != nil {
		return
	}

	go w.waitCommand()

	return
}

func (w *WrappedConn) waitCommand() {
	_ = w.wait()
	_ = w.Close()
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
	err = multierr.Append(err, w.close())
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
