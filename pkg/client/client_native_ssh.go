package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapio"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

type NativeSSHDialer struct {
	counter   atomic.Int64
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
			zap.L().With(zap.String("section", "native-ssh-dialer")).Warn("failed to connect to ssh agent", zap.Error(err))
		} else if s.agentConn != nil {
			zap.L().With(zap.String("section", "native-ssh-dialer")).Debug("connected to ssh agent")
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

		session.Stderr = &zapio.Writer{
			Log:   zap.L().With(zap.String("section", "native-ssh-dialer"), zap.Int64("n", s.counter.Inc()), zap.String("output", "ssh-stderr")),
			Level: zapcore.InfoLevel,
		}

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
				cmd := fmt.Sprintf("grpc-ssh-broker client %s", addr)
				return session.Start(cmd)
			},
			Wait:  session.Wait,
			Close: session.Close,
		})
	}
}

func (s *NativeSSHDialer) Close() (err error) {
	if s.sshConn != nil {
		err = multierr.Append(err, s.sshConn.Close())
	}
	if s.agentConn != nil {
		err = multierr.Append(err, s.agentConn.Close())
		s.agentConn = nil
		s.agent = nil
	}
	return
}
