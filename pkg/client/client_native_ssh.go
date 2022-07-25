package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"time"

	"github.com/hashicorp/yamux"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

type NativeSSHDialer struct {
	agentConn net.Conn
	agent     sshagent.Agent
	sshConn   *ssh.Client
}

func NewNativeDialer(target SSHConnectionDetails) (dialer SSHDialer, err error) {
	s := &NativeSSHDialer{}

	usern := target.User
	if usern == "" {
		var currentUser *user.User
		if currentUser, err = user.Current(); err != nil {
			return
		}

		usern = currentUser.Username
	}

	port := target.Port
	if port == 0 {
		port = 22
	}

	sshConfig := &ssh.ClientConfig{
		User: usern,
		Auth: []ssh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 10 * time.Second,
	}

	if target.EnableAgent {
		if aerr := s.connectAgent(); aerr != nil {
			fmt.Fprintf(os.Stderr, "failed to connect to ssh agent: %s\n", err)
		} else if s.agentConn != nil {
			sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(s.agent.Signers))
		}
	}

	s.sshConn, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", target.Hostname, port), sshConfig)
	if err != nil {
		return
	}

	return s, nil
}

func (s *NativeSSHDialer) connectAgent() (err error) {
	socketPath := os.Getenv("SSH_AUTH_SOCK")
	if socketPath == "" {
		err = ErrNoSSHAgent
		return
	}

	var conn net.Conn
	if conn, err = net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"}); err != nil {
		err = multierr.Append(err, ErrNoSSHAgent)
		err = multierr.Append(err, err)
		return
	}

	s.agentConn = conn
	s.agent = sshagent.NewClient(conn)

	return
}

func (s *NativeSSHDialer) Dialer() GRPCDialer {
	sshConn := s.sshConn

	return func(ctx context.Context, addr string) (conn net.Conn, err error) {
		var session *ssh.Session
		if session, err = sshConn.NewSession(); err != nil {
			return
		}

		_ = yamux.ErrConnectionReset

		return NewWrappedConn(WrappedConnAdapter{
			GetReadPipe: func() (stdoutPipe io.ReadCloser) {
				stdoutPipe, session.Stdout = io.Pipe()
				return
			},
			GetWritePipe: func() (stdinPipe io.WriteCloser) {
				session.Stdin, stdinPipe = io.Pipe()
				return
			},
			Start: func() error {
				// TODO: wire stderr to a logger
				session.Stderr = os.Stderr

				cmd := fmt.Sprintf("grpc-ssh-broker client %s", addr)
				if err := session.Start(cmd); err != nil {
					return fmt.Errorf("failed to start command: %w", err)
				}
				return nil
			},
			Wait:  session.Wait,
			Close: session.Close,
		})
	}
}

func (s *NativeSSHDialer) Close() (err error) {
	if s.agentConn != nil {
		err = multierr.Append(err, s.agentConn.Close())
		s.agentConn = nil
		s.agent = nil
	}
	if s.sshConn != nil {
		err = multierr.Append(err, s.sshConn.Close())
	}
	return
}
