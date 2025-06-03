package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStats struct {
	Hostname    string          `json:"hostname"`
	IP          string          `json:"ip"`
	Timestamp   time.Time       `json:"timestamp"`
	CPUPercent  float64         `json:"cpu_percent"`
	MemoryUsed  uint64          `json:"memory_used"`
	MemoryTotal uint64          `json:"memory_total"`
	MemoryUsage float64         `json:"memory_usage_percent"`
	Services    []ServiceConfig `json:"services"`
}

func startSystemStatsWebSocket(config Config) {
	for {
		url := fmt.Sprintf("ws://%s/api/%s/ws/system-stats", config.ServerURLNoProtocol(), config.ServerVersion)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Println("Error al conectar WebSocket:", err)
			time.Sleep(10 * time.Second)
			continue
		}
		defer conn.Close()

		log.Println("WebSocket de stats del sistema conectado.")

		for {
			hostname, _ := os.Hostname()
			ip, _ := GetOutboundIP()

			cpuPercentages, _ := cpu.Percent(0, false)
			memStats, _ := mem.VirtualMemory()

			stats := SystemStats{
				Hostname:    hostname,
				IP:          ip,
				Timestamp:   time.Now(),
				CPUPercent:  cpuPercentages[0],
				MemoryUsed:  memStats.Used,
				MemoryTotal: memStats.Total,
				MemoryUsage: memStats.UsedPercent,
				Services:    config.Services,
			}

			payload, _ := json.Marshal(stats)
			err = conn.WriteMessage(websocket.TextMessage, payload)
			if err != nil {
				log.Println("WebSocket cerrado:", err)
				break
			}

			time.Sleep(time.Duration(config.MonitorInterval) * time.Millisecond)
		}
	}
}
