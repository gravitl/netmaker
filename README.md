
<p align="center">
  <img src="netmaker.png" width="75%"><break/>
</p>
<p align="center">
<i>Create and control automated virtual networks.</i> 
</p>

<p align="center">
  <a href="https://github.com/gravitl/netmaker/releases">
    <img src="https://img.shields.io/badge/Version-0.8.5-informational?style=flat-square" />
  </a>
  <a href="https://hub.docker.com/r/gravitl/netmaker/tags">
    <img src="https://img.shields.io/docker/pulls/gravitl/netmaker" />
  </a>  
  <a href="https://discord.gg/zRb9Vfhk8A">
    <img src="https://img.shields.io/badge/community-discord-informational" />
  </a>
  <a href="https://github.com/gravitl/netmaker/graphs/contributors">
    <img src="https://img.shields.io/github/commit-activity/m/gravitl/netmaker?color=blue" />
  </a>
  <a href="https://twitter.com/intent/follow?screen_name=gravitlcorp">
    <img src="https://img.shields.io/twitter/follow/gravitlcorp?style=social" />
  </a>
  <a href="https://www.youtube.com/channel/UCach3lJY_xBV7rGrbUSvkZQ">
    <img src="https://img.shields.io/youtube/channel/views/UCach3lJY_xBV7rGrbUSvkZQ?style=social" />
  </a>
</p>

# WireGuard® Automation from Homelab to Enterprise
- [x] Peer-to-Peer Mesh Networks
- [x] Site-to-Site Gateways
- [x] Private DNS
- [x] Kubernetes Multi-Cloud
- [x] Linux, Mac, Windows, iPhone, and Android

# Get Started in 5 Minutes

**For production-grade installations, visit the [Install Docs](https://netmaker.readthedocs.io/en/develop/install.html).**  
**For an HA install using helm on k8s, visit the [Helm Repo](https://github.com/gravitl/netmaker-helm/).**
1. Get a cloud VM with Ubuntu 20.04 and a public IP.
2. Open ports 443, 53, and 51821-51830/udp on the VM firewall and in cloud security settings.
3. Run the script **(see below for optional configurations)**:

`sudo wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/develop/scripts/nm-quick.sh | bash`

Upon completion, the logs will display a script that can be used to automatically connect Linux and Mac devices.

It will also display instructions for Windows, iPhone, and Android.

<img src="./docs/images/install-server.gif" width="50%" /><img src="./docs/images/visit-website.gif" width="50%" />

After installing Netmaker, check out the [Walkthrough](https://itnext.io/getting-started-with-netmaker-a-wireguard-virtual-networking-platform-3d563fbd87f0) and [Getting Started](https://netmaker.readthedocs.io/en/ma(ster/getting-started.html) guide to begin setting up networks. Or, check out some of our other [Tutorials](https://gravitl.com/resources) for different use cases, including Kubernetes.

### Optional configurations

**Deploy a "Hub-And-Spoke VPN" on the server**  
a. This will configure a standard VPN (non-meshed) for private internet access, with 10 clients (-c).  
b. `sudo wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/develop/scripts/nm-quick.sh | bash -s -- -v true -c 7`  

**Specify Domain sand Email**  
a. Make sure your wildcard domain is pointing towards the server ip.  
b. `sudo wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/develop/scripts/nm-quick.sh | bash -s -- -d mynetmaker.domain.com -e example@email.com`  

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

## Disclaimer
 [WireGuard](https://wireguard.com/) is a registered trademark of Jason A. Donenfeld.

## License

Netmaker's source code and all artifacts in this repository are freely available. All versions are published under the Server Side Public License (SSPL), version 1, which can be found here: [LICENSE.txt](./LICENSE.txt).
