package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"

	"github.com/ZentriaMC/grpc-ssh/internal/core"
	"github.com/ZentriaMC/grpc-ssh/pkg/client"
)

func main() {
	if err := configureLogging(true); err != nil {
		panic(err)
	}

	defer func() { _ = zap.L().Sync() }()

	if err := entrypoint(); err != nil {
		zap.L().With(zap.String("section", "test-client")).Error("unhandled error", zap.Error(err))
	}
}

func entrypoint() (err error) {
	zap.L().With(zap.String("section", "test-client")).Debug("init", zap.String("version", core.Version))

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	var dialer client.SSHDialer
	var conn *grpc.ClientConn
	var res *pb.HelloReply

	dialer, err = client.NewDialer(client.SSHConnectionDetails{
		User:        "mark",
		Hostname:    "127.0.0.1",
		Port:        22,
		EnableAgent: true,
	})
	if err != nil {
		return
	}

	defer dialer.Close()

	dialOpts := []grpc.DialOption{
		grpc.WithContextDialer(dialer.Dialer()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if conn, err = grpc.DialContext(ctx, "helloworld", dialOpts...); err != nil {
		return
	}
	defer conn.Close()

	c := pb.NewGreeterClient(conn)

	res, err = c.SayHello(ctx, &pb.HelloRequest{
		Name: "mark",
	})
	if err != nil {
		return
	}

	fmt.Printf("%+v\n", res)

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
