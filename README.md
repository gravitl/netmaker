<p align="center">
  <img src="netmaker.png"><break/>
</p>
<p align="center">
<i>Connect any computers together over a secure, fast, private network, and manage multiple networks from a central server.</i> 
</p>

## What is Netmaker?
Netmaker is a tool for creating and managing virtual networks. The goal is to provide functionality similar to Tailscale, ZeroTier, and Nebula, but faster, easier, and more dynamic. It should be like clicking a button. Netmaker consists of a server, an agent, and a UI. You spin up the Netmaker server and then install netclient (the agent) on your computers. Netmaker will do the rest. It will tell all of your computers how to reach each other and will keep them informed of any changes to the network.

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

Netmaker works on most linux systems that have systemd. It works with Fedora, Ubuntu, and Raspian. Just make sure you have WireGuard installed. Having a problem? Open an issue or Contact us.

In future releases, we have plans to support other platforms such as Windows and MacOS. 


## Docs
**For more information, please read the docs, or check out the Quick Start below:**

 - [General Usage](docs/USAGE.md)
 - [Troubleshooting](docs/TROUBLESHOOTING.md)
 - [API Documentation](docs/API.md)
 - [Product Roadmap](docs/ROADMAP.md)
 - [Contributing](docs/CONTRIBUTING.md)


## Quick Start

[Video Tutorial](https://youtu.be/PWLPT320Ybo)

#### Prereqs:
1. A server with an IP reachable by your computers (a small ec2 instance or droplet would do just fine).
2. Linux installed on the above server (we use Ubuntu, but anything that runs Docker should work).
3. Install Docker if running in Docker Mode (see below).


#### Launch Netmaker:

Netmaker v0.2 introduces the server as a 'client'. This means the server can add itself into networks if you would like. To do so, Netmaker requires privileged access where it is running, and needs to modify the host filesystem. To run in this mode, we are not currently using Docker.

**If you would like to run with "client mode", you can use the following script to deploy:**
`sudo curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.2/netmaker-install-clientmode.sh | sudo SERVER_DOMAIN=< your server IP > sh -`

**If you would like to run without "client mode", and manually add/remove the server from networks:**

1. Clone this repo or just copy contents of "docker-compose.yml" to your Netmaker server (from prereqs).
2. In docker-compose.yml, change BACKEND_URL to the public IP of your server.
3. Run `sudo docker-compose up`
4. Navigate to your server's IP in the browser and you should see the Netmaker UI asking to create a new admin user.
5. Create a new admin user
6. You are now ready to begin using Netmaker. There should be a default network you can use or you can create your own. Then, Create a key or enable manual node sign up so that your nodes can connect.

#### On your machines :
A command will be displayed when you generate a token for signing up nodes. Run it on each machine you would like to connect.
`curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.2/netclient-install.sh | KEY=<your access key> sh -`  
(Note: Key can be left out if manual node signup is enabled)

#### LICENSE

Netmaker's source code and all artifacts in this repository are freely available. All versions are published under the Server Side Public License (SSPL), version 1, which can be found here: [LICENSE.txt](./LICENSE.txt).

#### CONTACT

Email: info@gravitl.com  
Discord: https://discord.gg/zRb9Vfhk8A
