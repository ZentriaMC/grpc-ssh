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

	setReadPipe  func(io.Reader)
	setWritePipe func(io.Writer)
	start        func() error
	wait         func() error
	close        func() error
}

type WrappedConnAdapter struct {
	SetReadPipe  func(io.Reader)
	SetWritePipe func(io.Writer)
	Start        func() error
	Wait         func() error
	Close        func() error
}

func NewWrappedConn(adapter WrappedConnAdapter) (w *WrappedConn, err error) {
	w = &WrappedConn{
		setReadPipe:  adapter.SetReadPipe,
		setWritePipe: adapter.SetWritePipe,
		start:        adapter.Start,
		wait:         adapter.Wait,
		close:        adapter.Close,
	}

	var readPipe io.Reader
	var writePipe io.Writer
	readPipe, w.stdin = io.Pipe()
	w.stdout, writePipe = io.Pipe()

	w.setReadPipe(readPipe)
	w.setWritePipe(writePipe)
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
