package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
)

type PathMapping map[string]string

const (
	fastDLConfigKey     = "FASTDL_CONFIG"
	fastDLConfigDefault = "configuration.json"
)

func getConfigPath() string {
	if path, exists := os.LookupEnv(fastDLConfigKey); exists {
		return path
	}
	return fastDLConfigDefault
}

func loadConfiguration(path string) (Configuration, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open configuration file: %w", err)
	}
	defer f.Close()
	c, err := parseConfiguration(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}
	return c, nil
}

func displayConfiguration(path string, c Configuration) {
	log.Printf("Using configuration file: %s", path)
	log.Printf("Configured FastDL servers:")
	for _, server := range c.GetServers() {
		displayServer(server)
	}
}

func displayServer(s Server) {
	log.Printf("  - route: %s", s.GetRoute())
	log.Printf("    installation_path: %s", s.GetInstallationPath())
	log.Printf("    dictionary:")
	for k, v := range s.GetDictionary() {
		log.Printf("       %s: %s", k, v)
	}
	log.Printf("    compress_max_size: %d bytes", s.GetCompressMaxSize())
}

func RunConfigurationLoader() (Configuration, error) {
	path := getConfigPath()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("configuration file not found: %s", path)
		}
		if errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("permission denied accessing configuration file: %s", path)
		}
		return nil, fmt.Errorf("failed to stat configuration file %s: %w", path, err)
	}

	c, err := loadConfiguration(path)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %w", err)
	}

	displayConfiguration(path, c)

	return c, nil
}
