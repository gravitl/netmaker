=====================================
Upgrades
=====================================

Introduction
===============

As of 0.9.4, upgrading Netmaker is a manual process. This is expected to be automated in the future, but for now is still a relatively straightforward process. 

Upgrade the Server (netmaker)
==================================

To upgrade the server, you only need to change the docker image versions:

1. `ssh root@my-server-ip`
2. `docker compose down`
3. `vi docker-compose.yml`
4. Change gravitl/netmaker:<version> and gravitl/netmaker-ui:<version> to the new version.
5. Save and close the file
6. `docker-compose up -d`

Upgrade the Clients (netclient)
==================================

To upgrade the client, you must get the new client binary and place it in /etc/netclient. Depending on the new vs. old version, there may be minor incompatibilities (discussed below).

1. Vists https://github.com/gravitl/netmaker/releases/
2. Find the appropriate binary for your machine.
3. Download. E.x.: `wget https://github.com/gravitl/netmaker/releases/download/vX.X.X/netclient-myversion`
4. Rename binary to `netclient` and move to folder. E.x.: `mv netclient-myversion /etc/netclient/netclient`
5. `netclient --version` (confirm it's the correct version)
6. `netclient pull`

This last step helps ensure any newly added fields are now present. You may run into a "panic" based on missing fields and your version mismatch. In such cases, you can either:

1. Add the missing field to /etc/netclient/config/netconfig-yournetwork and then run "netclient checkin"

or

2. Leave and rejoin the network