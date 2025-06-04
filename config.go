package main

import (
	"io/ioutil"
	"log"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ServerURL       string          `yaml:"server_url"`
	ServerVersion   string          `yaml:"server_version"`
	ReportInterval  uint            `yaml:"report_interval"`
	MonitorInterval uint            `yaml:"monitor_interval"`
	Services        []ServiceConfig `yaml:"services"`
	EventLogMinutes int             `yaml:"event_log_minutes"`
}

func (c *Config) ServerURLNoProtocol() string {
	url := c.ServerURL
	if strings.HasPrefix(url, "http://") {
		return strings.TrimPrefix(url, "http://")
	}
	if strings.HasPrefix(url, "https://") {
		return strings.TrimPrefix(url, "https://")
	}
	return url
}

type ServiceConfig struct {
	Name               string `yaml:"name" json:"name"`
	ExpectedStatus     string `yaml:"expected_status" json:"expected_status"`
	AutoStartIfStopped bool   `yaml:"auto_start_if_stopped" json:"auto_start_if_stopped"`
	OnlyReport         string `yaml:"only_report" json:"only_report"`
	FetchEventLogs     bool   `yaml:"fetch_event_logs,omitempty"`
}

func readConfig() Config {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error leyendo config.yaml: %v", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatalf("Error parseando config.yaml: %v", err)
	}
	return config
}

func writeConfig(newConfig Config) {
	data, err := yaml.Marshal(newConfig)
	if err != nil {
		log.Println("Error al serializar nueva configuración:", err)
		return
	}
	err = ioutil.WriteFile("config.yaml", data, 0644)
	if err != nil {
		log.Println("Error al guardar nueva configuración:", err)
	}
}
