package broker

import (
	"fmt"
	"strings"
	"sync"
)

type Configuration struct {
	Services []Service `json:"services" yaml:"services"`

	cache sync.Map `json:"-" yaml:"-"`
}

func (c *Configuration) DetermineTargetURL(serviceName string) (url string, ok bool) {
	g := func(key string) (string, bool) {
		v, ok := c.cache.Load(key)
		if ok {
			return v.(string), true
		}
		return "", false
	}

	key := fmt.Sprintf("byservice::%s", serviceName)
	if url, ok = g(key); ok {
		return
	}

	// Compute
	var service Service
	for _, s := range c.Services {
		if strings.EqualFold(s.Name, serviceName) {
			service = s
			break
		}
	}

	if url = service.URL; url != "" {
		ok = true
		c.cache.Store(key, url)
	}

	return
}

type Service struct {
	Name    string              `json:"name" yaml:"name"`
	URL     string              `json:"url" yaml:"url"`
	TLS     *TLSConfig          `json:"tls" yaml:"tls"`
	Header  map[string][]string `json:"header" yaml:"header"`
	Trailer map[string][]string `json:"trailer" yaml:"trailer"`
}

func (s Service) String() string {
	var targetInfo strings.Builder
	targetInfo.WriteString("to=")
	targetInfo.WriteString(s.URL)

	if t := s.TLS; t != nil {
		targetInfo.WriteString(", tls=")
		targetInfo.WriteString(t.String())
	}

	if h := s.Header; len(h) > 0 {
		targetInfo.WriteString(", header=[")
		var keys []string
		for k, v := range h {
			keys = append(keys, fmt.Sprintf("%s=%d", k, len(v)))
		}
		targetInfo.WriteString(strings.Join(keys, ", "))
		targetInfo.WriteString("]")
	}

	if t := s.Trailer; len(t) > 0 {
		targetInfo.WriteString(", trailer=[")
		var keys []string
		for k, v := range t {
			keys = append(keys, fmt.Sprintf("%s=%d", k, len(v)))
		}
		targetInfo.WriteString(strings.Join(keys, ", "))
		targetInfo.WriteString("]")
	}

	return fmt.Sprintf("Service(name=%s, %s)", s.Name, targetInfo.String())
}

type TLSConfig struct {
	FromRemote  bool   `json:"from_remote" yaml:"from_remote"`
	CA          string `json:"ca" yaml:"ca"`
	Certificate string `json:"certificate" yaml:"certificate"`
	Key         string `json:"key" yaml:"key"`
	SkipVerify  bool   `json:"skip_verify" yaml:"skip_verify"`
}

func (t TLSConfig) String() string {
	return fmt.Sprintf("TLSConfig(sensitive, skipVerify=%t)", t.SkipVerify)
}
