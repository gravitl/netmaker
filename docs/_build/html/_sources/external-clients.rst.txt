================
External Clients
================

Introduction
===============

Netmaker allows for "external clients" to reach into a network and access services via an Ingress Gateway. So what is an "external client"? An external client is any machine which cannot or should not be meshed. This can include:
        - Phones
        - Laptops
        - Desktops

An external client is not "managed," meaning it does not automatically pull the latest network configuration, or push changes to its configuration. Instead, it uses a generated WireGuard config file to access the designated **Ingress Gateway**, which **is** a managed server (running netclient). This server then forwards traffic to the appropriate endpoint, acting as a middle-man/relay.

By using this method, you can hook any machine into a netmaker network that can run WireGuard.

It is recommended to run the netclient where compatible, but for all other cases, a machine can be configured as an external client.

Important to note, an external client is not **reachable** by the network, meaning the client can establish connections to other machines, but those machines cannot independently establish a connection back. The External Client method should only be used in use cases where one wishes to access resource runnin on the virtual network, and **not** for use cases where one wishes to make a resource accessible on the network. For that, use netclient.
