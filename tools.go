//go:build tools
// +build tools

package tools

//nolint
import (
	_ "google.golang.org/grpc/examples/helloworld/greeter_server"
	_ "google.golang.org/grpc/examples/route_guide/server"
)
