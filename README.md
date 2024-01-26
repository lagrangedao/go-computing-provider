The repository has been moved to a new organization and the latest progress is here，**[https://github.com/swanchain/go-computing-provider](https://github.com/swanchain/go-computing-provider)**

# Computing Provider
[![Discord](https://img.shields.io/discord/770382203782692945?label=Discord&logo=Discord)](https://discord.gg/Jd2BFSVCKw)
[![Twitter Follow](https://img.shields.io/twitter/follow/swan_chain)](https://twitter.com/swan_chain)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg)](https://github.com/RichardLitt/standard-readme)

A computing provider is an individual or organization that participates in the decentralized computing network by offering computational resources such as processing power (CPU and GPU), memory, storage, and bandwidth. Their primary role is to execute tasks posted by users on the Lagrange platform.


# Table of Content

 - [Prerequisites](#Prerequisites)
 - [Install the Kubernetes](#Install-the-Kubernetes)
 	- [Install Container Runtime Environment](#install-Container-Runtime-Environment)
 	- [Optional-Setup a docker registry server](#Optional-setup-a-Docker-Registry-Server)
	- [Create a Kubernetes Cluster](#Create-a-Kubernetes-Cluster)
 	- [Install the Network Plugin](#Install-the-Network-Plugin)
	- [Install the NVIDIA Plugin](#Install-the-NVIDIA-Plugin)
	- [Install the Ingress-nginx Controller](#Install-the-Ingress-nginx-Controller)
 - [Install and config the Nginx](#Install-the-Ingress-nginx-Controller)
 - [Install the Hardware resource-exporter](#Install-the-Hardware-resource-exporter)
 - [Install the Redis service](#Install-the-Redis-service)
 - [Build and config the Computing Provider](#Build-and-config-the-Computing-Provider)
 - [Install AI Inference Dependency(Optional)](#Install-AI-Inference-Dependency)
 - [Start the Computing Provider](#Start-the-Computing-Provider)
 - [CLI of Computing Provider](#CLI-of-Computing-Provider)

## Prerequisites
Before you install the Computing Provider, you need to know there are some resources required:
 - Possess a public IP
 - Have a domain name (*.example.com)
 - Have an SSL certificate
 - `Go` version must 1.19+, you can refer here:

```bash
wget -c https://golang.org/dl/go1.19.7.linux-amd64.tar.gz -O - | sudo tar -xz -C /usr/local

echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc && source ~/.bashrc
```

## Install the Kubernetes
The Kubernetes version should be `v1.24.0+`

###  Install Container Runtime Environment
If you plan to run a Kubernetes cluster, you need to install a container runtime into each node in the cluster so that Pods can run there, refer to [here](https://kubernetes.io/docs/setup/production-environment/container-runtimes/). And you just need to choose one option to install the `Container Runtime Environment`

#### Option 1: Install the `Docker` and `cri-dockerd` （**Recommended**）
To install the `Docker Container Runtime` and the `cri-dockerd`, follow the steps below:
* Install the `Docker`:
    - Please refer to the official documentation from [here](https://docs.docker.com/engine/install/).
* Install `cri-dockerd`:
    - `cri-dockerd` is a CRI (Container Runtime Interface) implementation for Docker. You can install it refer to [here](https://github.com/Mirantis/cri-dockerd).

#### Option 2: Install the `Containerd`
`Containerd` is an industry-standard container runtime that can be used as an alternative to Docker. To install `containerd` on your system, follow the instructions on [getting started with containerd](https://github.com/containerd/containerd/blob/main/docs/getting-started.md).

### Optional-Setup a docker registry server
**If you are using the docker and you have only one node, the step can be skipped**.

If you have deployed a Kubernetes cluster with multiple nodes, it is recommended to set up a **private Docker Registry** to allow other nodes to quickly pull images within the intranet. 

* Create a directory `/docker_repo` on your docker server. It will be mounted on the registry container as persistent storage for our docker registry.
```bash
sudo mkdir /docker_repo
sudo chmod -R 777 /docker_repo
```
* Launch the docker registry container:
```bash
sudo docker run --detach \
  --restart=always \
  --name registry \
  --volume /docker_repo:/docker_repo \
  --env REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/docker_repo \
  --publish 5000:5000 \
  registry:2
```
![1](https://github.com/lagrangedao/go-computing-provider/assets/102578774/0c4cd53d-fb5f-43d9-b804-be83faf33986)


* Add the registry server to the node

 	- If you have installed the `Docker` and `cri-dockerd`(**Option 1**), you can update every node's configuration:


	```bash
	sudo vi /etc/docker/daemon.json
	```
	```
	## Add the following config
	"insecure-registries": ["<Your_registry_server_IP>:5000"]
	```
	Then restart the docker service
	```bash
	sudo systemctl restart docker
	```

 	- If you have installed the `containerd`(**Option 2**), you can update every node's configuration:

```bash
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."<Your_registry_server_IP>:5000"]
      endpoint = ["http://<Your_registry_server_IP>:5000"]

[plugins."io.containerd.grpc.v1.cri".registry.configs]
  [plugins."io.containerd.grpc.v1.cri".registry.configs."<Your_registry_server_IP>:5000".tls]
      insecure_skip_verify = true                                                               
```

Then restart `containerd` service

```bash
sudo systemctl restart containerd
```
**<Your_registry_server_IP>:** the intranet IP address of your registry server.

Finally, you can check the installation by the command:
```bash
docker system info
```
![2](https://github.com/lagrangedao/go-computing-provider/assets/102578774/4cfc1981-3fca-415c-948f-86c496915cff)




### Create a Kubernetes Cluster
To create a Kubernetes cluster, you can use a container management tool like `kubeadm`. The below steps can be followed:

* [Install the kubeadm toolbox](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/).

* [Create a Kubernetes cluster with kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/) 


### Install the Network Plugin
Calico is an open-source **networking and network security solution for containers**, virtual machines, and native host-based workloads. Calico supports a broad range of platforms including **Kubernetes**, OpenShift, Mirantis Kubernetes Engine (MKE), OpenStack, and bare metal services.

To install Calico, you can follow the below steps, more information can be found [here](https://docs.tigera.io/calico/3.25/getting-started/kubernetes/quickstart).

**step 1**: Install the Tigera Calico operator and custom resource definitions
```
kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.1/manifests/tigera-operator.yaml
```

**step 2**: Install Calico by creating the necessary custom resource
```
kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.1/manifests/custom-resources.yaml
```
**step 3**: Confirm that all of the pods are running with the following command
```
watch kubectl get pods -n calico-system
```
**step 4**: Remove the taints on the control plane so that you can schedule pods on it.
```
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
kubectl taint nodes --all node-role.kubernetes.io/master-
```
If you have installed it correctly, you can see the result shown in the figure by the command `kubectl get po -A`

![3](https://github.com/lagrangedao/go-computing-provider/assets/102578774/91ef353f-72af-41b2-82e8-061b92bfb999)

**Note:** 
 - If you are a single-host Kubernetes cluster, remember to remove the taint mark, otherwise, the task can not be scheduled to it.
```bash
kubectl taint node ${nodeName}  node-role.kubernetes.io/control-plane:NoSchedule-
```

### Install the NVIDIA Plugin
If your computing provider wants to provide a GPU resource, the NVIDIA Plugin should be installed, please follow the steps:

* [Install NVIDIA Driver](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#nvidia-drivers).
>Recommend NVIDIA Linux drivers version should be 470.xx+

* [Install NVIDIA Device Plugin for Kubernetes](https://github.com/NVIDIA/k8s-device-plugin#quick-start).

If you have installed it correctly, you can see the result shown in the figure by the command 
`kubectl get po -n kube-system`

![4](https://github.com/lagrangedao/go-computing-provider/assets/102578774/8209c589-d561-43ad-adea-5ecb52618909)

### Install the Ingress-nginx Controller
The `ingress-nginx` is an ingress controller for Kubernetes using `NGINX` as a reverse proxy and load balancer. You can run the following command to install it:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.7.1/deploy/static/provider/cloud/deploy.yaml
```
If you have installed it correctly, you can see the result shown in the figure by the command: 

 - Run `kubectl get po -n ingress-nginx`

![5](https://github.com/lagrangedao/go-computing-provider/assets/102578774/f3c0585a-df19-4971-91fe-d03365f4edee)

 - Run `kubectl get svc -n ingress-nginx`

![6](https://github.com/lagrangedao/go-computing-provider/assets/102578774/e3b3dadc-77c1-4dc0-843c-5b946e252b65)

### Install and config the Nginx
 -  Install `Nginx` service to the Server
```bash
sudo apt update
sudo apt install nginx
```
 -  Add a configuration for your Domain name
Assume your domain name is `*.example.com`
```
vi /etc/nginx/conf.d/example.conf
```

```bash
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
        listen 80;
        listen [::]:80;
        server_name *.example.com;                                           # need to your domain
        return 301 https://$host$request_uri;
        #client_max_body_size 1G;
}
server {
        listen 443 ssl;
        listen [::]:443 ssl;
        ssl_certificate  /etc/letsencrypt/live/example.com/fullchain.pem;     # need to config SSL certificate
        ssl_certificate_key  /etc/letsencrypt/live/example.com/privkey.pem;   # need to config SSL certificate

        server_name *.example.com;                                            # need to config your domain
        location / {
          proxy_pass http://127.0.0.1:<port>;  	# Need to configure the Intranet port corresponding to ingress-nginx-controller service port 80 
          proxy_set_header Host $http_host;
          proxy_set_header Upgrade $http_upgrade;
          proxy_set_header Connection $connection_upgrade;
       }
}
```

 - **Note:** 

	 - `server_name`: a generic domain name

	 - `ssl_certificate` and `ssl_certificate_key`: certificate for https.

	 - `proxy_pass`:  The port should be the Intranet port corresponding to `ingress-nginx-controller` service port 80

 - Reload the `Nginx` config
	```bash
	sudo nginx -s reload
	```
 - Map your "catch-all (wildcard) subdomain(*.example.com)" to a public IP address



### Install Hardware resource-exporter
 The `resource-exporter` plugin is developed to collect the node resource constantly, computing provider will report the resource to the Lagrange Auction Engine to match the space requirement. To get the computing task, every node in the cluster must install the plugin. You just need to run the following command:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  namespace: kube-system
  name: resource-exporter-ds
  labels:
    app: resource-exporter
spec:
  selector:
    matchLabels:
      app: resource-exporter
  template:
    metadata:
      labels:
        app: resource-exporter
    spec:
      containers:
      - name: resource-exporter
        image: filswan/resource-exporter:v11.2.5
        imagePullPolicy: IfNotPresent
EOF
```
If you have installed it correctly, you can see the result shown in the figure by the command:
`kubectl get po -n kube-system`

![7](https://github.com/lagrangedao/go-computing-provider/assets/102578774/38b0e15f-5ff9-4edc-a313-d0f6f4a0bda8)

### Install Redis service
 - Install the `redis-server`
```bash
sudo apt update
sudo apt install redis-server
```

 - Run Redis service:

```bash
systemctl start redis-server.service
```

## Build and config the Computing Provider

 - Build the Computing Provider 

	Firstly, clone the code to your local:
```bash
git clone https://github.com/lagrangedao/go-computing-provider.git
cd go-computing-provider
git checkout v0.3.0
```

Then build the Computing provider follow the below steps:

```bash
make clean && make
make install
```
 - Update Configuration 
The computing provider's configuration sample locate in `./go-computing-provider/config.toml.sample`

```
cp config.toml.sample config.toml
```

Edit the necessary configuration files according to your deployment requirements. These files may include settings for the computing-provider components, container runtime, Kubernetes, and other services.

```toml
[API]
Port = 8085                                     # The port number that the web server listens on
MultiAddress = "/ip4/<public_ip>/tcp/<port>"    # The multiAddress for libp2p
Domain = ""                                     # The domain name

RedisUrl = "redis://127.0.0.1:6379"           # The redis server address
RedisPassword = ""                            # The redis server access password

[LOG]
CrtFile = "/YOUR_DOMAIN_NAME_CRT_PATH/server.crt"	# Your domain name SSL .crt file path
KeyFile = "/YOUR_DOMAIN_NAME_KEY_PATH/server.key"   	# Your domain name SSL .key file path

[LAG]
ServerUrl = "https://api.lagrangedao.org"     # The lagrangedao.org API address
AccessToken = ""                              # Lagrange access token, acquired from “https://lagrangedao.org  -> setting -> Access Tokens -> New token”


[MCS]
ApiKey = ""                                   # Acquired from "https://www.multichain.storage" -> setting -> Create API Key
BucketName = ""                               # Acquired from "https://www.multichain.storage" -> bucket -> Add Bucket
Network = "polygon.mainnet"                   # polygon.mainnet for mainnet, polygon.mumbai for testnet
FileCachePath = "/tmp"                        # Cache directory of job data

[Registry]                                    
ServerAddress = ""                            # The docker container image registry address, if only a single node, you can ignore
UserName = ""                                 # The login username, if only a single node, you can ignore
Password = ""                                 # The login password, if only a single node, you can ignore
```
## Install AI Inference Dependency
It is necessary for Computing Provider to deploy the  AI inference endpoint. But if you do not want to support the feature, you can skip it.
```bash
export CP_PATH=xxx
./install.sh
```


## Start the Computing Provider
You can run `computing-provider` using the following command
```bash
export CP_PATH=xxx
nohup computing-provider run >> cp.log 2>&1 & 
```

## CLI of Computing Provider
* Check the current list of tasks running on CP, display detailed information for tasks using `-v`
```
computing-provider task list 
```
* Retrieve detailed information for a specific task using `space_uuid`
```
computing-provider task get [space_uuid]
```
* Delete task by `space_uuid`
```
computing-provider task delete [space_uuid]
```

## Getting Help

For usage questions or issues reach out to the Swan team either in the [Discord channel](https://discord.gg/3uQUWzaS7U) or open a new issue here on GitHub.

## License

Apache
