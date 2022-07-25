package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/context"
	"gopkg.in/yaml.v3"

	"github.com/ZentriaMC/grpc-ssh/internal/core"
	"github.com/ZentriaMC/grpc-ssh/pkg/broker"
)

func main() {
	sshForceCommand := false
	args := os.Args
	if cmd := os.Getenv("SSH_ORIGINAL_COMMAND"); cmd != "" {
		sshForceCommand = true
		args = strings.Split(cmd, " ")
		if len(args) <= 1 {
			args = []string{}
		} else {
			args = args[1:]
		}
	}

	if err := entrypoint(args, sshForceCommand); err != nil {
		exitError := &ExitError{}
		exitCode := 1
		if errors.As(err, &exitError) {
			fmt.Fprintf(os.Stderr, "%s\n", exitError.Error())
			exitCode = exitError.code
		} else {
			fmt.Fprintf(os.Stderr, "unhandled error: %s\n", err)
		}
		os.Exit(exitCode)
	}
}

func entrypoint(args []string, sshForceCommand bool) (err error) {
	fmt.Fprintf(os.Stderr, "version: %s\n", core.Version)

	fmt.Fprintf(os.Stderr, "v=%s\n", os.Getenv("SSH_CONNECTION"))
	fmt.Fprintf(os.Stderr, "v=%s\n", os.Getenv("SSH_CLIENT"))
	fmt.Fprintf(os.Stderr, "v=%s\n", os.Getenv("SSH_ORIGINAL_COMMAND"))

	serviceName := args[1]

	raw, err := ioutil.ReadFile("./examples/grpc-ssh-broker.services.yaml")
	if err != nil {
		panic(err)
	}

	var config broker.Configuration
	if err := yaml.Unmarshal(raw, &config); err != nil {
		panic(err)
	}

	url, ok := config.DetermineTargetURL(serviceName, "")
	if !ok {
		err = NewExitError(fmt.Errorf("no such service: %s", serviceName), 1)
		return
	}

	err = openConnection(url)

	return
}

func openConnection(targetURL string) (err error) {
	var parsed *url.URL
	if parsed, err = url.Parse(targetURL); err != nil {
		return
	}

	requiresTLS := false
	protocol := strings.ToLower(parsed.Scheme)
	address := ""

	switch protocol {
	case "http", "https":
		requiresTLS = protocol == "https"
		protocol = "tcp"
		address = parsed.Host
	case "unix":
		//protocol = "unix"
		address = parsed.Host
	default:
		err = fmt.Errorf("unsupported protocol: '%s'", protocol)
		return
	}

	_ = requiresTLS

	fmt.Fprintf(os.Stderr, "connecting=%s\n", address)

	var conn net.Conn
	if conn, err = net.Dial(protocol, address); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_, _ = io.Copy(conn, os.Stdin)
		cancel()
	}()

	go func() {
		_, _ = io.Copy(os.Stdout, conn)
		cancel()
	}()

	<-ctx.Done()

	return
}
