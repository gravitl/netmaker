
<p align="center">
  <a href="https://netmaker.io">
  <img src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/netmaker-teal.png" width="50%"><break/>
  </a>
</p>

<p align="center">
<a href="https://runacap.com/ross-index/annual-2022/" target="_blank" rel="noopener">
    <img src="https://runacap.com/wp-content/uploads/2023/02/Annual_ROSS_badge_white_2022.svg" alt="ROSS Index - Fastest Growing Open-Source Startups | Runa Capital" width="17%" />
</a>  
<a href="https://www.ycombinator.com/companies/netmaker/" target="_blank" rel="noopener">
    <img src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/y-combinator.png" alt="Y-Combinator" width="16%" />
</a>  
</p>

<p align="center">
  <a href="https://github.com/gravitl/netmaker/releases">
    <img src="https://img.shields.io/badge/Version-1.0.0-informational?style=flat-square" />
  </a>
  <a href="https://hub.docker.com/r/gravitl/netmaker/tags">
    <img src="https://img.shields.io/docker/pulls/gravitl/netmaker?label=downloads" />
  </a>  
  <a href="https://goreportcard.com/report/github.com/gravitl/netmaker">
    <img src="https://goreportcard.com/badge/github.com/gravitl/netmaker" />
  </a>
  <a href="https://twitter.com/intent/follow?screen_name=netmaker_io">
    <img src="https://img.shields.io/twitter/follow/netmaker_io?label=follow&style=social" />
  </a>
  <a href="https://www.youtube.com/channel/UCach3lJY_xBV7rGrbUSvkZQ">
    <img src="https://img.shields.io/youtube/channel/views/UCach3lJY_xBV7rGrbUSvkZQ?style=social" />
  </a>
  <a href="https://reddit.com/r/netmaker">
    <img src="https://img.shields.io/reddit/subreddit-subscribers/netmaker?label=%2Fr%2Fnetmaker&style=social" />
  </a>  
  <a href="https://discord.gg/zRb9Vfhk8A">
    <img src="https://img.shields.io/discord/825071750290210916?color=%09%237289da&label=chat" />
  </a> 
</p>

# WireGuard<sup>®</sup> automation from homelab to enterprise

| Create                                    | Manage                                  | Automate                                |
|-------------------------------------------|-----------------------------------------|-----------------------------------------|
| :heavy_check_mark: WireGuard Networks     | :heavy_check_mark: Admin UI             | :heavy_check_mark: Linux                |
| :heavy_check_mark: Remote Access Gateways | :heavy_check_mark: OAuth                | :heavy_check_mark: Docker              |
| :heavy_check_mark: Mesh VPNs              | :heavy_check_mark: Private DNS          | :heavy_check_mark: Mac                  |
| :heavy_check_mark: Site-to-Site           | :heavy_check_mark: Access Control Lists | :heavy_check_mark: Windows              |

# Try Netmaker SaaS  

If you're looking for a managed service, you can get started with just a few clicks, visit [netmaker.io](https://account.netmaker.io) to create your netmaker server.  

# Self-Hosted Open Source Quick Start  

These are the instructions for deploying a Netmaker server on your cloud VM as quickly as possible. For more detailed instructions, visit the [Install Docs](https://docs.netmaker.io/docs/server-installation/quick-install#quick-install-script).  

1. Get a cloud VM with Ubuntu 24.04 and a static public IP.
2. Allow inbound traffic on port 443,51821 TCP and UDP to the VM firewall in cloud security settings, and for simplicity, allow outbound on All TCP and All UDP.
3. (recommended) Prepare DNS - Set a wildcard subdomain in your DNS settings for Netmaker, e.g. *.netmaker.example.com, which points to your VM's public IP.
4. Run the script to setup open source version of Netmaker: 

`sudo wget -qO /root/nm-quick.sh https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh && sudo chmod +x /root/nm-quick.sh && sudo /root/nm-quick.sh`

**<pre>To Install Self-Hosted PRO Version - https://docs.netmaker.io/docs/server-installation/netmaker-professional-setup</pre>** 



<p float="left" align="middle">
<img src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/readme.gif" />
</p>

After installing Netmaker, check out the [Walkthrough](https://itnext.io/getting-started-with-netmaker-a-wireguard-virtual-networking-platform-3d563fbd87f0) and [Getting Started](https://docs.netmaker.io/docs/getting-started) guides to learn more about configuring networks. Or, check out some of our other [Tutorials](https://www.netmaker.io/blog) for different use cases, including Kubernetes.

# Get Support

- [Discord](https://discord.gg/zRb9Vfhk8A)

- [Reddit](https://reddit.com/r/netmaker)

- [Learning Resources](https://netmaker.io/blog)

# Why Netmaker + WireGuard?

- Netmaker automates virtual networks between data centres, clouds, and edge devices, so you don't have to.

- Kernel WireGuard offers maximum speed, performance, and security. 

- Netmaker is built to scale from small businesses to enterprises. 

- Netmaker with WireGuard can be highly customized for peer-to-peer, site-to-site, Kubernetes, and more.

# Community Projects

- [Netmaker + Traefik Proxy](https://github.com/bsherman/netmaker-traefik)

- [OpenWRT Netclient Packager](https://github.com/sbilly/netmaker-openwrt)

- [Golang GUI](https://github.com/mattkasun/netmaker-gui)

- [CoreDNS Plugin](https://github.com/gravitl/netmaker-coredns-plugin)

- [Multi-Cluster K8S Plugin](https://github.com/gravitl/netmak8s)

- [Terraform Provider](https://github.com/madacluster/netmaker-terraform-provider)

- [VyOS Integration](https://github.com/kylechase/vyos-netmaker)

- [Netmaker K3S](https://github.com/geragcp/netmaker-k3s)

- [Run Netmaker + Netclient with Podman](https://github.com/agorgl/nm-setup)

## Disclaimer
 [WireGuard](https://wireguard.com/) is a registered trademark of Jason A. Donenfeld.

## License

Netmaker's source code and all artifacts in this repository are freely available.
All content that resides under the "pro/" directory of this repository, if that
directory exists, is licensed under the license defined in "pro/LICENSE".
All third party components incorporated into the Netmaker Software are licensed
under the original license provided by the owner of the applicable component.
Content outside of the above mentioned directories or restrictions above is
available under the "Apache Version 2.0" license as defined below.
All details for the licenses used can be found here: [LICENSE.md](./LICENSE.md).
