
<p align="center">
  <img src="netmaker.png"><break/>
</p>
<p align="center">
<i>Connect any computers together over a secure, fast, private network, and manage multiple networks from a central server.</i> 
</p>

## What is Netmaker?
Netmaker is a tool for creating and managing virtual networks. If you have servers spread across multiple locations, data centers, or clouds, they all live on separate networks. This can make life very difficult. Netmaker takes all those machines and puts them on a single, flat network so that they can talk to each other easily and securely.

Think of it like Tailscale, ZeroTier, or Nebula, but faster, easier, and more dynamic.

You spin up the Netmaker server and UI, and then install the Netclient (agent) on your computers. Netmaker will do the rest. It will tell all of your computers how to reach each other and will keep them informed of any changes to the network.

Netmaker's handy dandy UI can be found [here](https://github.com/gravitl/netmaker-ui).

Under the hood, Netmaker uses WireGuard to create encrypted tunnels between every node in your virtual network, creating a full mesh overlay. Netmaker takes the work out of manually configuring machines with WireGuard and updating them every time you have a change in your network. The netclient agent is self-updating and pulls any necessary changes (such as new peers) from the server. 

## Why Netmaker?
 1. Create a flat, secure network between multiple/hybrid cloud environments
 2. Integrate central and edge services
 3. Secure a home or office network while providing remote connectivity
 4. Manage cryptocurrency proof-of-stake machines
 6. Provide an additional layer of security on an existing network
 7. Encrypt Kubernetes inter-node communications
 8. Secure site-to-site connections


<p align="center">
  <img src="mesh-diagram.png">
</p>

## Compatible Systems

Netmaker is primarily designed for **linux**, specifically **systemd-based linux.** This includes Fedora, Ubuntu, and Raspian. Just make sure you have WireGuard installed. Having a problem? Open an issue or Contact us.

In version 0.3 we have released Private DNS. Nameservers can be configured manually on any system, but to have the Netclient add dns automatically, it requires **resolvectl.**

In future releases, we have plans to support other platforms such as Windows and MacOS. 


## Docs
**For more information, please read the docs, or check out the Quick Start below:**

 - [General Usage](docs/USAGE.md)
 - [Troubleshooting](docs/TROUBLESHOOTING.md)
 - [API Documentation](docs/API.md)
 - [Product Roadmap](docs/ROADMAP.md)
 - [Contributing](docs/CONTRIBUTING.md)


## Quick Start

[Intro/Overview Video Tutorial](https://youtu.be/PWLPT320Ybo)  
[Site-to-Site Video Tutorial](https://youtu.be/krCKBJhwwDk)  

### Note about permissions
The default installation requires special privileges on the server side, because Netmaker will control the local kernel Wireguard. This can be turned off and run in non-privileged mode if necessary (but disables some features). For more details, see the **Usage** docs.

### Prereqs
 1. A running linux server to host Netmaker, with an IP reachable by your computers (Debian-based preferred but not required).
 2. Linux installed on the above server (Debian-based preferred but not required).
 3. Install Docker and Docker Compose if running in Docker Mode (see below).
 4. System dependencies installed:
	 - Docker (if running in default Docker mode. DO NOT use snap install for docker.)
	 - Docker Compose
	 - Wireguard + Resolvectl (if running in default Client mode)

#### CoreDNS Preparation
v0.3 introduces CoreDNS as a private nameserver. To run CoreDNS on your server host, you must disable systemd-resolved to open port 53: 
1. systemctl stop systemd-resolved
2. systemctl disable systemd-resolved
3. vim /etc/systemd/resolved.conf
	 - uncomment **DNS=** and add 8.8.8.8 or whatever is your preference
	 - uncomment **DNSStubListener=** and set to **"no"**
 4. sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf



### Launch Netmaker
Note, this installs Netmaker with CoreDNS and a Netclient (privileged).  If you want to run the server non-privileged or without CoreDNS, see the advanced usage docs. 

1. Clone this repo or just copy contents of "docker-compose.yml" to your Netmaker server (from prereqs).
2. In docker-compose.yml, change BACKEND_URL to the public IP of your server.
3. Run `sudo docker-compose up`
4. Navigate to your server's IP in the browser and you should see the Netmaker UI asking to create a new admin user.
5. Create a new admin user
6. You are now ready to begin using Netmaker. 

### Create a Network
You can also just use the "default" network.
1. Click "CREATE NETWORK" in the upper left of your console
2. Enter a valid address range, e.g. 10.11.12.0/24
3. Enter a name such as "homenet"
4. Additional options:
	- **Dual Stack**: Machines will recieve a private IPv6 address in addition to their IPv4 address.
	- **Local:** Will use local address range for endpoints instead of public. Use Case: Home or Office network where most devices do not have public IP's. In this case you can create a gateway into the network after creating the Local Network.

After Network creation, you can edit the network in the NETWORK DETAILS pane, modifying the address range and default options. You can also toggle on **Allow Node Signup Without Keys**, which makes the next step unnecessary, but allows anyone to create a node in your network, which will be cordoned in pending state.

### Create Keys
1. Click the "ACCESS KEYS" tab
2. Click "ADD NEW ACCESSS KEY"
3. Give your key a name and number of uses
4. Several values will be displayed. Save these somewhere, as they will only be displayed once:
	- **Access Key:** Use only in special edge cases where server connection string must be modified
	- **Access Token:** Use on machines that already have the netclient utility
	- **Install Command:** Use on machines that do not have the netclient utility

### Install Agent:
For machines **without** netclient, run the install command (from above): `curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.2/netclient-install.sh | KEY=<your access key> sh -`  
For machines **with** netclient run the following (with access token from above): `sudo netclient -c install -t <access token>`
For networks with **manual signup** enabled (see above), install using the network name: `sudo netclient -c install -n <network name>`

### Manage Nodes
Your machines should now be visible in the control pane. 
**Modify nodes:** Click the pencil icon in the NODES pane to modify details like WireGuard port, address, and node name. You can also **DELETE** nodes here and they will lose network access.
**Approve nodes:** If a node is in pending state (signed up without key), you can approve it. An icon will appear for pending nodes that need approval.

**Gateway Mode:** Click the Gateway icon to enable gateway mode on a given node. A popup will allow you to choose an existing network, or enter a custom address range.
*Example: You create a network in netmaker called Homenet. It has several machines on your home server. You create another network called Cloudnet. It has several machines in AWS. You have one server (server X) which is added to both networks. On Cloudnet, you make Server X a gateway to Homenet. Now, the cloudnet machines have access to your homenet machines. via  Server X.*

*On Homenet, you add Server Y, a machine in AWS, and make it a gateway to a custom address range 172.16.0.0/16. The machines on your home network now have access to any AWS machines in that address range via Server Y*

### Manage DNS
On the DNS tab you can create custom DNS entries for a given network.

 1. All dns entries will be *postfixed* with a private TLD of the network name, for example, ".mynet"
 2. Default DNS is created for node name + TLD, for instance, node-c42wt.mynet. This is not editable.
 3. Click ADD ENTRY to add custom DNS
	 - You can click CHOOSE NODE to direct DNS to a specific node in the network
	 - You can also specify any custom address you would like, which can be outside the network (for instance, the IP for google.com)
	 - Add a dns entry name, which will be postfixed with the network TLD. E.g. if you enter "privateapi.com", it will become "privateapi.com.networkname" 

### Uninstalling Client
To uninstall the client from a network: `sudo netclient -c remove -n < networkname >`
To uninstall entirely, run the above for each network,  and then run `sudo rm -rf /etc/netclient`

### Uninstralling Netmaker
To uninstall the netmaker server, simply run `docker-compose down`

#### LICENSE

Netmaker's source code and all artifacts in this repository are freely available. All versions are published under the Server Side Public License (SSPL), version 1, which can be found here: [LICENSE.txt](./LICENSE.txt).

#### CONTACT

Email: alex@gravitl.com  
Discord: https://discord.gg/zRb9Vfhk8A
