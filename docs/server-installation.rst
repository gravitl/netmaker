=================================
Advanced Server Installation
=================================

This section outlines installing the Netmaker server, including Netmaker, Netmaker UI, rqlite, and CoreDNS

System Compatibility
====================

Netmaker will require elevated privileges to perform network operations. Netmaker has similar limitations to :doc:`netclient <./client-installation>` (client networking agent). 

Typically, Netmaker is run inside of containers (Docker). To run a non-docker installation, you must run the Netmaker binary, CoreDNS binary, database, and a web server directly on the host. Each of these components have their own individual requirements.

The quick install guide is recommended for first-time installs. 

The following documents are meant for special cases like Kubernetes and LXC, or for more advanced setups. 


Server Configuration Reference
==========================================

Netmaker sets its configuration in the following order of precendence:

1. Defaults
2. Config File
3. Environment Variables

Variable Description
----------------------
VERBOSITY:
    **Default:** 0

    **Description:** Specify level of logging you would like on the server. Goes up to 3 for debugging.


GRPC_SSL:
    **Default:** "off"

    **Description:** Specifies if GRPC is going over secure GRPC or SSL. This is a setting for the clients and is passed through the access token. Can be set to "on" and "off". Set to on if SSL is configured for GRPC.

SERVER_API_CONN_STRING
    **Default:** ""

    **Description:**  Allows specification of the string used to connect to the server api. Format: IP:PORT or DOMAIN:PORT. Defaults to SERVER_HOST if not specified.

SERVER_GRPC_CONN_STRING
    **Default:** ""

    **Description:**  Allows specification of the string used to connect to grpc. Format: IP:PORT or DOMAIN:PORT. Defaults to SERVER_HOST if not specified.

SERVER_HOST: *(depreciated, use SERVER_API_CONN_STRING and SERVER_GRPC_CONN_STRING)* 
    **Default:** Server will perform an IP check and set automatically unless explicitly set, or DISABLE_REMOTE_IP_CHECK is set to true, in which case it defaults to 127.0.0.1

    **Description:** Sets the SERVER_HTTP_HOST and SERVER_GRPC_HOST variables if they are unset. The address where traffic comes in. 

SERVER_HTTP_HOST: *(depreciated, use SERVER_API_CONN_STRING and SERVER_GRPC_CONN_STRING)*
    **Default:** Equals SERVER_HOST if set, "127.0.0.1" if SERVER_HOST is unset.
    
    **Description:** Set to make the HTTP and GRPC functions available via different interfaces/networks.

SERVER_GRPC_HOST: *(depreciated, use SERVER_API_CONN_STRING and SERVER_GRPC_CONN_STRING)*
    **Default:** Equals SERVER_HOST if set, "127.0.0.1" if SERVER_HOST is unset.

    **Description:** Set to make the HTTP and GRPC functions available via different interfaces/networks.

API_PORT:
    **Default:** 8081 

    **Description:** The HTTP API port for Netmaker. Used for API calls / communication from front end.

GRPC_PORT:  
    **Default:** 50051

    **Description:** The GRPC port for Netmaker. Used for communications from nodes.

MASTER_KEY:  
    **Default:** "secretkey" 

    **Description:** The admin master key for accessing the API. Change this in any production installation.

CORS_ALLOWED_ORIGIN:  
    **Default:** "*"

    **Description:** The "allowed origin" for API requests. Change to restrict where API requests can come from.

REST_BACKEND:  
    **Default:** "on" 

    **Description:** Enables the REST backend (API running on API_PORT at SERVER_HTTP_HOST). Change to "off" to turn off.

AGENT_BACKEND:  
    **Default:** "on" 

    **Description:** Enables the AGENT backend (GRPC running on GRPC_PORT at SERVER_GRPC_HOST). Change to "off" to turn off.

DNS_MODE:  
    **Default:** "off"

    **Description:** Enables DNS Mode, meaning config files will be generated for CoreDNS.

DATABASE:  
    **Default:** "sqlite"

    **Description:** Specify db type to connect with. Currently, options include "sqlite", "rqlite", and "postgres".

SQL_CONN:
    **Default:** "http://"

    **Description:** Specify the necessary string to connect with your local or remote sql database.

SQL_HOST:
    **Default:** "localhost"

    **Description:** Host where postgres is running.

SQL_PORT:
    **Default:** "5432"

    **Description:** port postgres is running.

SQL_DB:
    **Default:** "netmaker"

    **Description:** DB to use in postgres.

SQL_USER:
    **Default:** "postgres"

    **Description:** User for posgres.

SQL_PASS:
    **Default:** "nopass"

    **Description:** Password for postgres.

CLIENT_MODE:  
    **Default:** "on"

    **Description:** Specifies if server should deploy itself as a node (client) in each network. May be turned to "off" for more restricted servers.

Config File Reference
----------------------
A config file may be placed under config/environments/<env-name>.yml. To read this file at runtime, provide the environment variable NETMAKER_ENV at runtime. For instance, dev.yml paired with ENV=dev. Netmaker will load the specified Config file. This allows you to store and manage configurations for different environments. Below is a reference Config File you may use.

.. literalinclude:: ../config/environments/dev.yaml
  :language: YAML

Compose File - Annotated
--------------------------------------

All environment variables and options are enabled in this file. It is the equivalent to running the "full install" from the above section. However, all environment variables are included, and are set to the default values provided by Netmaker (if the environment variable was left unset, it would not change the installation). Comments are added to each option to show how you might use it to modify your installation.

.. literalinclude:: ../compose/docker-compose.reference.yml
  :language: YAML


DNS Mode Setup
====================================

If you plan on running the server in DNS Mode, know that a `CoreDNS Server <https://coredns.io/manual/toc/>`_ will be installed. CoreDNS is a light-weight, fast, and easy-to-configure DNS server. It is recommended to bind CoreDNS to port 53 of the host system, and it will do so by default. The clients will expect the nameserver to be on port 53, and many systems have issues resolving a different port.

However, on your host system (for Netmaker), this may conflict with an existing process. On linux systems running systemd-resolved, there is likely a service consuming port 53. The below steps will disable systemd-resolved, and replace it with a generic (e.g. Google) nameserver. Be warned that this may have consequences for any existing private DNS configuration. 

With the latest docker-compose, it is not necessary to perform these steps. But if you are running the install and find that port 53 is blocked, you can perform the following steps, which were tested on Ubuntu 20.04 (these should be run prior to deploying the docker containers).

.. code-block::

  systemctl stop systemd-resolved
  systemctl disable systemd-resolved 
  vim /etc/systemd/resolved.conf
    *  uncomment DNS and add 8.8.8.8 or whatever reachable nameserver is your preference  *
    *  uncomment DNSStubListener and set to "no"  *
  ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf

Port 53 should now be available for CoreDNS to use.


Docker Compose Install
=======================

The most simple (and recommended) way of installing Netmaker is to use one of the provided `Docker Compose files <https://github.com/gravitl/netmaker/tree/master/compose>`_. Below are instructions for several different options to install Netmaker via Docker Compose, followed by an annotated reference Docker Compose in case your use case requires additional customization.

Test Install - No DNS, No Secure GRPC
--------------------------------------------------------

This install will run Netmaker on a server without HTTPS using an IP address. This is not secure and not recommended, but can be helpful for testing.

It also does not run the CoreDNS server, to simplify the deployment

**Prerequisites:**
  * server ports 80, 8081, and 50051 are not blocked by firewall

**Notes:** 
  * You can change the port mappings in the Docker Compose if the listed ports are already in use.

Assuming you have Docker and Docker Compose installed, you can just run the following, replacing **< Insert your-host IP Address Here >** with your host IP (or domain):

.. code-block::

  wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.test.yml
  sed -i ‘s/HOST_IP/< Insert your-host IP Address Here >/g’ docker-compose.yml
  docker-compose up -d`

Traefik Proxy
------------------------

To install with Traefik, rather than Nginx or the default Caddy, check out this repo: https://github.com/bsherman/netmaker-traefik 


No DNS - CoreDNS Disabled
----------------------------------------------

DNS Mode is currently limited to clients that can run resolvectl (systemd-resolved, see :doc:`Architecture docs <./architecture>` for more info). You may wish to disable DNS mode for various reasons. This installation option gives you the full feature set minus CoreDNS.

To run without DNS, follow the :doc:`Quick Install <./quick-start>` guide, omitting the steps for DNS setup. In addition, when the guide has you pull (wget) the Netmaker docker-compose template, use the following link instead:

#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.nodns.yml``

This template is equivalent but omits CoreDNS.


.. _NoDocker:

Linux Install without Docker
=============================

Most systems support Docker, but some do not. In such environments, there are many options for installing Netmaker. Netmaker is available as a binary file, and there is a zip file of the Netmaker UI static HTML on GitHub. Beyond the UI and Server, you need to install MongoDB and CoreDNS (optional). 

To start, we recommend following the Nginx instructions in the :doc:`Quick Install <./quick-start>` guide to enable SSL for your environment.

Once this is enabled and configured for a domain, you can continue with the below. The recommended server runs Ubuntu 20.04.

rqlite Setup
----------------
1. Install rqlite on your server: https://github.com/rqlite/rqlite

2. Run rqlite: rqlited -node-id 1 ~/node.1

Server Setup
-------------
1. **Run the install script:** 

``sudo curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/netmaker-server.sh | sh -``

2. Check status:  ``sudo journalctl -u netmaker``
3. If any settings are incorrect such as host or mongo credentials, change them under /etc/netmaker/config/environments/< your env >.yaml and then run ``sudo systemctl restart netmaker``

UI Setup
-----------

The following uses Nginx as an http server. You may alternatively use Apache or any other web server that serves static web files.

1. Download and Unzip UI asset files
2. Copy Config to Nginx
3. Modify Default Config Path
4. Change Backend URL
5. Start Nginx

.. code-block::
  
  sudo wget -O /usr/share/nginx/html/netmaker-ui.zip https://github.com/gravitl/netmaker-ui/releases/download/latest/netmaker-ui.zip
  sudo unzip /usr/share/nginx/html/netmaker-ui.zip -d /usr/share/nginx/html
  sudo cp /usr/share/nginx/html/nginx.conf /etc/nginx/conf.d/default.conf
  sudo sed -i 's/root \/var\/www\/html/root \/usr\/share\/nginx\/html/g' /etc/nginx/sites-available/default
  sudo sh -c 'BACKEND_URL=http://<YOUR BACKEND API URL>:PORT /usr/share/nginx/html/generate_config_js.sh >/usr/share/nginx/html/config.js'
  sudo systemctl start nginx

CoreDNS Setup
----------------

.. _KubeInstall:

Kubernetes Install
=======================

Server Install
--------------------------

This template assumes your cluster uses Nginx for ingress with valid wildcard certificates. If using an ingress controller other than Nginx (ex: Traefik), you will need to manually modify the Ingress entries in this template to match your environment.

This template also requires RWX storage. Please change references to storageClassName in this template to your cluster's Storage Class.

``wget https://raw.githubusercontent.com/gravitl/netmaker/master/kube/netmaker-template.yaml``

Replace the NETMAKER_BASE_DOMAIN references to the base domain you would like for your Netmaker services (ui,api,grpc). Typically this will be something like **netmaker.yourwildcard.com**.

``sed -i ‘s/NETMAKER_BASE_DOMAIN/<your base domain>/g’ netmaker-template.yaml``

Now, assuming Ingress and Storage match correctly with your cluster configuration, you can install Netmaker.

.. code-block::

  kubectl create ns nm
  kubectl config set-context --current --namespace=nm
  kubectl apply -f netmaker-template.yaml -n nm

In about 3 minutes, everything should be up and running:

``kubectl get ingress nm-ui-ingress-nginx``

Netclient Daemonset
--------------------------

The following instructions assume you have Netmaker running and a network you would like to add your cluster into. The Netmaker server does not need to be running inside of a cluster for this.

.. code-block::

  wget https://raw.githubusercontent.com/gravitl/netmaker/master/kube/netclient-template.yaml
  sed -i ‘s/ACCESS_TOKEN_VALUE/< your access token value>/g’ netclient-template.yaml
  kubectl apply -f netclient-template.yaml

For a more detailed guide on integrating Netmaker with MicroK8s, `check out this guide <https://itnext.io/how-to-deploy-a-cross-cloud-kubernetes-cluster-with-built-in-disaster-recovery-bbce27fcc9d7>`_. 

Nginx Reverse Proxy Setup with https
======================================

The `Swag Proxy <https://github.com/linuxserver/docker-swag>`_ makes it easy to generate a valid ssl certificate for the config bellow. Here is the `documentation <https://docs.linuxserver.io/general/swag>`_ for the installation.

The following file configures Netmaker as a subdomain. This config is an adaption from the swag proxy project.

./netmaker.subdomain.conf:

.. code-block:: nginx

    server {
        listen 443 ssl;
        listen [::]:443 ssl;

        server_name netmaker.*; # The external URL
        client_max_body_size 0;

        # A valid https certificate is needed.
        include /config/nginx/ssl.conf;

        location / {
            # This config file can be found at:
            # https://github.com/linuxserver/docker-swag/blob/master/root/defaults/proxy.conf
            include /config/nginx/proxy.conf;

            # if you use a custom resolver to find your app, needed with swag proxy
            # resolver 127.0.0.11 valid=30s;
            set $upstream_app netmaker-ui;                             # The internal URL
            set $upstream_port 80;                                     # The internal Port
            set $upstream_proto http;                                  # the protocol that is being used
            proxy_pass $upstream_proto://$upstream_app:$upstream_port; # combine the set variables from above
            }
        }

    server {
        listen 443 ssl;
        listen [::]:443 ssl;

        server_name backend-netmaker.*; # The external URL
        client_max_body_size 0;
        underscores_in_headers on;

        # A valid https certificate is needed.
        include /config/nginx/ssl.conf;

        location / {
            # if you use a custom resolver to find your app, needed with swag proxy
            # resolver 127.0.0.11 valid=30s;

            set $upstream_app netmaker;                                # The internal URL
            set $upstream_port 8081;                                   # The internal Port
            set $upstream_proto http;                                  # the protocol that is being used
            proxy_pass $upstream_proto://$upstream_app:$upstream_port; # combine the set variables from above

            # Forces the header to be the one that is visible from the outside
            proxy_set_header                Host backend.netmaker.example.org; # Please cange to your URL

            # Pass all headers through to the backend
            proxy_pass_request_headers      on;
            }
        }

.. _HAInstall:



Highly Available Installation (Kubernetes)
==================================================

Netmaker comes with a Helm chart to deploy with High Availability on Kubernetes:

.. code-block::

    helm repo add netmaker https://gravitl.github.io/netmaker-helm/
    helm repo update

Requirements
---------------

To run HA Netmaker on Kubernetes, your cluster must have the following:
- RWO and RWX Storage Classes (RWX is only required if running Netmaker with DNS Management enabled).
- An Ingress Controller and valid TLS certificates 
- This chart can currently generate ingress for Nginx or Traefik Ingress with LetsEncrypt + Cert Manager
- If LetsEncrypt and CertManager are not deployed, you must manually configure certificates for your ingress

Furthermore, the chart will by default install and use a postgresql cluster as its datastore.

Recommended Settings:
----------------------
A minimal HA install of Netmaker can be run with the following command:
`helm install netmaker --generate-name --set baseDomain=nm.example.com`
This install has some notable exceptions:
- Ingress **must** be manually configured post-install (need to create valid Ingress with TLS)
- Server will use "userspace" WireGuard, which is slower than kernel WG
- DNS will be disabled

Example Installations:
------------------------
An annotated install command:

.. code-block::

    helm install netmaker/netmaker --generate-name \ # generate a random id for the deploy 
    --set baseDomain=nm.example.com \ # the base wildcard domain to use for the netmaker api/dashboard/grpc ingress 
    --set replicas=3 \ # number of server replicas to deploy (3 by default) 
    --set ingress.enabled=true \ # deploy ingress automatically (requires nginx or traefik and cert-manager + letsencrypt) 
    --set ingress.className=nginx \ # ingress class to use 
    --set ingress.tls.issuerName=letsencrypt-prod \ # LetsEncrypt certificate issuer to use 
    --set dns.enabled=true \ # deploy and enable private DNS management with CoreDNS 
    --set dns.clusterIP=10.245.75.75 --set dns.RWX.storageClassName=nfs \ # required fields for DNS 
    --set postgresql-ha.postgresql.replicaCount=2 \ # number of DB replicas to deploy (default 2)


The below command will install netmaker with two server replicas, a coredns server, and ingress with routes of api.nm.example.com, grpc.nm.example.com, and dashboard.nm.example.com. CoreDNS will be reachable at 10.245.75.75, and will use NFS to share a volume with Netmaker (to configure dns entries).

.. code-block::

    helm install netmaker/netmaker --generate-name --set baseDomain=nm.example.com \
    --set replicas=2 --set ingress.enabled=true --set dns.enabled=true \
    --set dns.clusterIP=10.245.75.75 --set dns.RWX.storageClassName=nfs \
    --set ingress.className=nginx

The below command will install netmaker with three server replicas (the default), **no coredns**, and ingress with routes of api.netmaker.example.com, grpc.netmaker.example.com, and dashboard.netmaker.example.com. There will be one UI replica instead of two, and one database instance instead of two. Traefik will look for a ClusterIssuer named "le-prod-2" to get valid certificates for the ingress. 

.. code-block::

    helm3 install netmaker/netmaker --generate-name \
    --set baseDomain=netmaker.example.com --set postgresql-ha.postgresql.replicaCount=1 \
    --set ui.replicas=1 --set ingress.enabled=true \
    --set ingress.tls.issuerName=le-prod-2 --set ingress.className=traefik

Below, we discuss the considerations for Ingress, Kernel WireGuard, and DNS.

Ingress	
----------
To run HA Netmaker, you must have ingress installed and enabled on your cluster with valid TLS certificates (not self-signed). If you are running Nginx as your Ingress Controller and LetsEncrypt for TLS certificate management, you can run the helm install with the following settings:

- `--set ingress.enabled=true`
- `--set ingress.annotations.cert-manager.io/cluster-issuer=<your LE issuer name>`

If you are not using Nginx or Traefik and LetsEncrypt, we recommend leaving ingress.enabled=false (default), and then manually creating the ingress objects post-install. You will need three ingress objects with TLS:

- `dashboard.<baseDomain>`
- `api.<baseDomain>`
- `grpc.<baseDomain>`

If deploying manually, the gRPC ingress object requires special considerations. Look up the proper way to route grpc with your ingress controller. For instance, on Traefik, an IngressRouteTCP object is required.

There are some example ingress objects in the kube/example folder.

Kernel WireGuard
------------------
If you have control of the Kubernetes worker node servers, we recommend **first** installing WireGuard on the hosts, and then installing HA Netmaker in Kernel mode. By default, Netmaker will install with userspace WireGuard (wireguard-go) for maximum compatibility, and to avoid needing permissions at the host level. If you have installed WireGuard on your hosts, you should install Netmaker's helm chart with the following option:

- `--set wireguard.kernel=true`

DNS
----------
By Default, the helm chart will deploy without DNS enabled. To enable DNS, specify with:

- `--set dns.enabled=true` 

This will require specifying a RWX storage class, e.g.:

- `--set dns.RWX.storageClassName=nfs`

This will also require specifying a service address for DNS. Choose a valid ipv4 address from the service IP CIDR for your cluster, e.g.:

- `--set dns.clusterIP=10.245.69.69`

**This address will only be reachable from hosts that have access to the cluster service CIDR.** It is only designed for use cases related to k8s. If you want a more general-use Netmaker server on Kubernetes for use cases outside of k8s, you will need to do one of the following:
- bind the CoreDNS service to port 53 on one of your worker nodes and set the COREDNS_ADDRESS equal to the public IP of the worker node
- Create a private Network with Netmaker and set the COREDNS_ADDRESS equal to the private address of the host running CoreDNS. For this, CoreDNS will need a node selector and will ideally run on the same host as one of the Netmaker server instances.

Values
---------

To view all options for the chart, please visit the README in the code repo `here <https://github.com/gravitl/netmaker/tree/master/kube/helm#values>`_ .

Highly Available Installation (VMs/Bare Metal)
==================================================

For an enterprise Netmaker installation, you will need a server that is highly available, to ensure redundant WireGuard routing when any server goes down. To do this, you will need:

1. A load balancer
2. 3+ Netmaker server instances
3. rqlite or PostgreSQL as the backing database

These documents outline general HA installation guidelines. Netmaker is highly customizable to meet a wide range of enterprise environments. If you would like support with an enterprise-grade Netmaker installation, you can `schedule a consultation here <https://gravitl.com/book>`_ . 

The main consideration for this document is how to configure rqlite. Most other settings and procedures match the standardized way of making applications HA: Load balancing to multiple instances, and sharing a DB. In our case, the DB (rqlite) is distributed, making HA data more easily achievable.

If using PostgreSQL, follow their documentation for `installing in HA mode <https://www.postgresql.org/docs/14/high-availability.html>`_ and skip step #2.

1. Load Balancer Setup
------------------------

Your load balancer of choice will send requests to the Netmaker servers. Setup is similar to the various guides we have created for Nginx, Caddy, and Traefik. SSL certificates must also be configured and handled by the LB.

2. RQLite Setup
------------------

RQLite is the included distributed datastore for an HA Netmaker installation. If you have a different corporate database you wish to integrate, Netmaker is easily extended to other DB's. If this is a requirement, please contact us.

Assuming you use Rqlite, you must run it on each Netmaker server VM, or alongside that VM as a container. Setup a config.json for database credentials (password supports BCRYPT HASHING) and mount in working directory of rqlite and specify with `-auth config.json` :

.. code-block::

    [{
        "username": "netmaker",
        "password": "<YOUR_DB_PASSWORD>",
        "perms": ["all"]
    }]


Once your servers are set up with rqlite, the first instance must be started normally, and then additional nodes must be added with the "join" command. For instance, here is the first server node:

.. code-block::

    sudo docker run -d -p 4001:4001 -p 4002:4002 rqlite/rqlite -node-id 1 -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -http-adv-addr 1.2.3.4:4001 -raft-adv-addr 1.2.3.4:4002 -auth config.json

And here is a joining node:

.. code-block::

    sudo docker run -d -p 4001:4001 -p 4002:4002 rqlite/rqlite -node-id 2 -http-addr 0.0.0.0:4001 -raft-addr 0.0.0.0:4002 -http-adv-addr 2.3.4.5:4001  -raft-adv-addr 2.3.4.5:4002 -join https://netmaker:<YOUR_DB_PASSWORD>@1.2.3.4:4001

- reference for rqlite setup: https://github.com/rqlite/rqlite/blob/master/DOC/CLUSTER_MGMT.md#creating-a-cluster
- reference for rqlite security: https://github.com/rqlite/rqlite/blob/master/DOC/SECURITY.md

Once rqlite instances have been configured, the Netmaker servers can be deployed.

3. Netmaker Setup
------------------

Netmaker will be started on each node with default settings, except with DATABASE=rqlite (or DATABASE=postgress) and SQL_CONN set appropriately to reach the local rqlite instance. Rqlite will maintain consistency with each Netmaker backend.

If deploying HA with PostgreSQL, you will connect with the following settings:

.. code-block::

    SQL_HOST = <sql host>
    SQL_PORT = <port>
    SQL_DB   = <designated sql DB>
    SQL_USER = <your user>
    SQL_PASS = <your password>
    DATABASE = postgres


4. Other Considerations
------------------------

This is enough to get a functioning HA installation of Netmaker. However, you may also want to make the Netmaker UI or the CoreDNS server HA as well. The Netmaker UI can simply be added to the same servers and load balanced appropriately. For some load balancers, you may be able to do this with CoreDNS as well.




