package client

import (
	"context"
	"net"
	"os/exec"
	"sync"
)

type OpenSSHDialer struct {
	mtx       sync.Mutex
	instances []openSSHInstance
}

type openSSHInstance struct {
	cmd *exec.Cmd
}

func NewOpenSSHDialer(target SSHConnectionDetails) (dialer SSHDialer, err error) {
	s := &OpenSSHDialer{}

	err = ErrNoOpenSSHClient
	if err != nil {
		return
	}

	return s, nil
}

func (s *OpenSSHDialer) Dialer() GRPCDialer {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		return nil, nil
	}
}

func (s *OpenSSHDialer) Close() (err error) {
	return
}
