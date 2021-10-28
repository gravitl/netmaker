# Netmaker

## Name

*netmaker* - adds the ability to convert server names into their floating IPs.

## Description

This plugin allows to resolve names build like `<server_name>.[network-name]` into the
corresponding floating IP.

## Syntax

* `api_url` is the full URL of the netmaker server (ie: `https://netmaker.example.com`)
* `api_key` is the API_KEY that allows to make queries to the API server (https://docs.netmaker.org/api.html#authentication)
* `fallthrough` allow to fall to next plugin. It accepts zones arguments to filter next plugins.
* `refresh_duration` specifies the interval to refresh informations from netmaker servers.

## Examples

```
.:53 {
    log
    errors
    netmaker {
        api_url https://api.netmaker.example.com
        api_key secret_key
        refresh_duration 1s
        fallthrough
    }
    forward . /etc/resolv.conf
}
```
