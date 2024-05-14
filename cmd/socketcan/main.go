package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	socketcan "github.com/mpreu/k8s-device-plugin-socketcan/pkg"
)

func main() {
	flag.Parse()

	hw_devices := []string{}

	device_list := os.Getenv("SOCKETCAN_DEVICES")
	if device_list != "" {
		for _, device := range strings.Split(device_list, " ") {
			hw_devices = append(hw_devices, fmt.Sprintf("socketcan-%s", device))
		}
	}

	manager := dpm.NewManager(socketcan.Lister{Devices: hw_devices})
	manager.Run()
}
