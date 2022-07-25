package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"

	"github.com/ZentriaMC/grpc-ssh/internal/core"
	"github.com/ZentriaMC/grpc-ssh/pkg/client"
)

func main() {
	if err := entrypoint(); err != nil {
		fmt.Fprintf(os.Stderr, "unhandled error: %s\n", err)
	}
}

func entrypoint() (err error) {
	fmt.Println("version:", core.Version)

	var dialer *client.SSHDialer
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

	dialOpts := []grpc.DialOption{
		grpc.WithContextDialer(dialer.Dialer()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if conn, err = grpc.Dial("helloworld", dialOpts...); err != nil {
		return
	}
	defer conn.Close()

	ctx := context.Background()
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
