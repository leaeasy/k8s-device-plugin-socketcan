package socketcan

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/kubernetes-device-plugins/pkg/dockerutils"
	"github.com/vishvananda/netlink"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	fakeDeviceHostPath = "/var/run/device-plugin-socketcan-fakedev"
	nicCreationRetries = 10
)

// DevicePlugin is the represents the device plugin and implements
// the Kuberentes device plugin interface
type DevicePlugin struct {
	allocationCh chan *Allocation
	device_name  string
	client       *dockerutils.Client
}

type Allocation struct {
	DeviceID            string
	DeviceContainerPath string
}

// Implementation of the Kubernetes device plugin interface

// GetDevicePluginOptions return options for the device plugin.
// Implementation of the 'DevicePluginServer' interface.
func (DevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return nil, nil
}

// ListAndWatch communicates changes of device states and returns a
// new device list. Implementation of the 'DevicePluginServer' interface.
func (p *DevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	devices := []*pluginapi.Device{{
		ID:     p.device_name,
		Health: pluginapi.Healthy,
	}}

	s.Send(&pluginapi.ListAndWatchResponse{Devices: devices})

	for {
		time.Sleep(10 * time.Second)
	}
}

// Allocate is resposible to make the device available during the
// container creation process. Implementation of the 'DevicePluginServer' interface.
func (p *DevicePlugin) Allocate(c context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	for _, req := range r.GetContainerRequests() {
		var devices []*pluginapi.DeviceSpec
		for _, deviceID := range req.GetDevicesIDs() {
			dev := &pluginapi.DeviceSpec{}
			dev.HostPath = fakeDeviceHostPath
			dev.ContainerPath = fmt.Sprintf("/tmp/device-plugin-socketcan/%s", deviceID)
			dev.Permissions = "r"

			devices = append(devices, dev)

			p.allocationCh <- &Allocation{
				DeviceID:            deviceID,
				DeviceContainerPath: dev.ContainerPath,
			}

		}
		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{
			Devices: devices,
		})
	}

	return &response, nil
}

// PreStartContainer is called during registration phase of a container.
// Implementation of the 'DevicePluginServer' interface.
func (DevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}

// Implement PluginInterfaceStart from kubevirts device-plugin-manager

func (p *DevicePlugin) Start() error {
	err := createFakeDevice()
	if err != nil {
		glog.Exitf("Failed to create fake device: %s", err)
	}

	client, err := dockerutils.NewClient()
	if err != nil {
		glog.V(3).Info("Failed to connect to Docker")
		panic(err)
	}
	p.client = client

	go p.createVCANNic()

	return nil
}

// Additional functions
func (p *DevicePlugin) createVCANNic() {
	for alloc := range p.allocationCh {
		glog.V(3).Infof("New allocation request: %v", alloc)
		for i := 0; i < nicCreationRetries; i++ {
			if created := func() bool {
				containerID, err := p.client.GetContainerIDByMountedDevice(alloc.DeviceContainerPath)
				if err != nil {
					glog.V(3).Infof("Container was not found, due to: %s", err.Error())
					return false
				}

				containerPID, err := p.client.GetPidByContainerID(containerID)
				if err != nil {
					glog.V(3).Infof("Failed to obtain container's pid, due to: %s", err.Error())
					return false
				}

				// err = p.CreateVxcanPairAndAddToCangwRule(alloc.DeviceID, containerPID)
				err = p.CreateVxcanPairAndAddToCangwRule(alloc.DeviceID, containerPID)
				if err == nil {
					glog.V(3).Info("Successfully create vcan interface")
					return true
				}

				glog.V(3).Infof("Pod attachment failed with: %s", err.Error())
				return false
			}(); created {
				break
			}

			time.Sleep(time.Duration(i) * time.Second)
		}
	}
}

// // Creates the named vcan interface inside the pod namespace.
func (p *DevicePlugin) MoveSocketcanIntoPod(ifname string, containerPid int) error {
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		return err
	}

	return netlink.LinkSetNsPid(link, containerPid)
}

func (p *DevicePlugin) CreateVxcanPairAndAddToCangwRule(ifname string, containerPid int) error {
	glog.V(3).Infof("create vxcan %s to container's pid %d", ifname, containerPid)
	vxlanname := fmt.Sprintf("%s_%d", ifname, containerPid) // can0_92011
	commands := []string{
		// 创建vxcan pair: vxcan0_1 and vxcan00_1
		fmt.Sprintf("ip link add vx%s_1 type vxcan peer name vx%s0_1", vxlanname, vxlanname),

		// 将vxcan00_1移到pod1的netns内
		fmt.Sprintf("ip link set vx%s0_1 netns %d", vxlanname, containerPid),

		fmt.Sprintf("ip link set vx%s_1 up", vxlanname),

		// 将pod1内的vxcan00_1改名，改成can0
		fmt.Sprintf("nsenter -t %d -n ip link set vx%s0_1 name %s", containerPid, vxlanname, ifname),
		fmt.Sprintf("nsenter -t %d -n ip link set %s up", containerPid, ifname),

		// 将host的can0读出流量转发到vxcan0_1
		fmt.Sprintf("cangw -A -s %s -d vx%s_1 -e", ifname, vxlanname),
		// 将vxcan0_1写入流量转发到can0
		fmt.Sprintf("cangw -A -s vx%s_1 -d %s -e", vxlanname, ifname),
	}

	for _, command := range commands {
		if _, err := exec.Command("sh", "-c", command).CombinedOutput(); err != nil {
			glog.Errorf("%s failed: %s", command, err)
		}
	}

	return nil
}

func createFakeDevice() error {
	_, statErr := os.Stat(fakeDeviceHostPath)
	if statErr == nil {
		glog.V(3).Info("Fake block device already exists")
		return nil
	} else if os.IsNotExist(statErr) {
		glog.V(3).Info("Creating fake block device")
		cmd := exec.Command("mknod", fakeDeviceHostPath, "b", "1", "1")
		err := cmd.Run()
		return err
	} else {
		panic(statErr)
	}
}
