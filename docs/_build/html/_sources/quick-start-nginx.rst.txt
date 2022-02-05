==================================
Install with Nginx (depreciated)
==================================

This is the old quick start guide, which contains instructions using Nginx and Docker CE. It is recommended to use the new quick start guide with Caddy instead.

0. Introduction
==================

We assume for this installation that you want all of the Netmaker features enabled, you want your server to be secure, and you want your server to be accessible from anywhere.

This instance will not be HA. However, it should comfortably handle around one hundred concurrent clients and support the most common use cases.

If you are deploying for a business or enterprise use case and this setup will not fit your needs, please contact info@gravitl.com, or check out the business subscription plans at https://gravitl.com/plans/business.

By the end of this guide, you will have Netmaker installed on a public VM linked to your custom domain, secured behind an Nginx reverse proxy.

For information about deploying more advanced configurations, see the :doc:`Advanced Installation <./server-installation>` docs. 


1. Prerequisites
==================
-  **Virtual Machine**
   
   - Preferably from a cloud provider (e.x: DigitalOcean, Linode, AWS, GCP, etc.)
      - We do not recommend Oracle Cloud, as VM's here have been known to cause network interference.
   - Public, static IP 
   - Min 1GB RAM, 1 CPU (4GB RAM, 2CPU preferred)
      - Nginx may have performance issues if using a cloud VPS with a single, shared CPU
   - 2GB+ of storage 
   - Ubuntu  20.04 Installed

- **Domain**

  - A publicly owned domain (e.x. example.com, mysite.biz) 
  - Permission and access to modify DNS records via DNS service (e.x: Route53)

2. Install Dependencies
========================

``ssh root@your-host``

Install Docker
---------------
Begin by installing the community version of Docker and docker-compose (there are issues with the snap version). You can follow the official `Docker instructions here <https://docs.docker.com/engine/install/>`_. Or, you can use the below series of commands which should work on Ubuntu 20.04.

.. code-block::

  sudo apt-get remove docker docker-engine docker.io containerd runc
  sudo apt-get update
  sudo apt-get -y install apt-transport-https ca-certificates curl gnupg lsb-release
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg  
  echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt-get update
  sudo apt-get -y install docker-ce docker-ce-cli containerd.io
  sudo curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
  sudo chmod +x /usr/local/bin/docker-compose
  docker --version
  docker-compose --version

At this point Docker should be installed.

Install Dependencies
-----------------------------

In addition to Docker, this installation requires WireGuard, Nginx, and Certbot.

``sudo apt -y install wireguard wireguard-tools nginx certbot python3-certbot-nginx net-tools``

 
3. Prepare VM
===============================

Prepare Domain
----------------------------
1. Choose a base domain or subdomain for Netmaker. If you own **example.com**, this should be something like **netmaker.example.com**

- You must point your wildcard domain to the public IP of your VM, e.x: *.example.com --> <your public ip>

2. Add an A record pointing to your VM using your DNS service provider for *.netmaker.example.com (inserting your own subdomain of course).
3. Netmaker will create three subdomains on top of this. For the example above those subdomains would be:

- dashboard.netmaker.example.com

- api.netmaker.example.com

- grpc.netmaker.example.com

Moving forward we will refer to your base domain using **<your base domain>**. Replace these references with your domain (e.g. netmaker.example.com).

4. ``nslookup host.<your base domain>`` (inserting your domain) should now return the IP of your VM.

5. Generate SSL Certificates using certbot:

``sudo certbot certonly --manual --preferred-challenges=dns --email your@email.com --server https://acme-v02.api.letsencrypt.org/directory --agree-tos --manual-public-ip-logging-ok -d "*.<your base domain>"``

The above command (using your domain instead of <your base domain>), will prompt you to enter a TXT record in your DNS service provider. Do this, and **wait one  minute** before clicking enter, or it may fail and you will have to run the command again.

Prepare Firewall
-----------------

Make sure firewall settings are appropriate for Netmaker. You need ports 53 and 443. On the server you can run:


.. code-block::

  sudo ufw allow proto tcp from any to any port 443 && sudo ufw allow 53/udp && sudo ufw allow 53/tcp

**Based on your cloud provider, you may also need to set inbound security rules for your server. This will be dependent on your cloud provider. Be sure to check before moving on:**
  - allow 443/tcp from all
  - allow 53/udp and 53/tcp from all

In addition to the above ports, you will need to make sure that your cloud's firewall or security groups are opened for the range of ports that Netmaker's WireGuard interfaces consume.

Netmaker will create one interface per network, starting from 51821. So, if you plan on having 5 networks, you will want to have at least 51821-51825 open (udp).

Prepare Nginx
-----------------

Nginx will serve the SSL certificate with your chosen domain and forward traffic to netmaker.

Get the nginx configuration file:

``wget https://raw.githubusercontent.com/gravitl/netmaker/master/nginx/netmaker-nginx-template.conf``

Insert your domain in the configuration file and add to nginx:

.. code-block::

  sed -i 's/NETMAKER_BASE_DOMAIN/<your base domain>/g' netmaker-nginx-template.conf
  sudo cp netmaker-nginx-template.conf /etc/nginx/conf.d/<your base domain>.conf
  nginx -t && nginx -s reload
  systemctl restart nginx

4. Install Netmaker
====================

Prepare Templates
------------------

**Note on COREDNS_IP:** Depending on your cloud provider, the public IP may not be bound directly to the VM on which you are running. In such cases, CoreDNS cannot bind to this IP, and you should use the IP of the default interface on your machine in place of COREDNS_IP. If the public IP **is** bound to the VM, you can simply use the same IP as SERVER_PUBLIC_IP.

.. code-block::

  wget https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.yml
  sed -i 's/NETMAKER_BASE_DOMAIN/<your base domain>/g' docker-compose.yml
  sed -i 's/SERVER_PUBLIC_IP/<your server ip>/g' docker-compose.yml
  sed -i 's/COREDNS_IP/<your server ip>/g' docker-compose.yml

Generate a unique master key and insert it:

.. code-block::

  tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo ''
  sed -i 's/REPLACE_MASTER_KEY/<your generated key>/g' docker-compose.yml

You may want to save this key for future use with the API.

Start Netmaker
----------------

``sudo docker-compose -f docker-compose.yml up -d``

navigate to dashboard.<your base domain> to log into the UI.

To troubleshoot issues, start with:

``docker logs netmaker``

Or check out the :doc:`troubleshoooting docs <./troubleshoot>`.
