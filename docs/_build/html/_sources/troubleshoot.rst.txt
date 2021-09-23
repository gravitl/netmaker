=================
Troubleshooting
=================

Common Issues
--------------
**How can I connect my Android or IOS device to my Netmaker VPN?**
  Currently meshing one of these devices is not supported, however it will be soon. 
  For now you can connect to your VPN by making one of the nodes an Ingress Gateway, then 
  create an Ext Client for each device. Finally, use the official WG app or another 
  WG configuration app to connect via QR or downloading the device's WireGuard configuration. 

**I've made changes to my nodes but the nodes themselves haven't updated yet, why?**
  Please allow your nodes to complete a check in or two, in order to reconfigure themselves.
  In some cases, it could take up to a minute or so.

**Do I have to use access keys to join a network?**
  Although keys are the preferred way to join a network, Netmaker does allow for manual node sign-ups.
  Simply turn on "allow manual signups" on your network and nodes will not connect until you manually aprove each one.

**Is there a community or forum to ask questions about Netmaker?**
  Yes, we have an active `discord <https://discord.gg/Pt4T9y9XK8>`_ community and issues on our `github <https://github.com/gravitl/netmaker/issues>`_ are answered frequently!
  You can also sign-up for updates at our `gravitl site <https://gravitl.com/>`_!

**How can I get additional support for my business?**
  Check out our business support subscriptions at https://gravitl.com/plans. Subscription holders can also purchase consulting credits via the site.


Server
-------

**I upgraded from 0.7 to 0.8 and now I dont have any data in my server!**
  In 0.8, sqlite becomes the default database. If you were running with rqlite, you must set the DATABASE environment variable to rqlite in order to continue using rqlite.

**Can I secure/encrypt all the traffic to my server and UI?**
  This can fairly simple to achieve assuming you have access to a domain and are familiar with Nginx.
  Please refer to the quick-start guide to see!

**Can I connect multiple nodes (mesh clients) behind a single firewall/router?**
  Yes! As of version 0.7 Netmaker supports UDP Hole Punching to allow this, without the use of a third party STUN server!
  Is UDP hole punching a risk for you? Well you can turn it off and make static nodes/ports for the server to refer to as well.

**What are the minimum specs to run the server?**
  We recommend at least 1 CPU and 2 GB Memory.

**Does this support IPv6 addressing?**
  Yes, Netmaker supports IPv6 addressing. When you create a network, just make sure to turn on Dual Stack.
  Nodes will be given IPv6 addresses along with their IPv4 address. It does not currently support IPv6 only.

**Does Netmaker support Raft Consensus?**
  Netmaker does not directly support it, but it uses `rqlite <https://github.com/rqlite/rqlite>`_ (which supports Raft) as the database.

**How do I uninstall Netmaker?**
  There is no official uninstall script for the Netmaker server at this time. If you followed the quick-start guide, simply run ``sudo docker-compose -f docker-compose.quickstart.yml down --volumes``
  to completely wipe your server. Otherwise kill the running binary and it's up to you to remove database records/volumes.

UI
----
**I want to make a seperate network and give my friend access to only that network.**
  Simply navigate to the UI (as an admin account). Select users in the top left and create them an account.
  Select the network(s) to give them and they should be good to go! They are an admin of that network(s) only now.

**I'm done with an access key, can I delete it?**
  Simply navigate to the UI (as an admin account). Select your network of interest, then the select the ``Access Keys`` tab.
  Then delete the rogue access key.

**I can't delete my network, why?**
  You **MUST** remove all nodes in a network before you can delete it.

**Can I have multiple nodes with the same name?**
  Yes, nodes can share names without issue. It may just be harder on you to know which is which.

Netclient
-----------
**How do I connect a node to my Netmaker network with Netclient?**
  First get your access token (not just access key), then run ``sudo netclient join -t <access token>``.
  **NOTE:** netclient may be under /etc/netclient/, i.e run ``sudo /etc/netclient/netclient join -t <access token>``

**How do I disconnect a node on a Netmaker network?**
  In order to leave a Netmaker network, run ``sudo netclient leave -n <network-name>``

**How do I check the logs of my agent on a node?**
  You will need sudo/root permissions, but you can run ``sudo systemctl status netclient@<insert network name>``
  or you may also run ``sudo journalctl -u netclient@<network name>``. 
  Note for journalctl: you should hit the ``end`` key to get to view the most recent logs quickly or use ``journalctl -u netclient@<network name> -f`` instead.

**Can I check the configuration of my node on the node?**
  **A:** Yes, on the node simply run ``sudo cat /etc/netclient/netconfig-<network name>`` and you should see what your current configuration is! 
  You can also see the current WireGuard configuration with ``sudo wg show``

**I am done with the agent on my machine, can I uninstall it?**
  Yes, on the node simply run ``sudo /etc/netclient/netclient uninstall``. 

**I am running SELinux and when I reboot my node I get a permission denied in my netclient logs and it doesn't connect anymore, why?**
  If you're running SELinux, it will interfere with systemd's ability to restart the client properly. Therefore, please run the following:
  .. code-block::
  
    sudo semanage fcontext -a -t bin_t '/etc/netclient/netclient' 
    sudo chcon -Rv -u system_u -t bin_t '/etc/netclient/netclient' 
    sudo restorecon -R -v /etc/netclient/netclient

**I have a handshake with a peer but can't ping it, what gives?**
  This is commonly due to incorrect MTU settings. Typically, it will be because MTU is too high. Try setting MTU lower on the node. This can be done via netconfig, or by editing the node in the UI. 

**I have a hard to reach machine behind a firewall or a corporate NAT, what can I do?**
  In this situation you can use the Relay Server functionality introduced in Netmaker v0.8 to designate a node as a relay to your "stuck" machine. Simply click the button to make a node into a relay and tell it to relay traffic to this hard-to-reach peer. 


CoreDNS
--------
**Is CoreDNS required to use Netmaker?**
  CoreDNS is not required. Simply start your server with ``DNS_MODE="off"``.

**What is the minimum DNS entry value I can use?**
  Netmaker supports down to two characters for DNS names for your networks domains**
