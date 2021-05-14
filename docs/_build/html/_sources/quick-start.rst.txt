===========
Quick Start
===========

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
3. Run `sudo docker-compose up -d`
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
For machines **without** netclient, run the install command (from above): `curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.3/netclient-install.sh | KEY=<your access key> sh -`  
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
