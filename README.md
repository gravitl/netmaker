
<p align="center">
  <a href="https://netmaker.io">
  <img src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/netmaker-teal.png" width="50%"><break/>
  </a>
</p>

<p align="center">
<a href="https://runacap.com/ross-index/q1-2022/" target="_blank" rel="noopener">
    <img src="https://runacap.com/wp-content/uploads/2022/06/ROSS_badge_white_Q1_2022.svg" alt="ROSS Index - Fastest Growing Open-Source Startups in Q1 2022 | Runa Capital"  width="15%"/>
</a>  
<a href="https://www.ycombinator.com/companies/netmaker/" target="_blank" rel="noopener">
    <img src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/y-combinator.png" alt="Y-Combinator" width="16%" />
</a>  

</p>

<p align="center">
  <a href="https://github.com/gravitl/netmaker/releases">
    <img src="https://img.shields.io/badge/Version-0.15.1-informational?style=flat-square" />
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
| :heavy_check_mark: Remote Access Gateways | :heavy_check_mark: OAuth                | :heavy_check_mark: FreeBSD              |
| :heavy_check_mark: Mesh VPNs              | :heavy_check_mark: Private DNS          | :heavy_check_mark: Mac                  |
| :heavy_check_mark: Site-to-Site           | :heavy_check_mark: Access Control Lists | :heavy_check_mark: Windows              |

# Get Started in 5 Minutes  

(For production-grade installations, visit the [Install Docs](https://netmaker.readthedocs.io/en/master/install.html).)  

1. Get a cloud VM with Ubuntu 20.04 and a public IP.
2. Open ports 443 and 51821-51830/udp on the VM firewall and in cloud security settings.
3. Run the script: `wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh | sudo bash`  
3.a. (with custom domain + email): `wget -qO - https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh | sudo bash -s -- -d mynetmaker.domain.com -e example@email.com`    

<p float="left" align="middle">
<img src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/readme.gif" />
</p>

After installing Netmaker, check out the [Walkthrough](https://itnext.io/getting-started-with-netmaker-a-wireguard-virtual-networking-platform-3d563fbd87f0) and [Getting Started](https://netmaker.readthedocs.io/en/master/getting-started.html) guides to learn more about configuring networks. Or, check out some of our other [Tutorials](https://www.netmaker.io/blog) for different use cases, including Kubernetes.

# Get Support

- [Discord](https://discord.gg/zRb9Vfhk8A)

- [Reddit](https://reddit.com/r/netmaker)

- [Learning Resources](https://netmaker.io/blog)

# Why Netmaker + WireGuard?

- Netmaker automates virtual networks between data centers, clouds, and edge devices, so you don't have to.

- Kernel WireGuard offers maximum speed, performance, and security. 

- Netmaker is built to scale from the small business to the enterprise. 

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

## Disclaimer
 [WireGuard](https://wireguard.com/) is a registered trademark of Jason A. Donenfeld.

## License

Netmaker's source code and all artifacts in this repository are freely available. All versions are published under the Server Side Public License (SSPL), version 1, which can be found here: [LICENSE.txt](./LICENSE.txt).
