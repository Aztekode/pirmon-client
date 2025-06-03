package main

import (
	"log"

	"golang.org/x/sys/windows/svc"
)

type pirmonService struct{}

func (m *pirmonService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.StartPending}
	go runClientLoop()
	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for c := range r {
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			s <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}

func runService(name string, isDebug bool) {
	err := svc.Run(name, &pirmonService{})
	if err != nil {
		log.Fatalf("Fallo al iniciar el servicio: %v", err)
	}
}
