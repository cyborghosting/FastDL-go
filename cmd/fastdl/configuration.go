package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
)

type PathMapping map[string]string

type Server struct {
	Route           string      `json:"route"`
	BasePath        string      `json:"base_path"`
	Directories     PathMapping `json:"directories"`
	CompressMaxSize int64       `json:"compress_max_size" default:"1048576"`
}

type Configuration struct {
	Servers []Server `json:"servers"`
}

const (
	FastDLConfigKey     = "FASTDL_CONFIG"
	FastDLConfigDefault = "configuration.json"
)

func getConfigPath() string {
	if path, exists := os.LookupEnv(FastDLConfigKey); exists {
		return path
	}
	return FastDLConfigDefault
}

func loadConfiguration(filePath string) (*Configuration, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open configuration file: %w", err)
	}
	defer file.Close()

	var config Configuration
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}
	return &config, nil
}

func displayConfiguration(configPath string, config *Configuration) {
	log.Printf("Using configuration file: %s", configPath)
	log.Println("Configured FastDL servers:")
	for _, server := range config.Servers {
		displayServer(server)
	}
}

func displayServer(server Server) {
	log.Printf("  - route: %s", server.Route)
	log.Printf("    base_path: %s", server.BasePath)
	log.Println("    directories:")
	for key, value := range server.Directories {
		log.Printf("       %s: %s", key, value)
	}
	log.Printf("    compress_max_size: %d bytes", server.CompressMaxSize)
}

func runConfigurationLoader() (*Configuration, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("configuration file not found: %s", configPath)
		}
		if errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("permission denied accessing configuration file: %s", configPath)
		}
		return nil, fmt.Errorf("failed to stat configuration file %s: %w", configPath, err)
	}

	config, err := loadConfiguration(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %w", err)
	}

	displayConfiguration(configPath, config)

	return config, nil
}
