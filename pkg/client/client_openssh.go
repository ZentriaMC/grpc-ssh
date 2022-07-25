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
	sshPath   string
	details   SSHConnectionDetails
	socketDir string
}

func NewOpenSSHDialer(target SSHConnectionDetails) (dialer SSHDialer, err error) {
	s := &OpenSSHDialer{details: target}

	s.sshPath, err = exec.LookPath("ssh")
	if errors.Is(err, exec.ErrNotFound) {
		err = multierr.Append(err, ErrNoOpenSSHClient)
		return
	}

	tmpDir := os.TempDir()
	if len(tmpDir) > 20 {
		// XXX: macOS loves its massively long TMPDIR paths
		tmpDir = "/tmp"
	}

	if s.socketDir, err = os.MkdirTemp(tmpDir, "grpcsshctrl*"); err != nil {
		err = fmt.Errorf("unable to create tmpdir for ssh control sockets: %w\n", err)
		return
	}

	master := s.createMaster()
	master.Stderr = os.Stderr
	if err = master.Run(); err != nil {
		err = fmt.Errorf("failed to start ssh master: %w", err)
		return
	}

	dialer = s
	return
}

func (s *OpenSSHDialer) Dialer() GRPCDialer {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		cmd := s.createClient(addr)

		return NewWrappedConn(WrappedConnAdapter{
			GetReadPipe: func() (stdoutPipe io.ReadCloser) {
				stdoutPipe, _ = cmd.StdoutPipe()
				return stdoutPipe
			},
			GetWritePipe: func() (stdinPipe io.WriteCloser) {
				stdinPipe, _ = cmd.StdinPipe()
				return
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
				return cmd.Process.Signal(os.Interrupt)
			},
		})
	}
}

func (s *OpenSSHDialer) Close() (err error) {
	args := []string{
		"-O", "stop",
		fmt.Sprintf("-oControlPath=%s/%s", s.socketDir, "%C"),
	}

	if s.details.User != "" {
		args = append(args, "-oUser="+s.details.User)
	}

	if s.details.Port != 0 {
		args = append(args, fmt.Sprintf("-oPort=%d", s.details.Port))
	}

	args = append(args, s.details.Hostname)

	cmd := exec.Command(s.sshPath, args...)
	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("unable to stop ssh master: %w", err)
	}

	return
}

func (s *OpenSSHDialer) createMaster() *exec.Cmd {
	args := []string{
		"-x",
		"-N",
		"-f",
		"-oBatchMode=yes",
		"-oClearAllForwardings=yes",
		"-oSessionType=default",
		"-oControlMaster=auto",
		fmt.Sprintf("-oControlPath=%s/%s", s.socketDir, "%C"),
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

	args = append(args, s.details.Hostname)
	return exec.Command(s.sshPath, args...)
}

func (s *OpenSSHDialer) createClient(target string) *exec.Cmd {
	remoteCommand := fmt.Sprintf("grpc-ssh-broker client %s", target)

	args := []string{
		"-x",
		"-oBatchMode=yes",
		"-oClearAllForwardings=yes",
		"-oSessionType=default",
		"-oControlMaster=no",
		fmt.Sprintf("-oControlPath=%s/%s", s.socketDir, "%C"),
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
