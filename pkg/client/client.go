package client

import (
	"context"
	"errors"
	"io"
	"net"
)

var (
	ErrNoSSHAgent      = errors.New("no ssh agent available")
	ErrNoOpenSSHClient = errors.New("could not locate OpenSSH client binary")
)

type SSHConnectionDetails struct {
	User               string
	Hostname           string
	Port               uint16
	EnableAgent        bool
	PreferNativeClient bool
}

type SSHDialer interface {
	io.Closer
	Dialer() GRPCDialer
}

type GRPCDialer func(ctx context.Context, addr string) (net.Conn, error)

func NewDialer(target SSHConnectionDetails) (dialer SSHDialer, err error) {
	if !target.PreferNativeClient {
		dialer, err = NewOpenSSHDialer(target)
		if err != nil && !errors.Is(err, ErrNoOpenSSHClient) {
			return nil, err
		} else if err == nil {
			return
		}
	}

	return NewNativeDialer(target)
}
