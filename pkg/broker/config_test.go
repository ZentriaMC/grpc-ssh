package broker_test

import (
	"io/ioutil"
	"testing"

	"github.com/ZentriaMC/grpc-ssh/pkg/broker"
	"github.com/davecgh/go-spew/spew"
	"gopkg.in/yaml.v3"
)

func TestParseConfig(t *testing.T) {
	raw, err := ioutil.ReadFile("../../examples/grpc-ssh-broker.services.yaml")
	if err != nil {
		panic(err)
	}

	var config broker.Configuration
	if err := yaml.Unmarshal(raw, &config); err != nil {
		panic(err)
	}

	spew.Dump(config)
}

func TestURLDetermination(t *testing.T) {
	raw, err := ioutil.ReadFile("../../examples/grpc-ssh-broker.services.yaml")
	if err != nil {
		panic(err)
	}

	var config broker.Configuration
	if err := yaml.Unmarshal(raw, &config); err != nil {
		panic(err)
	}

	test := func(service, grpc string) {
		url, ok := config.DetermineTargetURL(service, grpc)
		if !ok {
			t.Errorf("service='%s', grpc='%s' failed to compute", service, grpc)
		}
		t.Logf("service='%s', grpc='%s' => '%s'", service, grpc, url)
	}

	test("helloworld", "")
	test("routeguide", "")
	test("merged", "/helloworld.Greeter")
	test("merged", "/routeguide.RouteGuide")

	spew.Dump(config)
}
