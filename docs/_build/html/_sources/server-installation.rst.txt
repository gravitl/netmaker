=================================
Advanced Server Installation
=================================

This section outlines installing the Netmaker server, including Netmaker, Netmaker UI, rqlite, and CoreDNS

System Compatibility
====================

Netmaker will require elevated privileges to perform network operations. Netmaker has similar limitations to :doc:`netclient <./client-installation>` (client networking agent). 

Typically, Netmaker is run inside of containers (Docker). To run a non-docker installation, you must run the Netmaker binary, CoreDNS binary, rqlite, and a web server directly on the host. Each of these components have their own individual requirements.

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

However, on your host system (for Netmaker), this may conflict with an existing process. On linux systems running systemd-resolved, there is likely a service consuming port 53. The below steps will disable systemd-resolved, and replace it with a generic (e.g. Google) nameserver. Be warned that this may have consequences for any existing private DNS configuration. The following was tested on Ubuntu 20.04 and should be run prior to deploying the docker containers.

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


No DNS - CoreDNS Disabled
----------------------------------------------

DNS Mode is currently limited to clients that can run resolvectl (systemd-resolved, see :doc:`Architecture docs <./architecture>` for more info). You may wish to disable DNS mode for various reasons. This installation option gives you the full feature set minus CoreDNS.

To run without DNS, follow the :doc:`Quick Install <./quick-start>` guide, omitting the steps for DNS setup. In addition, when the guide has you pull (wget) the Netmaker docker-compose template, use the following link instead:

#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.nodns.yml``

This template is equivalent but omits CoreDNS.


Linux Install without Docker
=============================

Most systems support Docker, but some, such as LXC, do not. In such environments, there are many options for installing Netmaker. Netmaker is available as a binary file, and there is a zip file of the Netmaker UI static HTML on GitHub. Beyond the UI and Server, you need to install MongoDB and CoreDNS (optional). 

To start, we recommend following the Nginx instructions in the :doc:`Quick Install <./quick-start>` guide to enable SSL for your environment.

Once this is enabled and configured for a domain, you can continue with the below. The recommended server runs Ubuntu 20.04.

rqlite Setup
----------------
1. Install rqlite on your server: https://github.com/rqlite/rqlite

2. Run rqlite: rqlited -node-id 1 ~/node.1

Server Setup
-------------
1. **Run the install script:** 

``sudo curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/develop/scripts/netmaker-server.sh | sh -``

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

Kubernetes Install
=======================

Server Install
--------------------------

This template assumes your cluster uses Nginx for ingress with valid wildcard certificates. If using an ingress controller other than Nginx (ex: Traefik), you will need to manually modify the Ingress entries in this template to match your environment.

This template also requires RWX storage. Please change references to storageClassName in this template to your cluster's Storage Class.

``wget https://raw.githubusercontent.com/gravitl/netmaker/develop/kube/netmaker-template.yaml``

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

  wget https://raw.githubusercontent.com/gravitl/netmaker/develop/kube/netclient-template.yaml
  sed -i ‘s/ACCESS_TOKEN_VALUE/< your access token value>/g’ netclient-template.yaml
  kubectl apply -f netclient-template.yaml

For a more detailed guide on integrating Netmaker with MicroK8s, `check out this guide <https://itnext.io/how-to-deploy-a-cross-cloud-kubernetes-cluster-with-built-in-disaster-recovery-bbce27fcc9d7>`_. 

Nginx Reverse Proxy Setup with https
====================================

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
