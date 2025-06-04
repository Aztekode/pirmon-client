package main

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/sys/windows/svc"
)

// Ejecuta el cliente en modo consola (no como servicio de Windows).
func runConsoleMode(config Config) {
	log.Println("🖥️ Ejecutando en modo consola...")

	// Monitoreo de impresoras
	go safeGoRoutine("printer monitor", func() {
		for {
			printJobs := InitializePrinterDetection(config)
			for _, job := range printJobs {
				fmt.Printf("🖨️ %s - 📄 %s - 👤 %s - 🚦 Estado: 0x%X\n",
					job.PrinterName, job.Document, job.User, job.StatusCode)
			}
			time.Sleep(30 * time.Second)
		}
	})

	// Conexión al WebSocket
	go safeGoRoutine("system stats websocket", func() {
		startSystemStatsWebSocket(config)
	})

	// Bucle principal
	runClientLoop()
}

// safeGoRoutine ejecuta una goroutine con protección contra panic y logging.
func safeGoRoutine(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("❌ Panic en goroutine '%s': %v", name, r)
			}
		}()
		log.Printf("🚀 Iniciando goroutine '%s'...", name)
		fn()
	}()
}

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("❌ Error detectando si se ejecuta como servicio: %v", err)
	}

	if isService {
		runService("pirmon-client", false)
	} else {
		config := readConfig()
		runConsoleMode(config)
	}
}
