====================
Server Installation
====================

This section outlines installing the Netmaker server, including Netmaker, Netmaker UI, MongoDB, and CoreDNS

Notes on Optional Features
============================

There are a few key options to keep in mind when deploying Netmaker. All of the following options are enabled by default but can be disabled with a single flag at runtime (see Customization). In addition to these options, there are many more Customizable components which will be discussed later on and help to solve for special challenges and use cases.

**Client Mode:** Client Mode enables Netmaker to control the underlying host server's Network. This can make management a bit easier, because Netmaker can be added into networks via a button click in the UI. This is especially useful for things like Gateways, and will open up additional options in future versions, for instance, allowing Netmaker to easily become a relay server.

Client Mode requires many additional privileges on the host machine, since Netmaker needs to control kernel WireGuard. Because of this, if running in Client Mode, you must run with root privileges and mount many system directories to the Netmaker container. Running without Client Mode allows you to install without privilege escalation and increases the number of compatible systems substantially.

**DNS Mode:** DNS Mode enables Netmaker to write configuration files for CoreDNS, which can be set as a DNS Server for nodes. DNS Mode, paired with a CoreDNS deployment, requires use of port 53. On many linux systems (such as Ubuntu), port 53 is already in use to support local DNS, via systemd-resolved. Running in DNS Mode may require making modifications on the host machine.

**Secure GRPC**: Secure GRPC ensures all communications between nodes and the server are encrypted. Netmaker sets up a default "comms" network that exists only for nodes to connect to the server. It acts as a hub-and-spoke WireGuard network. In the below installation instructions, when port 50555 needs to be open, this is referring to the WireGuard port for Netmaker's GRPC comms. When it is port 50051, secure comms is not enabled. 

When Secure GRPC is enabled, before any nodes can join a Netmaker network, they request to join the comms network, and are given the appropriate WireGuard configs to connect to the server. Then they are able to make requests against the private netmaker endpoint specified for the comms network (10.101.0.1 by default). If switched off, communications are not secure between the hub and nodes over GRPC (it is like http vs https), and likewise, certificates must be added to gain secure communications.

**Agent Backend:** The Agent Backend is the GRPC server (by default running on port 50051). This port is not needed for the admin server. If your use case requires special access configuration, you can run two Netmaker instances, one for the admin server, and one for node access.

**REST Backend:** Similar to the above, the REST backend runs by default on port 8081, and is used for admin API and UI access. By enabling the REST backend while disabling the Agent backend, you can separate the two functions for more restricted environments.


System Compatibility
====================

Both **Client Mode** and **Secure GRPC** require WireGuard to be installed on the host system, and will require elevated privileges to perform network operations..

When both of these features are **disabled**, Netmaker can be run on any system that supports Docker, including Windows, Mac, and Linux, and other systems. With these features disabled, no special privileges are required. Netmaker will only need ports for GRPC (50051 by default), the API (8081 by default), and CoreDNS (53, if enabled).

With Client Mode and/or Secure GRPC **enabled** (the default), Netmaker has the same limitations as the :doc:`netclient <./client-installation>` (client networking agent), because client mode just means that the Netmaker server is also running a netclient. 

These modes require privileged (root) access to the host machine. In addition, Client Mode requires multiple host directory mounts. WireGuard must be installed, the system must be systemd Linux (see :doc:`compatible systems <./architecture>` for more details).

To run a non-docker installation, you must run the Netmaker binary, CoreDNS binary, MongoDB, and a web server directly on the host. This requires all the requirements for those individual components. Our guided install assumes systemd-based linux, but there are many other ways to install Netmaker's individual components onto machines that do not support Docker. 

DNS Mode Prereqisite Setup
====================================

If you plan on running the server in DNS Mode, know that a `CoreDNS Server <https://coredns.io/manual/toc/>`_ will be installed. CoreDNS is a light-weight, fast, and easy-to-configure DNS server. It is recommended to bind CoreDNS to port 53 of the host system, and it will do so by default. The clients will expect the nameserver to be on port 53, and many systems have issues resolving a different port.

However, on your host system (for Netmaker), this may conflict with an existing process. On linux systems running systemd-resolved, there is likely a service consuming port 53. The below steps will disable systemd-resolved, and replace it with a generic (e.g. Google) nameserver. Be warned that this may have consequences for any existing private DNS configuration. The following was tested on Ubuntu 20.04 and should be run prior to deploying the docker containers.

1. ``systemctl stop systemd-resolved`` 
2. ``systemctl disable systemd-resolved`` 
3. ``vim /etc/systemd/resolved.conf``
    * uncomment DNS and add 8.8.8.8 or whatever reachable nameserver is your preference
    * uncomment DNSStubListener and set to "no"
4. ``ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf``

Port 53 should now be available for CoreDNS to use.

Docker Compose Install
=======================

The most simple (and recommended) way of installing Netmaker is to use one of the provided `Docker Compose files <https://github.com/gravitl/netmaker/tree/feature_v0.3.5_docs/compose>`_. Below are instructions for several different options to install Netmaker via Docker Compose, followed by an annotated reference Docker Compose in case your use case requires additional customization.

Slim Install - No DNS, No Client Mode, No Secure GRPC
--------------------------------------------------------

This is the same docker compose covered in the :doc:`quick start <./quick-start>`. It requires no special privileges and can run on any system with Docker and Docker Compose. However, it also does not have the full feature set, and lacks Client Mode and DNS Mode.

**Prerequisites:**
  * ports 80, 8081, and 50051 are not blocked by firewall
  * ports 80, 8081, 50051, and 27017 are not in use 

**Notes:** 
  * You can still run the netclient on the host system even if Client Mode is not enabled. It will just be managed like the netclient on any other nodes, and will not be automatically managed by thhe server/UI.
  * You can change the port mappings in the Docker Compose if the listed ports are already in use.

Assuming you have Docker and Docker Compose installed, you can just run the following, replacing **< Insert your-host IP Address Here >** with your host IP (or domain):

#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.slim.yml``
#. ``sed -i ‘s/HOST_IP/< Insert your-host IP Address Here >/g’ docker-compose.yml``
#. ``docker-compose up -d``

Full Install - DNS, Client Mode, and Secure GRPC Enabled
----------------------------------------------------------

This installation gives you the fully-featured product with Client Mode and DNS Mode. 

**Prerequisites:**
  * systemd linux (Debian or Ubuntu reccommended)
  * sudo privileges
  * DNS Mode Prerequisite Setup (see above)
  * WireGuard installed
  * ports 80, 8081, 53, and 50555 are not blocked by firewall
  * ports 80, 8081, 53, 50555, and 27017 are not in use

**Notes:** 
  * You can change the port mappings in the Docker Compose if the listed ports are already in use.
  * You can run CoreDNS on a non-53 port, but this likely will cause issues on the client side (DNS on non-standard port). We do not recommend this and do not cover how to manage running CoreDNS on a different port for clients, which will likely have problems resolving a nameserver on a non-53 port.

Assuming you have Docker and Docker Compose installed, you can just run the following, replacing **< Insert your-host IP Address Here >** with your host IP (or domain):

#. ``sudo su -``
#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.yml``
#. ``sed -i ‘s/HOST_IP/< Insert your-host IP Address Here >/g’ docker-compose.yml``
#. ``docker-compose up -d``


Server Only Install - UI, DNS, Client Disabled
------------------------------------------------

A "Server Only" install can be helpful for scenarios in which you do not want to run the UI. the UI is not mandatory for running a Netmaker network, but it makes the process easier. This mode also diables DNS and Client Modes, though you can add those back in if needed. There is no UI dependency on Client Mode or DNS Mode.

**Prerequisites:**
  * ports 8081 and 50051 are not blocked by firewall
  * ports 8081, 50051, and 27017 are not in use

**Notes:**
  * You can still run the netclient on the host system even if Client Mode is not enabled. It will just be managed like the netclient on any other nodes, and will not be automatically managed by thhe server/UI.
  * You can change the port mappings in the Docker Compose if the listed ports are already in use.

Assuming you have Docker and Docker Compose installed, you can just run the following, replacing **< Insert your-host IP Address Here >** with your host IP (or domain):

#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.server-only.yml``
#. ``sed -i ‘s/HOST_IP/< Insert your-host IP Address Here >/g’ docker-compose.yml``

No DNS - CoreDNS Disabled, Client Enabled
----------------------------------------------

DNS Mode is currently limited to clients that can run resolvectl (systemd-resolved, see :doc:`Architecture docs <./architecture>` for more info). You may wish to disable DNS mode for various reasons. This installation option gives you the full feature set minus CoreDNS.

**Prerequisites:**
  * systemd linux (Debian or Ubuntu reccommended)
  * sudo privileges
  * WireGuard installed
  * ports 80, 8081, and 50555 are not blocked by firewall
  * ports 80, 8081, 50555, and 27017 are not in use

**Notes:** 
  * You can change the port mappings in the Docker Compose if the listed ports are already in use.
  * If you would like to run DNS Mode, but disable it on some clients, this is also an option. See the :doc:`client installation <./client-installation>` documentation for more details.

Assuming you have Docker and Docker Compose installed, you can just run the following, replacing **< Insert your-host IP Address Here >** with your host IP (or domain):

#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.nodns.yml``
#. ``sed -i ‘s/HOST_IP/< Insert your-host IP Address Here >/g’ docker-compose.yml``

No DNS - CoreDNS Disabled, Client Enabled

No Client - DNS Enabled, Client Disabled
---------------------------------------------

You may want to provide DNS, but do not want to run the server with special privileges, in which case you can run with just Client Mode disabled. It requires no special privileges and can run on any system with Docker and Docker Compose. 

**Prerequisites:**
  * ports 80, 8081, 53, and 50051 are not blocked by firewall
  * ports 80, 8081, 53, 50051, and 27017 are not in use
  * DNS Mode Prerequisite Setup (see above)

**Notes:** 
  * You can still run the netclient on the host system even if Client Mode is not enabled. It will just be managed like the netclient on any other nodes, and will not be automatically managed by thhe server/UI.
  * You can change the port mappings in the Docker Compose if the listed ports are already in use.

Assuming you have Docker and Docker Compose installed, you can just run the following, replacing **< Insert your-host IP Address Here >** with your host IP (or domain):

#. ``wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/docker-compose.noclient.yml``
#. ``sed -i ‘s/HOST_IP/< Insert your-host IP Address Here >/g’ docker-compose.yml``
#. ``docker-compose up -d``


Reference Compose File - Annotated
--------------------------------------

All environment variables and options are enabled in this file. It is the equivalent to running the "full install" from the above section. However, all environment variables are included, and are set to the default values provided by Netmaker (if the environment variable was left unset, it would not change the installation). Comments are added to each option to show how you might use it to modify your installation.

.. literalinclude:: ../compose/docker-compose.reference.yml
  :language: YAML


Linux Install without Docker
=============================

Most systems support Docker, but some, such as LXC, do not. In such environments, there are many options for installing Netmaker. Netmaker is available as a binary file, and there is a zip file of the Netmaker UI static HTML on GitHub. Beyond the UI and Server, you need to install MongoDB and CoreDNS (optional). 

Below is a guided set of instructions for installing without Docker on Ubuntu 20.04. Depending on your system, the steps may vary.

MongoDB Setup
----------------
1. Install MongoDB on your server:
    * For Ubuntu: `sudo apt install -y mongodb`
    * For more advanced installation or other operating systems, see  the `MongoDB documentation <https://docs.mongodb.com/manual/administration/install-community/>`_.

2. Create a user:
    * ``mongo admin``  
    * > `db.createUser({ user: "mongoadmin" , pwd: "mongopass", roles: ["userAdminAnyDatabase", "dbAdminAnyDatabase", "readWriteAnyDatabase"]})`

Server Setup
-------------
1. **Run the install script:** ``sudo curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.3.5/scripts/netmaker-server.sh | sh -``
2. Check status:  ``sudo journalctl -u netmaker``
3. If any settings are incorrect such as host or mongo credentials, change them under /etc/netmaker/config/environments/< your env >.yaml and then run ``sudo systemctl restart netmaker``

UI Setup
-----------

The following uses NGinx as an http server. You may alternatively use Apache or any other web server that serves static web files.

1. **Download UI asset files:** ``sudo wget -O /usr/share/nginx/html/netmaker-ui.zip https://github.com/gravitl/netmaker-ui/releases/download/latest/netmaker-ui.zip``
2. **Unzip:** ``sudo unzip /usr/share/nginx/html/netmaker-ui.zip -d /usr/share/nginx/html``
3. **Copy Config to Nginx:** ``sudo cp /usr/share/nginx/html/nginx.conf /etc/nginx/conf.d/default.conf``
4. **Modify Default Config Path:** ``sudo sed -i 's/root \/var\/www\/html/root \/usr\/share\/nginx\/html/g' /etc/nginx/sites-available/default``
5. **Change Backend URL:** ``sudo sh -c 'BACKEND_URL=http://<YOUR BACKEND API URL>:PORT /usr/share/nginx/html/generate_config_js.sh >/usr/share/nginx/html/config.js'``
6. **Start Nginx:** ``sudo systemctl start nginx``

CoreDNS Setup
----------------

Kubernetes Install
=======================

**This configuration is coming soon.** It will allow you to deploy Netmaker on a Kubernetes cluster.

Configuration Reference
=========================

The "Reference Compose File" (above) explains many of these options. However, it is important to understand fundamentally how Netmaker sets its configuration:

1. Defaults
2. Config File
3. Environment Variables

Variable Description
----------------------

SERVER_HOST: 
    **Default:** Server will perform an IP check and set automatically unless explicitly set, or DISABLE_REMOTE_IP_CHECK is set to true, in which case it defaults to 127.0.0.1

    **Description:** Sets the SERVER_HTTP_HOST and SERVER_GRPC_HOST variables if they are unset. The address where traffic comes in. 

SERVER_HTTP_HOST: 
    **Default:** Equals SERVER_HOST if set, "127.0.0.1" if SERVER_HOST is unset.
    
    **Description:** Set to make the HTTP and GRPC functions available via different interfaces/networks.

SERVER_GRPC_HOST: 
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

CLIENT_MODE:  
    **Default:** "on" 

    **Description:** Enables Client Mode, meaning netclient will be deployed on server and will be manageable from UI. Change to "off" to turn off.

DNS_MODE:  
    **Default:** "on"

    **Description:** Enables DNS Mode, meaning config files will be generated for CoreDNS.

DISABLE_REMOTE_IP_CHECK:  
    **Default:** "off" 

    **Description:** If turned "on", Server will not set Host based on remote IP check. This is already overridden if SERVER_HOST is set. Turned "off" by default.

MONGO_ADMIN:  
    **Default:** "mongoadmin" 

    **Description:** Admin user for MongoDB.

MONGO_PASS:  
    **Default:** "mongopass" 

    **Description:** Admin password for MongoDB.

MONGO_HOST:  
    **Default:** "127.0.0.1"

    **Description:** Address of MongoDB.

MONGO_PORT:  
    **Default:** "27017"

    **Description:** Port of MongoDB.

MONGO_OPTS:  
    **Default:** "/?authSource=admin"

    **Description:** Opts to enable admin login for Mongo.

SERVER_GRPC_WIREGUARD: 
    **Default:** "on"

    **Description:** Whether to run GRPC over a WireGuard network. On by default. Secures the server comms. Switch to "off" to turn off. If off and running in production, make sure to have certificates installed to secure GRPC communications. 

SERVER_GRPC_WG_INTERFACE: 
    **Default:** "nm-grpc-wg"

    **Description:** Interface to use for GRPC WireGuard network if enabled

SERVER_GRPC_WG_ADDRESS:
    **Default:** "10.101.0.1"

    **Description:** Private Address to use for GRPC WireGuard network if enabled

SERVER_GRPC_WG_ADDRESS_RANGE:
    **Default:** "10.101.0.0/16"

    **Description:** Private Address range to use for GRPC WireGard clients if enabled. Gives 65,534 total addresses for all of netmaker. If running a larger network, will need to configure addresses differently, for instance using ipv6, or use certificates instead.

SERVER_GRPC_WG_PORT:
    **Default:** 50555

    **Description:** Port to use for GRPC WireGuard if enabled

SERVER_GRPC_WG_PUBKEY:
    **Default:** < generated at startup >

    **Description:** PublicKey for GRPC WireGuard interface. Generated if left blank.

SERVER_GRPC_WG_PRIVKEY:
    **Default:** < generated at startup >

    **Description:** PrivateKey for GRPC WireGuard interface. Generated if left blank.

SERVER_GRPC_WG_KEYREQUIRED
    **Default:** ""

    **Description:** Determines if an Access Key is required to join the Comms network. Blank (meaning 'no') by default. Set to "yes" to turn on.

GRPC_SSL
    **Default:** ""

    **Description:** Specifies if GRPC is going over secure GRPC or SSL. This is a setting for the clients and is passed through the access token. Can be set to "on" and "off". Set to on if SSL is configured for GRPC.

SERVER_API_CONN_STRING
    **Default:** ""

    **Description:**  Allows specification of the string used to connect to the server api. Format: IP:PORT or DOMAIN:PORT. Defaults to SERVER_HOST if not specified.

SERVER_GRPC_CONN_STRING
    **Default:** ""

    **Description:**  Allows specification of the string used to connect to grpc. Format: IP:PORT or DOMAIN:PORT. Defaults to SERVER_HOST if not specified.

Config File Reference
----------------------
A config file may be placed under config/environments/<env-name>.yml. To read this file at runtime, provide the environment variable ENV at runtime. For instance, dev.yml paired with ENV=dev. Netmaker will load the specified Config file. This allows you to store and manage configurations for different environments. Below is a reference Config File you may use.

.. literalinclude:: ../config/environments/dev.yaml
  :language: YAML


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
