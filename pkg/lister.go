package socketcan

import (
	"strings"

	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
)

const (
	resourceNamespace = "socketcan.generals.space"
)

// Lister implements the Lister interface from the
// device plugin manager
type Lister struct {
	Devices []string
}

// GetResourceNamespace declares the resource namespace in the FQDN format
func (s Lister) GetResourceNamespace() string {
	return resourceNamespace
}

// Discover which device plugins exist in the given resource namespace
func (s Lister) Discover(pluginListCh chan dpm.PluginNameList) {
	plugins := dpm.PluginNameList(s.Devices)

	pluginListCh <- plugins
}

// NewPlugin is called by the device plugin manager to create a device plugin
func (s Lister) NewPlugin(kind string) dpm.PluginInterface {
	glog.V(3).Infof("Creating device plugin %s", kind)
	return &DevicePlugin{
		allocationCh: make(chan *Allocation),
		device_name:  strings.TrimPrefix(kind, "socketcan-"),
	}
}
