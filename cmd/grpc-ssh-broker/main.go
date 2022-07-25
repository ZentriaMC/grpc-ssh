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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
		}
	}

	if err := configureLogging(true); err != nil {
		panic(err)
	}

	defer func() { _ = zap.L().Sync() }()

	if err := entrypoint(args, sshForceCommand); err != nil {
		exitError := &ExitError{}
		exitCode := 1
		if errors.As(err, &exitError) {
			zap.L().With(zap.String("section", "broker")).Error("exit", zap.Error(exitError.Unwrap()))
			exitCode = exitError.code
		} else {
			zap.L().With(zap.String("section", "broker")).Error("unhandled error", zap.Error(err))
		}

		_ = zap.L().Sync() // os.Exit does not run deferreds
		os.Exit(exitCode)
	}
}

func entrypoint(args []string, sshForceCommand bool) (err error) {
	zap.L().With(zap.String("section", "broker")).Debug("init", zap.String("version", core.Version))
	zap.L().With(zap.String("section", "broker")).Debug("args", zap.Strings("value", args))
	zap.L().With(zap.String("section", "broker")).Debug("ssh original command", zap.String("value", os.Getenv("SSH_ORIGINAL_COMMAND")))

	serviceName := args[2]

	configPath := "./examples/grpc-ssh-broker.services.yaml"
	zap.L().With(zap.String("section", "broker")).Debug("loading configuration", zap.String("path", configPath))

	var f *os.File
	if f, err = os.OpenFile(configPath, os.O_RDONLY, 0); err != nil {
		err = fmt.Errorf("unable to open configuration at '%s': %w", configPath, err)
		return
	}

	var config broker.Configuration
	if err = yaml.NewDecoder(f).Decode(&config); err != nil {
		err = fmt.Errorf("unable to parse configuration at '%s': %w", configPath, err)
		return
	}

	url, ok := config.DetermineTargetURL(serviceName, "")
	if !ok {
		err = NewExitError(fmt.Errorf("no such service: '%s'", serviceName), 2)
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

	zap.L().With(zap.String("section", "broker")).Debug("connecting", zap.String("address", address), zap.String("protocol", protocol))

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

func configureLogging(debugMode bool) (err error) {
	var cfg zap.Config

	if debugMode {
		cfg = zap.NewDevelopmentConfig()
		cfg.Level.SetLevel(zapcore.DebugLevel)
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.Development = false
	} else {
		cfg = zap.NewProductionConfig()
		cfg.Level.SetLevel(zapcore.InfoLevel)
	}

	cfg.OutputPaths = []string{
		"stderr",
	}

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	_ = zap.ReplaceGlobals(logger)
	return
}
