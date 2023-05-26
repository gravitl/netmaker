# Dev Scripts

Dev scripts for Netmaker

## Tunnel Compose

Creates tunnels between a local instance and a docker-compose deployment on a droplet. Allows for fast local builds and local debugging.

Steps:
1. Consider migrating the DB (see below)
1. Create 2 ssh hosts in `~/.ssh/config` (adjust `DROPLET` and `IP`):
```
Host DROPLET
    User root
    Hostname IP

Host DROPLET-docker-netmaker
    User user
    Hostname localhost
    Port 2222
    ProxyJump DROPLET
    StrictHostKeyChecking no
    UserKnownHostsFile=/dev/null
    RequestTTY no
    RemoteCommand cat
```
2. Copy [./scripts/dev/docker-compose.override.yml](docker-compose.override.yml) to the installation dir on DROPLET (merge if already exists)
3. `docker-compose down`
4. `docker-compose up --force-recreate`
5. `./scripts/dev/tunnel-compose.sh DROPLET-docker-netmaker`
6. Add env vars to the local build (include `MQ_PASSWORD` from the droplet), eg:<br />
    `MQ_PASSWORD=SECRET;MQ_USERNAME=netmaker;SERVER_BROKER_ENDPOINT=ws://localhost:1883;VERBOSE=3`

At this point tunnels should be set up and running a local build should talk to the docker-compose services on the droplet.


### DB migration

**Option 1** - run docker-compose (WITHOUT the override) and copy the existing DB: 
```bash
# on the droplet
docker-compose up --force-recreate netmaker
docker cp netmaker:/data/netmaker.db .
# on the host
scp DROPLET:netmaker.db data 
```

**Option 2** (CE ONLY) - re-run nm-quick (WITH the override and a running local server) to re-create the DB:
```bash
wget https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/nm-quick.sh
chmod +x nm-quick.sh
env NM_SKIP_BUILD=1 ./nm-quick.sh -b local -t BRANCH_NAME
```
