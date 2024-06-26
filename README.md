# SocketCAN Device Plugin for Kubernetes

## Intro
`SocketCAN` is a set of CAN drivers in the Linux kernel contributed by Volkswagen. Via different kernel modules support for real hardware CAN devices as well as virtual loopback CAN devices can be enabled. This Kubernetes device plugin is responsible to provide the corresponding devices inside of docker containers.

## Supported vcan modules
Right now, the device plugin supports the usage of the virtual can devices provided by the `vcan` and `can-gw` kernel module.

## Build
To build the device plugin in a ready to use docker plugin run:

```
make build-docker
```

## Deployment
To deploy the device plugin as a DaemonSet in the Kubernetes cluster an example [yaml configuration](./deployments/socketcan-ds.yml) is provided.

## Usage of the device plugin
The device plugin is available through the namespace `socketcan.mpreu.de/vcan` or `socketcan.mpreu.de/socketcan-can0`.
An example deployment is given under [example/consumer-vcan](./example/consumer-vcan/dc.yml).

## Dependencies on the Host
To be able to consume the `SocketCAN` devices the corresponding kernel modules must be available on the Kubernetes compute nodes.
