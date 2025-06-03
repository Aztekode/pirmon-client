package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type ServiceLog struct {
	Hostname    string    `json:"hostname"`
	IP          string    `json:"ip"`
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"`
	Timestamp   time.Time `json:"timestamp"`
	Error       string    `json:"error,omitempty"`
}

type ServerResponse struct {
	UpdateConfig *Config `json:"update_config"`
}

type AutoStartAlert struct {
	ServiceName string    `json:"service_name"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	Hostname    string    `json:"hostname"`
	IP          string    `json:"ip"`
}

// LogErrorToFile guarda errores y payloads en client.log
func LogErrorToFile(err error, payload []byte) {
	f, fileErr := os.OpenFile("client.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		log.Println("Error al abrir client.log:", fileErr)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] Error al enviar logs: %s\nPayload:\n%s\n\n", timestamp, err.Error(), string(payload))
	f.WriteString(logEntry)
}

func sendAutoStartAlert(serverURL, version, serviceName string) {
	hostname, _ := os.Hostname()
	ip, _ := GetOutboundIP()

	alert := AutoStartAlert{
		ServiceName: serviceName,
		Timestamp:   time.Now(),
		Message:     "El servicio fue iniciado automáticamente por el monitor.",
		Hostname:    hostname,
		IP:          ip,
	}

	payload, err := json.Marshal(alert)
	if err != nil {
		LogErrorToFile(err, payload)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/%s/log/service-auto-start", serverURL, version), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		LogErrorToFile(err, payload)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		err := fmt.Errorf("Respuesta HTTP: %d: %s", resp.StatusCode, string(body))
		LogErrorToFile(err, payload)
	}
}

// checkServices revisa el estado de los servicios indicados
func checkServices(config Config) []ServiceStatus {
	var results []ServiceStatus

	m, err := mgr.Connect()
	if err != nil {
		log.Println("Error al conectar con el manejador de servicios:", err)
		return results
	}
	defer m.Disconnect()

	for _, cfg := range config.Services {
		status := ServiceStatus{Name: cfg.Name}
		svc, err := m.OpenService(cfg.Name)
		if err != nil {
			status.Status = "not found"
			status.Error = err.Error()
		} else {
			s, err := svc.Query()
			if err != nil {
				status.Status = "unknown"
				status.Error = err.Error()
			} else {
				switch s.State {
				case 1:
					status.Status = "stopped"
				case 4:
					status.Status = "running"
				default:
					status.Status = fmt.Sprintf("state_%d", s.State)
				}

				// Verifica estado esperado vs real
				if cfg.ExpectedStatus != "" && status.Status != cfg.ExpectedStatus {
					// Guardamos el error pero NO cambiamos aún el status
					status.Error = fmt.Sprintf("Estado actual '%s' difiere del esperado '%s'", status.Status, cfg.ExpectedStatus)

					// Creamos una copia del status antes de actuar
					results = append(results, status)

					// Luego intentamos iniciar el servicio
					if cfg.ExpectedStatus == "running" && status.Status == "stopped" && cfg.AutoStartIfStopped {
						err := svc.Start()
						if err != nil {
							status.Error += fmt.Sprintf(" | Falló al iniciar: %s", err.Error())
						} else {
							status.Error += " | Servicio iniciado automáticamente."
							status.Status = "running"
							sendAutoStartAlert(config.ServerURL, config.ServerVersion, cfg.Name)
						}

						// Registramos el nuevo estado después de intentar iniciar
						results = append(results, ServiceStatus{
							Name:   cfg.Name,
							Status: status.Status,
							Error:  status.Error,
						})

						svc.Close()
						continue // ya agregamos ambos estados, continuamos
					}
				}
			}
			svc.Close()
		}
		results = append(results, status)
	}

	return results
}

func GetOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// runClientLoop ejecuta el cliente en bucle
func runClientLoop() {
	config := readConfig()
	for {
		var logs []ServiceLog
		timestamp := time.Now()
		serviceStatuses := checkServices(config)
		hostname, _ := os.Hostname()
		ip, _ := GetOutboundIP()

		for _, s := range serviceStatuses {
			// Buscar configuración para el servicio
			var svcCfg *ServiceConfig
			for i := range config.Services {
				if config.Services[i].Name == s.Name {
					svcCfg = &config.Services[i]
					break
				}
			}

			// Filtrar según only_report si está definido
			if svcCfg != nil && svcCfg.OnlyReport != "" && s.Status != svcCfg.OnlyReport {
				continue // Saltar este estado si no coincide con only_report
			}

			logs = append(logs, ServiceLog{
				Hostname:    hostname,
				IP:          ip,
				ServiceName: s.Name,
				Status:      s.Status,
				Timestamp:   timestamp,
				Error:       s.Error,
			})
		}

		payload, _ := json.Marshal(logs)

		resp, err := http.Post(fmt.Sprintf("%s/api/%s/log/report", config.ServerURL, config.ServerVersion), "application/json", bytes.NewBuffer(payload))
		if err != nil {
			log.Println("Error al enviar logs:", err)
			LogErrorToFile(err, payload)
		} else {
			defer resp.Body.Close()
			var response ServerResponse
			body, _ := ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, &response)
			if response.UpdateConfig != nil {
				log.Println("Configuración actualizada desde el servidor.")
				writeConfig(*response.UpdateConfig)
				config = *response.UpdateConfig
			}
		}

		time.Sleep(time.Duration(config.ReportInterval) * time.Second)
	}
}
