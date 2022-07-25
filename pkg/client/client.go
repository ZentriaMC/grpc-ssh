package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"time"

	"github.com/hashicorp/yamux"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

var (
	ErrNoSSHAgent = errors.New("no ssh agent available")
)

type SSHConnectionDetails struct {
	User        string
	Hostname    string
	Port        uint16
	EnableAgent bool
}

type SSHDialer struct {
	agentConn net.Conn
	agent     sshagent.Agent
	sshConn   *ssh.Client
}

type GRPCDialer func(ctx context.Context, addr string) (net.Conn, error)

func NewDialer(target SSHConnectionDetails) (s *SSHDialer, err error) {
	s = &SSHDialer{}

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
			// TODO log
		} else if s.agentConn != nil {
			sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(s.agent.Signers))
		}
	}

	s.sshConn, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", target.Hostname, port), sshConfig)
	if err != nil {
		return
	}

	return
}

func (s *SSHDialer) connectAgent() (err error) {
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

func (s *SSHDialer) Dialer() GRPCDialer {
	sshConn := s.sshConn

	return func(ctx context.Context, addr string) (conn net.Conn, err error) {
		var session *ssh.Session
		if session, err = sshConn.NewSession(); err != nil {
			return
		}

		_ = yamux.ErrConnectionReset

		cmd := fmt.Sprintf("grpc-ssh-broker client %s", addr)
		conn, err = NewWrappedConn(session, cmd)
		return
	}
}

func (s *SSHDialer) Close() (err error) {
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
