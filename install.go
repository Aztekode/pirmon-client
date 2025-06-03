package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc/mgr"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Uso: install.exe pirmon-client.exe")
		return
	}

	exePath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Println("Ruta inválida:", err)
		return
	}

	m, err := mgr.Connect()
	if err != nil {
		fmt.Println("No se puede conectar al gestor de servicios:", err)
		return
	}
	defer m.Disconnect()

	s, err := m.CreateService("pirmon-client", exePath, mgr.Config{
		DisplayName: "Pirmon Monitoring Client",
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		fmt.Println("Error al crear el servicio:", err)
		return
	}
	defer s.Close()

	fmt.Println("Servicio instalado correctamente.")
}
