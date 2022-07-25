package main

import (
	"fmt"
	"os"

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
	err = client.Run()
	return
}
