package main

import (
	"log"

	"golang.org/x/sys/windows/svc"
)

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Error detectando si se ejecuta como servicio: %v", err)
	}

	if isService {
		runService("pirmon-client", false)
	} else {
		log.Println("Ejecutando en modo consola...")
		var config Config = readConfig()
		go startSystemStatsWebSocket(config)
		runClientLoop()
	}
}
