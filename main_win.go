//go:build windows

package main

import (
	"flag"
	"fmt"
	"time"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

type veepnService struct{}

func (m *veepnService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	go run()
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func init() {
	flag.BoolVar(&args.serviceInstall, "service-install", false, "install program as windows service")
	flag.BoolVar(&args.serviceUninstall, "service-uninstall", false, "uninstall windows service")
	flag.StringVar(&args.serviceName, "service-name", "Veepn-proxy", "windows service name")
	flag.Parse()

	if args.serviceInstall && args.serviceUninstall {
		arg_fail("serviceInstall and serviceUninstall options are mutually exclusive")
	}

	if args.serviceInstall {
		err := installService(args.serviceName, "VeePN proxy service")
		if err != nil {
			fmt.Printf("Service installation error, %s\n", err)
			os.Exit(-1)
		}
		fmt.Println("Service installed.")
		os.Exit(0)
	}

	if args.serviceUninstall {
		err := uninstallService(args.serviceName)
		if err != nil {
			fmt.Printf("Service uninstallation error, %s\n", err)
			os.Exit(-1)
		}
		fmt.Println("Service uninstalled.")
		os.Exit(0)
	}
}

func installService(name, desc string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}

	s, err = m.CreateService(name, exepath,
		mgr.Config{DisplayName: desc, Description: desc, StartType: mgr.StartAutomatic},
		"-bind-address", args.bindAddress, "-country", args.country)

	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func uninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

func CheckRunService() {
	inService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Printf("failed to determine if we are running in service: %v", err)
	}
	if inService {
		svc.Run(args.serviceName, &veepnService{})
		os.Exit(0)
	}
}
