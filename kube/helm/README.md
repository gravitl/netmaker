# Netmaker Helm

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.9.0](https://img.shields.io/badge/AppVersion-0.9.0-informational?style=flat-square)

A Helm chart to run Netmaker with High Availability on Kubernetes:

```
helm repo add netmaker https://gravitl.github.io/netmaker-helm/
helm repo update
```

## Requirements

To run HA Netmaker on Kubernetes, your cluster must have the following:
- RWO and RWX Storage Classes (RWX is only required if running Netmaker with DNS Management enabled).
- An Ingress Controller and valid TLS certificates 
	- This chart can currently generate ingress for:
		- Nginx Ingress + LetsEncrypt/Cert-Manager
		- Traefik Ingress + LetsEncrypt/Cert-Manager
	- to generate automatically, make sure one of the two is configured for your cluster

Furthermore, the chart will by default install and use a postgresql cluster as its datastore: 

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | postgresql-ha | 7.11.0 |

### Example Install

```
helm install netmaker/netmaker --generate-name \ # generate a random id for the deploy 
--set baseDomain=nm.example.com \ # the base wildcard domain to use for the netmaker api/dashboard/grpc ingress 
--set replicas=3 \ # number of server replicas to deploy (3 by default) 
--set ingress.enabled=true \ # deploy ingress automatically (requires nginx or traefik and cert-manager + letsencrypt) 
--set ingress.className=nginx \ # ingress class to use 
--set ingress.tls.issuerName=letsencrypt-prod \ # LetsEncrypt certificate issuer to use 
--set dns.enabled=true \ # deploy and enable private DNS management with CoreDNS 
--set dns.clusterIP=10.245.75.75 --set dns.RWX.storageClassName=nfs \ # required fields for DNS 
--set postgresql-ha.postgresql.replicaCount=2 \ # number of DB replicas to deploy (default 2)
```

### Recommended Settings:
A minimal HA install of Netmaker can be run with the following command:
`helm install netmaker --generate-name --set baseDomain=nm.example.com`
This install has some notable exceptions:
- Ingress **must** be manually configured post-install (need to create valid Ingress with TLS)
- Server will use "userspace" WireGuard, which is slower than kernel WG
- DNS will be disabled

Below, we discuss the considerations for Ingress, Kernel WireGuard, and DNS.

#### Ingress	
To run HA Netmaker, you must have ingress installed and enabled on your cluster with valid TLS certificates (not self-signed). If you are running Nginx or Traefik as your Ingress Controller and LetsEncrypt for TLS certificate management, you can run the helm install with the following settings:
`--set ingress.enabled=true`
`--set ingress.className=<nginx|traefik>`
`--set ingress.annotations.cert-manager.io/cluster-issuer=<your LE issuer name>`

If you are not using Nginx or Traefik and LetsEncrypt, we recommend leaving ingress.enabled=false (default), and then manually creating the ingress objects post-install. You will need three ingress objects with TLS:
`dashboard.<baseDomain>`
`api.<baseDomain>`
`grpc.<baseDomain>`

The gRPC ingress object must include annotations to use the gRPC protocol, which is supported by most ingress controllers. For instance, on Traefik, the annotation is:
`ingress.kubernetes.io/protocol: h2c`

You can find example ingress objects in the kube/example folder.

#### Kernel WireGuard
If you have control of the Kubernetes worker node servers, we recommend **first** installing WireGuard on the hosts, and then installing HA Netmaker in Kernel mode. By default, Netmaker will install with userspace WireGuard (wireguard-go) for maximum compatibility, and to avoid needing permissions at the host level. If you have installed WireGuard on your hosts, you should install Netmaker's helm chart with the following option:
`--set wireguard.kernel=true`

#### DNS
By Default, the helm chart will deploy without DNS enabled. To enable DNS, specify with:
`--set dns.enabled=true` 
This will require specifying a RWX storage class, e.g.:
`--set dns.RWX.storageClassName=nfs`
This will also require specifying a service address for DNS. Choose a valid ipv4 address from the service IP CIDR for your cluster, e.g.:
`--set dns.clusterIP=10.245.69.69`

**This address will only be reachable from hosts that have access to the cluster service CIDR.** It is only designed for use cases related to k8s. If you want a more general-use Netmaker server on Kubernetes for use cases outside of k8s, you will need to do one of the following:
- bind the CoreDNS service to port 53 on one of your worker nodes and set the COREDNS_ADDRESS equal to the public IP of the worker node
- Create a private Network with Netmaker and set the COREDNS_ADDRESS equal to the private address of the host running CoreDNS. For this, CoreDNS will need a node selector and will ideally run on the same host as one of the Netmaker server instances.



## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| dns.enabled | bool | `false` | whether or not to run with DNS (CoreDNS) |
| dns.storageSize | string | `"128Mi"` | volume size for DNS (only needs to hold one file) |
| fullnameOverride | string | `""` | override the full name for netmaker objects  |
| image.pullPolicy | string | `"Always"` | Pull Policy for images |
| image.repository | string | `"gravitl/netmaker"` | The image repo to pull Netmaker image from  |
| image.tag | string | `"v0.8.4"` | Override the image tag to pull  |
| ingress.annotations.base."kubernetes.io/ingress.allow-http" | string | `"false"` | annotation to generate ACME certs if available |
| ingress.annotations.grpc.nginx."nginx.ingress.kubernetes.io/backend-protocol" | string | `"GRPC"` | annotation to use grpc protocol on grpc domain |
| ingress.annotations.grpc.traefik."ingress.kubernetes.io/protocol" | string | `"h2c"` | annotation to use grpc protocol on grpc domain |
| ingress.annotations.nginx."nginx.ingress.kubernetes.io/rewrite-target" | string | `"/"` | destination addr for route |
| ingress.annotations.nginx."nginx.ingress.kubernetes.io/ssl-redirect" | string | `"true"` | Redirect http to https  |
| ingress.annotations.tls."kubernetes.io/tls-acme" | string | `"true"` | use acme cert if available |
| ingress.annotations.traefik."traefik.ingress.kubernetes.io/redirect-entry-point" | string | `"https"` | Redirect to https |
| ingress.annotations.traefik."traefik.ingress.kubernetes.io/redirect-permanent" | string | `"true"` | Redirect to https permanently |
| ingress.annotations.traefik."traefik.ingress.kubernetes.io/rule-type" | string | `"PathPrefixStrip"` | rule type |
| ingress.enabled | bool | `false` | attempts to configure ingress if true |
| ingress.hostPrefix.grpc | string | `"grpc."` | grpc route subdomain |
| ingress.hostPrefix.rest | string | `"api."` | api (REST) route subdomain |
| ingress.hostPrefix.ui | string | `"dashboard."` | ui route subdomain |
| ingress.tls.enabled | bool | `true` |  |
| ingress.tls.issuerName | string | `"letsencrypt-prod"` |  |
| nameOverride | string | `""` | override the name for netmaker objects  |
| podAnnotations | object | `{}` | pod annotations to add |
| podSecurityContext | object | `{}` | pod security contect to add |
| postgresql-ha.persistence.size | string | `"3Gi"` | size of postgres DB |
| postgresql-ha.postgresql.database | string | `"netmaker"` | postgress db to generate |
| postgresql-ha.postgresql.password | string | `"netmaker"` | postgres pass to generate |
| postgresql-ha.postgresql.username | string | `"netmaker"` | postgres user to generate |
| replicas | int | `3` | number of netmaker server replicas to create  |
| service.grpcPort | int | `443` | port for GRPC service |
| service.restPort | int | `8081` | port for API service |
| service.type | string | `"ClusterIP"` | type for netmaker server services |
| service.uiPort | int | `80` | port for UI service |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | Name of SA to use. If not set and create is true, a name is generated using the fullname template |
| ui.replicas | int | `2` | how many UI replicas to create |
| wireguard.enabled | bool | `true` | whether or not to use WireGuard on server |
| wireguard.kernel | bool | `false` | whether or not to use Kernel WG (should be false unless WireGuard is installed on hosts). |
| wireguard.networkLimit | int | `10` | max number of networks that Netmaker will support if running with WireGuard enabled |

