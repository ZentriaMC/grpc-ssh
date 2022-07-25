package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"go.uber.org/atomic"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapio"
)

type OpenSSHDialer struct {
	counter   atomic.Int64
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
	master.Stderr = &zapio.Writer{
		Log:   zap.L().With(zap.String("section", "openssh-dialer"), zap.String("instance", "master"), zap.String("output", "ssh-stderr")),
		Level: zapcore.InfoLevel,
	}
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

		cmd.Stderr = &zapio.Writer{
			Log:   zap.L().With(zap.String("section", "openssh-dialer"), zap.Int64("n", s.counter.Inc()), zap.String("output", "ssh-stderr")),
			Level: zapcore.InfoLevel,
		}

		return NewWrappedConn(WrappedConnAdapter{
			GetReadPipe: func() (stdoutPipe io.ReadCloser) {
				stdoutPipe, _ = cmd.StdoutPipe()
				return stdoutPipe
			},
			GetWritePipe: func() (stdinPipe io.WriteCloser) {
				stdinPipe, _ = cmd.StdinPipe()
				return
			},
			Start: cmd.Start,
			Wait:  cmd.Wait,
			Close: func() error {
				return cmd.Process.Signal(os.Interrupt)
			},
		})
	}
}

func (s *OpenSSHDialer) commonArgs(extra []string) (args []string) {
	args = append(args, []string{
		"-x",
		"-oBatchMode=yes",
		"-oClearAllForwardings=yes",
		fmt.Sprintf("-oControlPath=%s/%s", s.socketDir, "%C"),
	}...)

	if !s.details.EnableAgent {
		args = append(args, "-oIdentityAgent=none")
	}

	if s.details.User != "" {
		args = append(args, "-oUser="+s.details.User)
	}

	if s.details.Port != 0 {
		args = append(args, fmt.Sprintf("-oPort=%d", s.details.Port))
	}

	args = append(args, extra...)
	return
}

func (s *OpenSSHDialer) Close() (err error) {
	args := s.commonArgs([]string{
		"-O", "stop",
		s.details.Hostname,
	})

	cmd := exec.Command(s.sshPath, args...)
	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("unable to stop ssh master: %w", err)
	}

	return
}

func (s *OpenSSHDialer) createMaster() *exec.Cmd {
	args := s.commonArgs([]string{
		"-N",
		"-f",
		"-oControlMaster=auto",
		s.details.Hostname,
	})
	return exec.Command(s.sshPath, args...)
}

func (s *OpenSSHDialer) createClient(target string) *exec.Cmd {
	args := s.commonArgs([]string{
		"-oControlMaster=no",
		s.details.Hostname,
		"grpc-ssh-broker client " + target,
	})
	return exec.Command(s.sshPath, args...)
}
