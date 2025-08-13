# kertical
[![Go Report Card](https://goreportcard.com/badge/github.com/wjiec/kertical)](https://goreportcard.com/report/github.com/wjiec/kertical)
[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)
[![Go Tests](https://github.com/wjiec/kertical/actions/workflows/test.yml/badge.svg)](https://github.com/wjiec/kertical/actions/workflows/test.yml)

Managing and integrating your Homelab on Kubernetes.


## Introduction

The goal of kertical is to make it easy to manage and integrate your various
environments or services that are not in the cluster through kubernetes, and
of course to enhance the ability of kubernetes to provide services outside the
cluster through the controllers in kertical.


## Getting Started

This tutorial will detail how to configure and install the kertical to your cluster.

> [!CAUTION]
> Currently, kertical is in the development phase, so the design and implementation logic
> may change frequently. If you are using the program or are interested in it, feel free
> to ask any suggestions or questions.

### Install kertical

If you have Helm, you can deploy the mobius with the following command:
```bash
helm upgrade --install kertical-manager kertical-manager \
    --repo https://wjiec.github.io/kertical \
    --namespace kertical-system --create-namespace
```

It will install the kertical in the kertical-system namespace, creating that namespace if it doesn't already exist.

#### Aliyun registry

If you can't get the image directly through DockerHub, you can use Aliyun's image repository
by adding the following parameter to the installation command:
```bash
--set global.imageRegistry=registry.cn-hangzhou.aliyuncs.com
```

### Custom Resources

kertical implements the management and integration of various types of resources
inside and outside the cluster by means of customized resources (CRD), mainly providing
the following kinds.

#### ExternalProxy

As the name suggests, ExternalProxy creates a unified abstraction for out-of-cluster services, allowing
us to create Services or Ingresses for these out-of-cluster content, just like in-cluster resources.

For example, I have some standalone services on my Homelab (TrueNAS, OpenWrt, ...), and I'd like to
provide HTTPS ingress for those services via cert-manager or use names within the cluster to access
specific services.

We can accomplish what we've said above by doing so in the following way in this manifest file:
```yaml
kind: ExternalProxy
apiVersion: networking.kertical.com/v1alpha1
metadata:
  name: openwrt
spec:
  backends:
    - addresses:
        - ip: 172.16.1.1
      ports:
        - name: http
          port: 80
  ingress:
    rules:
      - host: openwrt.kertical.com
    tls:
      - hosts:
          - openwrt.kertical.com
        secretName: star-kertical-com
---
kind: ExternalProxy
apiVersion: networking.kertical.com/v1alpha1
metadata:
  name: truenas
spec:
  backends:
    - addresses:
        - ip: 172.16.1.8
      ports:
        - name: api
          port: 80
  service:
    metadata:
      name: truenas-api
      labels:
        app.kubernetes.io/name: truenas
```
More details can be found at `kubectl explain externalproxies --api-version=networking.kertical.com/v1alpha1`.

#### PortForwarding

PortForwarding can directly expose the configured port on the selected worker node. Unlike `nodePort` or `hostPort`,
PortForwarding allows you to customize the port number used on the worker node and is not limited by
Kubernetes' NodePort port number range restrictions. It can also forward data on all deployed worker nodes.
PortForwarding can be helpful when it is inconvenient to modify the Service. As shown below:
```yaml
kind: PortForwarding
apiVersion: networking.kertical.com/v1alpha1
metadata:
  name: kafka
spec:
  serviceRef:
    name: kafka-headless
  ports:
    - target: tcp-client
      hostPort: 9092
```

If you need to enable PortForwarding, you need to add the following parameter
during installation to enable the controller:
```bash
--set controller.forwarding.enabled=true
```

#### Known issues

If the proxy mode is not specified using `--set controller.forwarding.mode=[nftables|purego]` during
installation, the default behavior is to prefer the `nftables` mode.

If the cluster uses a CNI plugin based on `iptables` (such as the default `kindnetd`
installed in `kind`), the `nftables` mode will disrupt communication between clusters.


## Contributing

We warmly welcome your participation in the development of kertical.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html).

### Need-to-know conventions

* [API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- Access to a Kubernetes v1.21.0+ cluster.


## License

kertical is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
