
# Why another CoreDNS plugin

Netmaker when running on HA architecture needs a shared filesystem to store the
DNS data.  These shared filesystems are hard to build and maintain. The goal of
this plugin is to rely on the API (hence the database) as the only source of
truth.

This allows to run a CoreDNS server uncorrelated from the Netmaker installation
that handles the DNS queries accurately. 

# Configuration

```
.:53 {
  log
  debug
  netmaker {
    api_url https://vpn.example.com
    api_key secret_key
    fallthrough
  }
  forward . /etc/resolv.conf
}
```


With the previous configuration, all requests will be given to the Netmaker
plugin. If it can find a match on the given name, it will return the IP from
within the Netmaker domain. Else it will forward the request to the next
resolver.

# How to work on this plugin

First, you need to fetch the CoreDNS repository, and be able to build it.

Then copy the plugin content into `[coredns_path]/plugin/netmaker`

```bash
cd <coredns_path>
ln -s [netmaker_path/coredns/plugin/] plugins/netmaker/
```

run the following commande once:

```
sed -i '/file:file/i netmaker:netmaker' plugin.cfg
```

Then you can prepare a configuration file that could respond to your queries:

```
cat > coredns.conf<<EOF
.:10053 {
  log
  debug
  netmaker {
    api_url https://vpn.example.com
    api_key secret_key
    fallthrough
  }
  forward . /etc/resolv.conf
}
EOF
```

Your should generate the resources (based on the `plugin.cfg` list) with the command

```bash
go generate
go run . -plugin # to list plugins
```

Then run the test server with the following command:

```bash
go run . -conf coredns.conf
```

