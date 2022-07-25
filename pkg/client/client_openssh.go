package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"go.uber.org/multierr"
)

type OpenSSHDialer struct {
	sshPath string
	details SSHConnectionDetails
}

func NewOpenSSHDialer(target SSHConnectionDetails) (dialer SSHDialer, err error) {
	s := &OpenSSHDialer{details: target}

	s.sshPath, err = exec.LookPath("ssh")
	if errors.Is(err, exec.ErrNotFound) {
		err = multierr.Append(err, ErrNoOpenSSHClient)
		return
	}

	// TODO: ssh ControlMaster

	dialer = s
	return
}

func (s *OpenSSHDialer) Dialer() GRPCDialer {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		cmd := s.createClient(addr)

		// TODO: detect SSH client connection failure, otherwise this takes quite a while to time out
		return NewWrappedConn(WrappedConnAdapter{
			SetReadPipe: func(r io.Reader) {
				cmd.Stdin = r
			},
			SetWritePipe: func(w io.Writer) {
				cmd.Stdout = w
			},
			Start: func() error {
				// TODO: wire stderr to a logger
				cmd.Stderr = os.Stderr

				if err := cmd.Start(); err != nil {
					return fmt.Errorf("failed to start command: %w", err)
				}
				return nil
			},
			Wait: cmd.Wait,
			Close: func() error {
				return cmd.Process.Kill()
			},
		})
	}
}

func (s *OpenSSHDialer) Close() (err error) {
	return
}

func (s *OpenSSHDialer) createClient(target string) *exec.Cmd {
	remoteCommand := fmt.Sprintf("grpc-ssh-broker client %s", target)

	args := []string{
		"-v",
		"-x",
		"-oBatchMode=yes",
		"-oClearAllForwardings=yes",
		"-oSessionType=default",
	}

	if !s.details.EnableAgent {
		args = append(args, "-oIdentityAgent=none")
	}

	if s.details.User != "" {
		args = append(args, "-oUser="+s.details.User)
	}

	if s.details.Port != 0 {
		args = append(args, fmt.Sprintf("-oPort=%d", s.details.Port))
	}

	args = append(args, s.details.Hostname, remoteCommand)
	return exec.Command(s.sshPath, args...)
}
