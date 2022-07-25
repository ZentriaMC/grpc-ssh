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

func (c *Configuration) DetermineTargetURL(serviceName string, grpcServicePath string) (url string, ok bool) {
	g := func(key string) (string, bool) {
		v, ok := c.cache.Load(key)
		if ok {
			return v.(string), true
		}
		return "", false
	}

	// Check if we have service in the cache
	key1 := fmt.Sprintf("bypath::%s::%s", serviceName, grpcServicePath)
	if grpcServicePath != "" {
		if url, ok = g(key1); ok {
			return
		}
	}

	key2 := fmt.Sprintf("byservice::%s", serviceName)
	if url, ok = g(key2); ok {
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

	if grpcServicePath != "" {
		for _, s := range service.URLs {
			for _, path := range s.Paths {
				if path == grpcServicePath {
					url = s.URL
					break
				}
			}
		}

		if url != "" {
			ok = true
			c.cache.Store(key1, url)
			return
		}
	}

	if url = service.URL; url != "" {
		ok = true
		c.cache.Store(key2, url)
	}

	return
}

type Service struct {
	serviceBase `yaml:",inline"`
	Name        string       `json:"name" yaml:"name"`
	URLs        []ServiceURL `json:"urls" yaml:"urls"`
}

func (s Service) String() string {
	var targetInfo strings.Builder
	if len(s.URLs) > 0 {
		var services []string
		for _, s := range s.URLs {
			services = append(services, s.String())
		}
		targetInfo.WriteString("to=[")
		targetInfo.WriteString(strings.Join(services, ", "))
		targetInfo.WriteString("]")
	} else {
		targetInfo.WriteString("to=")
		targetInfo.WriteString(s.URL)
	}

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

type serviceBase struct {
	URL     string              `json:"url" yaml:"url"`
	TLS     *TLSConfig          `json:"tls" yaml:"tls"`
	Header  map[string][]string `json:"header" yaml:"header"`
	Trailer map[string][]string `json:"trailer" yaml:"trailer"`
}

type TLSConfig struct {
	FromRemote  bool   `json:"from_remote" yaml:"from_remote"`
	CA          string `json:"ca" yaml:"ca"`
	Certificate string `json:"certificate" yaml:"certificate"`
	Key         string `json:"key" yaml:"key"`
}

func (t TLSConfig) String() string {
	return "TLSConfig(sensitive)"
}

type ServiceURL struct {
	serviceBase `yaml:",inline"`
	Paths       []string `json:"paths" yaml:"paths"`
}

func (s ServiceURL) String() string {
	var buf strings.Builder
	buf.WriteString(strings.Join(s.Paths, "; "))
	buf.WriteString(" => ")
	buf.WriteString(s.URL)
	return buf.String()
}
