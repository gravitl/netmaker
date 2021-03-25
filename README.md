
<p align="center">
  <img src="netmaker.png"><break/>
</p>
<p align="center">
<i>Connect any computers together over a secure, fast, private network, and manage multiple networks from a central server.</i> 
</p>

## What is Netmaker?
Netmaker lets you easily create secure virtual networks: Just spin up a Netmaker server and install the agent on your computers. Netmaker relies on WireGuard to create encrypted tunnels between every node in your virtual network, creating a mesh overlay. Netmaker takes the work  out of manually configuring machines and updating them every time something changes in your network. The agents are self-updating and pull necessary changes  from the server. 

Netmaker also has a handy dandy UI, which you can find in [this repo](https://github.com/falconcat-inc/WireCat-UI). We recommend deploying the UI alongside the server to make the experience easier and better.

## Why Netmaker?
 1. Create a flat, secure network between multiple/hybrid cloud environments
 2. Integrate central and edge services + IoT
 3. Secure an office or home network while providing remote connectivity
 4. Manage cryptocurrency proof-of-stake machines
 5. Provide an additional layer of security on an existing network
 6. Encrypt Kubernetes inter-node communications
 7. Secure site-to-site connections

<p align="center">
  <img src="mesh-diagram.png">
</p>

## Docs
**For more information, please read the docs, or check out the Quick Start below:**

 - [Getting Started](docs/GETTING_STARTED.md)
 - [API Documentation](docs/API.md)
 - [Product Roadmap](docs/ROADMAP.md)
 - [Contributing](docs/CONTRIBUTING.md)


## Quick Start

Setup Docker (Prereq):
1. Create an access token on github with artifact access.
2. On your VPS, create a file in the home dir called TOKEN.txt with the value of your token inside.
3. `cat ~/TOKEN.txt | sudo docker login https://docker.pkg.github.com -u GITHUB_USERNAME --password-stdin`

Setup Server:
1. Clone this repo or just copy contents of "docker-compose.yml" to a machine with a public IP.
2. In docker-compose.yml, change BACKEND_URL to the public IP ofthat machine.
3. Run `sudo docker-compose up`
4. Navigate to your IP and you should see the WireCat UI asking for a new admin user (if not or if it takes you straight to login screen without asking for user creation, investigate the error).
5. Create the admin user
6. Click "Create Group" and fill out the details (group == network)
7. You are now ready to begin using WireCat. Create a key or "allow manual node sign up."

Run on each machine in network:
1. Get the binary: `sudo wget 52.55.6.84:8081/meshclient/files/meshclient`
2. Make it executable: `sudo chmod +x meshclient`
3. Run the install command: `sudo ./meshclient -c install -g <group name> -s <server:port> -k <key value>`


#### LICENSE

Netmaker's source code and all artifacts in this repository are freely available. All versions are published under the Server Side Public License (SSPL), version 1, which can be found under the "licensing" directory: [LICENSE.txt](licensing/LICENSE.txt).
