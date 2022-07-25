package main

import (
	"fmt"
	"os"

	"github.com/ZentriaMC/grpc-ssh/internal/core"
)

func main() {
	if err := entrypoint(); err != nil {
		fmt.Fprintf(os.Stderr, "unhandled error: %s\n", err)
	}
}

func entrypoint() (err error) {
	fmt.Println("version:", core.Version)
	return
}
