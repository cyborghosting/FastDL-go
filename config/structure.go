package config

import (
	"encoding/json"
	"io"
)

func parseConfiguration(r io.Reader) (Configuration, error) {
	d := json.NewDecoder(r)
	var conf configuration
	err := d.Decode(&conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

type Configuration interface {
	GetServers() []Server
}

type Server interface {
	GetRoute() string
	GetInstallationPath() string
	GetDictionary() map[string]string
	GetCompressMaxSize() int64
	GetCachePath() string
}

type configuration struct {
	Servers []server `json:"servers"`
}

func (c *configuration) GetServers() []Server {
	servers := make([]Server, len(c.Servers))
	for i, server := range c.Servers {
		servers[i] = &server
	}
	return servers
}

type server struct {
	Route            string            `json:"route"`
	InstallationPath string            `json:"installation_path"`
	Dictionary       map[string]string `json:"dictionary"`
	CompressMaxSize  int64             `json:"compress_max_size"`
	CachePath        string            `json:"cache_path"`
}

func (s *server) GetRoute() string {
	return s.Route
}

func (s *server) GetInstallationPath() string {
	return s.InstallationPath
}

func (s *server) GetDictionary() map[string]string {
	return s.Dictionary
}

func (s *server) GetCompressMaxSize() int64 {
	return int64(s.CompressMaxSize)
}

func (s *server) GetCachePath() string {
	return s.CachePath
}
