package gost

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type gostAuthBinding struct {
	Auther  string
	Authers []string
	Auth    any
}

type gostAuthConfig struct {
	Authers []struct {
		Name   string
		Plugin struct {
			Type    string
			Addr    string
			Token   string
			Timeout time.Duration
		}
	}
	Services []struct {
		Listener gostAuthBinding
		Handler  struct {
			Type            string
			gostAuthBinding `yaml:",inline"`
		}
	}
}

func validateGOSTConfig(path, endpoint, token string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return errors.New("provide the readable rendered GOST config with -gost-config")
	}
	var config gostAuthConfig
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return errors.New("invalid rendered GOST config")
	}
	if len(config.Authers) != 1 || len(config.Services) == 0 {
		return errors.New("GOST must define exactly one HTTP auther and at least one SOCKS5 service")
	}
	auther := config.Authers[0]
	if auther.Name == "" || auther.Plugin.Type != "http" || auther.Plugin.Addr != endpoint ||
		auther.Plugin.Token != token || auther.Plugin.Timeout <= 0 {
		return errors.New("GOST auther must use the configured loopback relay, local token and a positive timeout")
	}
	for _, service := range config.Services {
		if service.Handler.Type != "socks5" || service.Handler.Auther != auther.Name ||
			len(service.Handler.Authers) != 0 || service.Handler.Auth != nil ||
			service.Listener.Auther != "" || len(service.Listener.Authers) != 0 || service.Listener.Auth != nil {
			return errors.New("each SOCKS5 service must reference only the registered relay auther")
		}
	}
	return nil
}
