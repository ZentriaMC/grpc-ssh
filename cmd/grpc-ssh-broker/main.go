package main

import (
	"crypto/tls"
	"crypto/x509"
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

	err = openConnection(url, nil)

	return
}

func openConnection(targetURL string, tlsConfig *broker.TLSConfig) (err error) {
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

	fmt.Fprintf(os.Stderr, "connecting=%s protocol=%s\n", address, protocol)

	var conn net.Conn
	if requiresTLS && (tlsConfig == nil || !tlsConfig.FromRemote) {
		cfg := &tls.Config{
			InsecureSkipVerify: tlsConfig != nil && tlsConfig.SkipVerify,
		}

		if tlsConfig != nil && tlsConfig.CA != "" {
			cfg.RootCAs = x509.NewCertPool()

			var buf []byte
			if buf, err = ioutil.ReadFile(tlsConfig.CA); err != nil {
				err = fmt.Errorf("unable to load CA certificate: %w", err)
				return
			}

			if !cfg.RootCAs.AppendCertsFromPEM(buf) {
				err = fmt.Errorf("unable to load CA certificate: %w", errors.New("no certificates were found"))
				return
			}
		}

		if tlsConfig != nil && tlsConfig.Certificate != "" && tlsConfig.Key != "" {

			// Set up certificate
			var cert tls.Certificate
			if cert, err = tls.LoadX509KeyPair(tlsConfig.Certificate, tlsConfig.Key); err != nil {
				return
			}
			cfg.Certificates = append(cfg.Certificates, cert)
		}

		if conn, err = tls.Dial(protocol, address, cfg); err != nil {
			return
		}
	} else {
		if conn, err = net.Dial(protocol, address); err != nil {
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_, err := io.Copy(conn, os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stdin copy err=%s\n", err)
		}
		cancel()
	}()

	go func() {
		_, err := io.Copy(os.Stdout, conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stdout copy err=%s\n", err)
		}
		cancel()
	}()

	<-ctx.Done()

	return
}
