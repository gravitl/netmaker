
<p align="center">
  <a href="https://netmaker.org">
  <img src="./img/netmaker.png" width="75%"><break/>
  </a>
</p>
<p align="center">
a platform for modern, blazing fast virtual networks 
</p>

<p align="center">
  <a href="https://github.com/gravitl/netmaker/releases">
    <img src="https://img.shields.io/badge/Version-0.14.1-informational?style=flat-square" />
  </a>
  <a href="https://hub.docker.com/r/gravitl/netmaker/tags">
    <img src="https://img.shields.io/docker/pulls/gravitl/netmaker" />
  </a>  
  <a href="https://goreportcard.com/report/github.com/gravitl/netmaker">
    <img src="https://goreportcard.com/badge/github.com/gravitl/netmaker" />
  </a>
  <a href="https://github.com/gravitl/netmaker/graphs/contributors">
    <img src="https://img.shields.io/github/commit-activity/m/gravitl/netmaker?color=blue" />
  </a>
  <a href="https://twitter.com/intent/follow?screen_name=netmaker_io">
    <img src="https://img.shields.io/twitter/follow/netmaker_io?style=social" />
  </a>
  <a href="https://www.youtube.com/channel/UCach3lJY_xBV7rGrbUSvkZQ">
    <img src="https://img.shields.io/youtube/channel/views/UCach3lJY_xBV7rGrbUSvkZQ?style=social" />
  </a>
</p>

# WireGuard® Automation from Homelab to Enterprise
- [x] Peer-to-Peer Mesh Networks
- [x] Kubernetes and Multi-Cloud Enablement
- [x] Remote Site Access via Gateway
- [x] OAuth and Private DNS Features
- [x] Fine-grained access controls 
- [x] Support for Linux, Mac, Windows, FreeBSD, iPhone, and Android

# Get Started in 5 Minutes  

**For DigitalOcean, use the 1-Click App:** <a href="https://marketplace.digitalocean.com/apps/netmaker?refcode=496ffcf1e252"><img src="https://www.deploytodo.com/do-btn-blue.svg" width="15%" /></a>  
**For production-grade installations, visit the [Install Docs](https://netmaker.readthedocs.io/en/master/install.html).**  
**For an HA install using helm on k8s, visit the [Helm Repo](https://github.com/gravitl/netmaker-helm/).**
1. Get a cloud VM with Ubuntu 20.04 and a public IP.
2. Open ports 443, 80, 53, 8883, and 51821-51830/udp on the VM firewall and in cloud security settings.
3. Run the script **(see below for optional configurations)**:

`wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh | sudo bash`

<img src="./img/visit-website.gif" width="49%" /><img src="./img/graph-readme.gif" width="49%" />

Upon completion, the logs will display the instructions to connect various devices. These can also be retrieved from the UI under "Access Keys."

After installing Netmaker, check out the [Walkthrough](https://itnext.io/getting-started-with-netmaker-a-wireguard-virtual-networking-platform-3d563fbd87f0) and [Getting Started](https://netmaker.readthedocs.io/en/master/getting-started.html) guides to learn more about configuring networks. Or, check out some of our other [Tutorials](https://gravitl.com/resources) for different use cases, including Kubernetes.

### Optional configurations

**Deploy a "Hub-And-Spoke VPN" on the server**  
*This will configure a standard VPN (non-meshed) for private internet access, with 10 clients (-c).*  
`wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh | sudo bash -s -- -v true -c 10`  

**Specify Domain and Email**  
*Make sure your wildcard domain is pointing towards the server ip.*  
`wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh | sudo bash -s -- -d mynetmaker.domain.com -e example@email.com`  

**Script Options**  
```
./nm-quick
-d domain.example.com # specify a wildcard domain for netmaker to use (DNS must point to this server)
-e myemail@example.com # specify your email (for SSL certificates)
-m true # create a default 'mesh network' (on by default)
-v false # create a default 'VPN network' (off by default)
-c 7 # number of client configs to create (for VPN network, 5 by default)
```

# Why Netmaker + WireGuard?

- Netmaker automates virtual networks between data centers, clouds, and edge devices, so you don't have to.

- Kernel WireGuard offers maximum speed, performance, and security. 

- Netmaker is built to scale from the small business to the enterprise. 

- Netmaker with WireGuard can be highly customized for peer-to-peer, site-to-site, Kubernetes, and more.

# Get Support

- [Community (Discord)](https://discord.gg/zRb9Vfhk8A)

- [Business (Subscription)](https://gravitl.com/plans/business)

- [Learning Resources](https://gravitl.com/resources)

# Community Projects

- [Netmaker + Traefik Proxy](https://github.com/bsherman/netmaker-traefik)

- [OpenWRT Netclient Packager](https://github.com/sbilly/netmaker-openwrt)

- [Golang GUI](https://github.com/mattkasun/netmaker-gui)

- [CoreDNS Plugin](https://github.com/gravitl/netmaker-coredns-plugin)

- [Multi-Cluster K8S Plugin](https://github.com/gravitl/netmak8s)

- [Terraform Provider](https://github.com/madacluster/netmaker-terraform-provider)

- [VyOS Integration](https://github.com/kylechase/vyos-netmaker)

- [Netmaker K3S](https://github.com/geragcp/netmaker-k3s)

## Disclaimer
 [WireGuard](https://wireguard.com/) is a registered trademark of Jason A. Donenfeld.

## License

Netmaker's source code and all artifacts in this repository are freely available. All versions are published under the Server Side Public License (SSPL), version 1, which can be found here: [LICENSE.txt](./LICENSE.txt).
