package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"syscall"
	"time"
	"unsafe"

	"github.com/alexbrainman/printer"
	"golang.org/x/sys/windows"
)

var (
	winspool         = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinter  = winspool.NewProc("OpenPrinterW")
	procEnumJobs     = winspool.NewProc("EnumJobsW")
	procClosePrinter = winspool.NewProc("ClosePrinter")
)

type JobInfo1 struct {
	JobID        uint32
	pPrinterName *uint16
	pMachineName *uint16
	pUserName    *uint16
	pDocument    *uint16
	pDatatype    *uint16
	pStatus      *uint16
	Status       uint32
	Priority     uint32
	Position     uint32
	TotalPages   uint32
	PagesPrinted uint32
	Submitted    windows.Systemtime
}

type PrinterIssueReport struct {
	PrinterName string `json:"printer_name"`
	Document    string `json:"document"`
	User        string `json:"user"`
	StatusCode  uint32 `json:"status_code"`
	Timestamp   string `json:"timestamp"`
}

func utf16Ptr(s string) *uint16 {
	ptr, _ := syscall.UTF16PtrFromString(s)
	return ptr
}

func sendPrinterIssueReport(config Config, report PrinterIssueReport) {
	if config.ServerURL == "" || config.ServerVersion == "" {
		log.Println("⚠️ Configuración incompleta: faltan ServerURL o ServerVersion.")
		return
	}

	jsonData, err := json.Marshal(report)
	if err != nil {
		log.Printf("❌ Error serializando reporte de impresora: %v", err)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/%s/log/printer", config.ServerURL, config.ServerVersion), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Error enviando reporte de impresora: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ El servidor respondió con código: %d", resp.StatusCode)
	}
}

func checkPrinterQueue(config Config, printerName string) []PrinterIssueReport {
	var reports []PrinterIssueReport

	var hPrinter uintptr
	ret, _, _ := procOpenPrinter.Call(
		uintptr(unsafe.Pointer(utf16Ptr(printerName))),
		uintptr(unsafe.Pointer(&hPrinter)),
		0,
	)
	if ret == 0 || hPrinter == 0 {
		log.Printf("❌ No se pudo abrir la impresora '%s'", printerName)
		return reports
	}
	defer procClosePrinter.Call(hPrinter)

	var needed, returned uint32
	procEnumJobs.Call(hPrinter, 0, 10, 1, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))

	if needed == 0 {
		return reports
	}

	buf := make([]byte, needed)
	ret, _, _ = procEnumJobs.Call(
		hPrinter, 0, 10, 1,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)),
	)

	if ret == 0 {
		log.Printf("⚠️ No se pudieron leer trabajos para '%s'", printerName)
		return reports
	}

	entrySize := int(unsafe.Sizeof(JobInfo1{}))
	count := int(returned)

	for i := 0; i < count; i++ {
		job := (*JobInfo1)(unsafe.Pointer(&buf[i*entrySize]))
		document := windows.UTF16PtrToString(job.pDocument)
		user := windows.UTF16PtrToString(job.pUserName)
		status := job.Status

		fmt.Printf("🖨️ Impresora: %s\n", printerName)
		fmt.Printf("   📄 Documento: %s\n", document)
		fmt.Printf("   👤 Usuario: %s\n", user)
		fmt.Printf("   🛑 Estado: 0x%X\n", status)

		if status != 0 {
			fmt.Printf("   🚨 Hay un problema con el trabajo de impresión.\n")
			report := PrinterIssueReport{
				PrinterName: printerName,
				Document:    document,
				User:        user,
				StatusCode:  status,
				Timestamp:   time.Now().Format(time.RFC3339),
			}
			reports = append(reports, report)
			sendPrinterIssueReport(config, report)
		}
	}

	return reports
}

func ensureSpoolerRunning() {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		log.Printf("❌ No se pudo abrir el administrador de servicios: %v", err)
		return
	}
	defer windows.CloseServiceHandle(m)

	serviceName := syscall.StringToUTF16Ptr("Spooler")
	h, err := windows.OpenService(m, serviceName, windows.SERVICE_QUERY_STATUS|windows.SERVICE_START)
	if err != nil {
		log.Printf("❌ No se pudo abrir el servicio 'Spooler': %v", err)
		return
	}
	defer windows.CloseServiceHandle(h)

	var status windows.SERVICE_STATUS
	err = windows.QueryServiceStatus(h, &status)
	if err != nil {
		log.Printf("❌ No se pudo consultar el estado del servicio 'Spooler': %v", err)
		return
	}

	if status.CurrentState != windows.SERVICE_RUNNING {
		log.Println("🛠️ El servicio 'Spooler' está detenido. Intentando iniciarlo...")
		err = windows.StartService(h, 0, nil)
		if err != nil {
			log.Printf("❌ No se pudo iniciar el servicio 'Spooler': %v", err)
			return
		}

		// Esperar hasta que el servicio esté corriendo
		for i := 0; i < 10; i++ {
			err = windows.QueryServiceStatus(h, &status)
			if err != nil {
				log.Printf("⚠️ Error al consultar estado del servicio: %v", err)
				break
			}
			if status.CurrentState == windows.SERVICE_RUNNING {
				log.Println("✅ Servicio 'Spooler' está corriendo.")
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func InitializePrinterDetection(config Config) []PrinterIssueReport {
	var reports []PrinterIssueReport

	ensureSpoolerRunning()

	names, err := printer.ReadNames()
	if err != nil {
		log.Printf("❌ Error al leer nombres de impresoras: %v", err)
		return reports
	}
	if len(names) == 0 {
		log.Println("⚠️ No se encontraron impresoras instaladas.")
		return reports
	}

	for _, name := range names {
		reports = append(reports, checkPrinterQueue(config, name)...)
	}

	return reports
}
