package config

import (
	"encoding/json"
	"io"
)

func parseConfiguration(r io.Reader) (conf Configuration, err error) {
	d := json.NewDecoder(r)
	conf, err = decodeV2(d)
	if err == nil {
		return
	}
	conf, err = decodeV1(d)
	if err == nil {
		return
	}
	return
}

type Configuration interface {
	GetServers() []Server
}

type Server interface {
	GetRoute() string
	GetRootDir() string
	GetBaseDirs() map[string]string
	GetCompressMaxSize() int64
}

func decodeV1(d *json.Decoder) (conf ConfigurationV1, err error) {
	err = d.Decode(&conf)
	return
}

type ConfigurationV1 struct {
	Servers []ServerV1 `json:"servers"`
}

func (c ConfigurationV1) GetServers() []Server {
	servers := make([]Server, len(c.Servers))
	for i, server := range c.Servers {
		servers[i] = server
	}
	return servers
}

type ServerV1 struct {
	Route           string            `json:"route"`
	RootDir         string            `json:"path_base"`
	BaseDirs        map[string]string `json:"path_mapping"`
	CompressMaxSize int64             `json:"compress_max_size" default:"0"`
}

func (s ServerV1) GetRoute() string {
	return s.Route
}

func (s ServerV1) GetRootDir() string {
	return s.RootDir
}

func (s ServerV1) GetBaseDirs() map[string]string {
	return s.BaseDirs
}

func (s ServerV1) GetCompressMaxSize() int64 {
	return s.CompressMaxSize
}

func decodeV2(d *json.Decoder) (conf ConfigurationV2, err error) {
	err = d.Decode(&conf)
	return
}

type ConfigurationV2 struct {
	Servers []ServerV2 `json:"servers"`
}

func (c ConfigurationV2) GetServers() []Server {
	servers := make([]Server, len(c.Servers))
	for i, server := range c.Servers {
		servers[i] = server
	}
	return servers
}

type ServerV2 struct {
	Route           string            `json:"route"`
	RootDir         string            `json:"root"`
	BaseDirs        map[string]string `json:"bases"`
	CompressMaxSize int64             `json:"max_compress_size" default:"0"`
}

func (s ServerV2) GetRoute() string {
	return s.Route
}

func (s ServerV2) GetRootDir() string {
	return s.RootDir
}

func (s ServerV2) GetBaseDirs() map[string]string {
	return s.BaseDirs
}

func (s ServerV2) GetCompressMaxSize() int64 {
	return s.CompressMaxSize
}
