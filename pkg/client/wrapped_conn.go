package client

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

var _ net.Conn = &WrappedConn{}

type WrappedConn struct {
	session *ssh.Session
	mtx     sync.Mutex

	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func NewWrappedConn(session *ssh.Session, cmd string) (w *WrappedConn, err error) {
	w = &WrappedConn{
		session: session,
	}

	w.session.Stdin, w.stdin = io.Pipe()
	w.stdout, w.session.Stdout = io.Pipe()

	// TODO: wire stderr to a logger
	w.session.Stderr = os.Stderr

	if err = w.session.Start(cmd); err != nil {
		err = fmt.Errorf("failed to start command: %w", err)
		return
	}

	go w.waitCommand()

	return
}

func (w *WrappedConn) waitCommand() {
	err := w.session.Wait()
	if err != nil {
		fmt.Printf("wait err: %s\n", err)
	}
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
	err = multierr.Append(err, w.stdout.Close())
	err = multierr.Append(err, w.stdin.Close())
	err = multierr.Append(err, w.session.Close())
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
