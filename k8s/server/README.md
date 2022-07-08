# Netmaker K8S YAML Templates - Run Netmaker on Kubernetes

This will walk you through setting up a highly-available netmaker deployment on Kubernetes. Note: do not attempt to control networking on a Kubernetes cluster where Netmaker is deployed. This can result in circular logic and unreachable networks! Typically, a cluster should be designated as a "control" cluster.

You may want a more simple Kubernetes setup. We recommend [this community project](https://github.com/geragcp/netmaker-k3s). It is specific to K3S, but should be editable to work on most Kubernetes distributions.

### 0. Prerequisites

Your cluster must meet a few conditions to host a netmaker server. Primarily:  
a) **Nodes:** You must have at least 3 worker nodes available for Netmaker to deploy. Netmaker nodes have anti-affinity and will not deploy on the same kubernetes node.  
b) **Storage:** RWX and RWO storage classes must be available  
c) **Ingress:** Ingress must be configured with certs. Nginx + LetsEncrypt configs are provided by default. In addition, Traefik allows special ingress for TCP that may be preferable for HA MQTT. Additionally, be sure to have a wildcard DNS entry for use with Ingress/Netmaker  
e) **Helm:** For our Postgresql installation we rely on a helm chart, so you must have helm installed and configured.  
d) **MQ Broker Considerations:** Our method uses a NodePort for MQ. This can be used either with or without an external load balancer configured. If deploying without a load balancer, you must specify a node to host MQ, and this will not be HA. If using a load balancer, be aware that LB configuration could lead MQ connections to be lost. If this happens, on the client you will see a log like "unable to connect to broker, retrying ..." In either case DNS must be configured to point broker.domain either to the LB or directly to the hosting node. Finally, you can use a special TCPIngressRoute if Traefik is your Ingress provider ([see this repo](https://github.com/geragcp/netmaker-k3s) for an example). This is ideal, but is not a standard k8s object, so we avoid it to make installations possible across an array of k8s configurations.  

Assuming you are prepared for the above, we can begin to deploy Netmaker.  

### 1. Create Namespace
`kubectl create ns netmaker`  
`kubectl config set-context --current --namespace=netmaker`  

### 2. Deploy Database

Netmaker can use sqlite, postgres, or rqlite as a backing database. For HA, we recommend using Postgres, as Bitnami provides a reliable helm chart for deploying an HA pqsql cluster.
  
Follow these instructions:  
https://github.com/bitnami/charts/tree/master/bitnami/postgresql-ha  
  
`helm install postgres bitnami/postgresql`
  
Once completed, retrieve the password to access postgres:

`kubectl get secret --namespace netmaker postgres-postgresql -o jsonpath="{.data.postgres-password}" | base64 -d`  

### 3. Deploy MQTT

Based on the prerequisites, you will have either an external load balancer, traefik + TCPIngressRoute, or neither:  
a) **External LB:** Configure your LB to load balance 31883/tcp to your nodes. This will be for MQ traffic. **Important:** make sure that latency is not significant between clients and MQ. In testing, we found that some load balancers introduce too much latency, causing MQ to be unuseable.  
b) **Traefik:** If you have Traefik as your Ingress provider, create a TCPIngressRoute from 443 to mq service on port 8883. [Example Here](https://github.com/geragcp/netmaker-k3s/blob/main/08-ingress.yaml)  
c) **Neither (default):** Choose a cluster node to house MQTT and then run the following:  

c.a) `kubectl label node <your node name> mqhost=true`  

c.b) `sed -i 's/MQ_NODE_NAME/<your node name>/g' mosquitto.yaml`  

(If you are not using option C, comment out the pod affinity section from mosquitto.yaml.)

You also need an RWX storage class. Run the following to input your RWX storage class:

`sed -i 's/RWX_STORAGE_CLASS/<your storage class name>/g' mosquitto.yaml`

Now, apply the file:

`kubectl apply -f mosquitto.yaml`  

MQ should be in CrashLoopBackoff until Netmaker is deployed. If it's in pending state, check the pvc or the pod status (node selector may be incorrect).  

### 4. Deploy Netmaker Server

Make sure Wildcard DNS is set up for a netmaker subdomain, for instance: nm.mydomain.com. If you do not wish to use wildcard, edit the YAML file directly. Note you will need entries for broker.domain, api.domain, and dashboard.domain.  

`sed -i 's/NETMAKER_SUBDOMAIN/<your subdomain>/g' netmaker-server.yaml`  

Next, enter your postgres info, including the name of your postgres deployment and the password you retrieved above.  
  
`sed -i 's/DB_NAME/<postgres helm name>/g' netmaker-server.yaml`  
  
`sed -i 's/DB_PASS/<postgres helm password>/g' netmaker-server.yaml`  

Next, choose a secret password for your Netmaker API:

`sed -i 's/REPLACE_MASTER_KEY/<super secret password>/g' netmaker-server.yaml`  
  
Finally, you will need to create an Ingress object for your Netmaker API. An example is included in the YAML file for nginx + letsencrypt. You may use/modify this example, or create your own ingress which routes to the netmaker-rest service on port 8081. But make sure to deploy Ingress before moving on!  

Now, apply the file:  

`kubectl apply -f netmaker-server.yaml`  

Most likely, MQ will require a restart at this point:  

`kubectl delete pod mosquitto-<pod name>`  

When successful, all netmaker pods should display the following log:  

`[netmaker] 2022-00-00 00:00:00 successfully connected to mq broker`  


### 5. Deploy Netmaker UI

Much like above, you must make sure wildcard DNS is configured and make considerations for Ingress. Once again, add in your subdomain:  
  
`sed -i 's/NETMAKER_SUBDOMAIN/<your subdomain>/g' netmaker-ui.yaml`  

Again, Ingress is commented out. If you are using Nginx + LetsEncrypt, you can uncomment and use the yaml. Otherwise, set up Ingress manually.  

`kubectl apply -f netmaker-ui.yaml`  

At this point, you should be able to reach the server at domain.yourdomain and start setting up your networks.  
  
### Troubleshooting

Sometimes, the server has a hard time connecting to MQ using the self-generated certs on the first try. If this happens, try the following:

1. restart MQ: `kubectl delete pod <mq pod name>`  
2. restart netmaker pods:  
2.a.  `kubectl scale sts netmaker --replicas=0`  
2.b.  `kubectl delete pods netmaker-0 netmaker-1 netmaker-2`  
2.c.  `kubectl scale sts netmaker --replicas=3`  
  
In addition, try deleting the certs in MQ before running the above:

1. `kubectl exec -it mosquitto-<pod name> /bin/sh`
2. `rm mosquitto/certs/*`
3. `exit`
4. `kubectl delete pod mosquitto-<pod name>`
2. `kubectl scale sts netmaker --replicas=0` (wait until pods are down) `kubectl scale sts netmaker --replicas=3`