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

	spew.Dump(&config)
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

	test := func(service string) {
		url, ok := config.DetermineTargetURL(service)
		if !ok {
			t.Errorf("service='%s' failed to compute", service)
		}
		t.Logf("service='%s' => '%s'", service, url)
	}

	test("helloworld")
	test("routeguide")
	test("merged")

	spew.Dump(&config)
}
