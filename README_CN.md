# Computing Provider部署文档

## Computing Provider

[![Discord](https://img.shields.io/discord/770382203782692945?label=Discord\&logo=Discord)](https://discord.gg/Jd2BFSVCKw) [![Twitter Follow](https://img.shields.io/twitter/follow/swan\_chain)](https://twitter.com/swan\_chain) [![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg)](https://github.com/RichardLitt/standard-readme)

Computing Provider是通过提供计算资源（如处理能力（CPU 和 GPU）、内存、存储和带宽）参与分散式计算网络的个人或组织。它们的主要角色是执行用户在 Lagrange 平台上发布的任务。

* 0.[准备工作](#准备工作) 
* 1.[安装 Kubernetes](#安装Kubernetes)
  * [1.1 安装Container Runtime Environment](#安装Container-Runtime-Environment)
  * [1.2 设置 Docker 仓库（可选）](#可选-设置Docker仓库)
  * [1.3 创建 Kubernetes 集群](#创建Kubernetes集群)
  * [1.4 安装网络插件](#安装网络插件)
  * [1.5 安装 NVIDIA 插件](#安装NVIDIA插件)
  * [1.6 安装 Ingress-nginx 控制器](#安装Ingress-nginx控制器)
* 2.[安装和配置 Nginx](#安装和配置Nginx)
* 3.[安装Hardware resource-exporter](#安装Hardware-resource-exporter)
* 4.[安装 Redis service](#安装Redis-service)
* 5.[部署和配置Computing Provider](#部署和配置Computing-Provider)
* 6.[安装 AI Inference 依赖项（可选）](#安装AI-Inference依赖项)
* 7.[启动Computing Provider](#启动Computing-Provider)
* 8.[Computing Provider的 CLI](#Computing-Provider的CLI)


## 准备工作

在部署 Computing Provider 之前，您需要满足以下资源条件：

* 拥有公共IP
* 拥有泛域名（\*.example.com）
* 拥有SSL证书
* `Go` 版本必须是1.19+，您可以参考以下步骤：

```bash
wget -c https://golang.org/dl/go1.19.7.linux-amd64.tar.gz -O - | sudo tar -xz -C /usr/local

echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc && source ~/.bashrc
```

## 安装Kubernetes

Kubernetes版本应为`v1.24.0+`

### 安装Container Runtime Environment

如果您计划运行Kubernetes集群，您需要在集群中的每个节点上安装`Container Runtime Environment`，以便Pod可以运行，详情请参考[这里](https://kubernetes.io/docs/setup/production-environment/container-runtimes/)。您只需选择一种选项来安装`Container Runtime Environment`。

**选项1：安装`Docker`和`cri-dockerd`（推荐）**

要安装`Docker容器运行时`和`cri-dockerd`，请按照以下步骤操作：

* 安装`Docker`：
  * 请参考[此处](https://docs.docker.com/engine/install/)的官方文档。
* 安装`cri-dockerd`：
  * `cri-dockerd`是Docker的CRI（Container Runtime Interface）实现。您可以按照[这里](https://github.com/Mirantis/cri-dockerd)的说明进行安装。

**选项2：安装`Containerd`**

`Containerd`是一种符合行业标准的容器运行时(Container Runtime)，可用作Docker的替代方案。要在系统上安装`containerd`，请按照[containerd入门](https://github.com/containerd/containerd/blob/main/docs/getting-started.md)上的说明操作。

### 可选-设置Docker仓库

**如果您使用Docker且只有一个节点，则可以跳过此步骤**。

如果您已部署了具有多个节点的Kubernetes集群，建议设置一个**私有Docker仓库**，以允许其他节点在内网中快速拉取镜像。

* 在您的Docker服务器上创建一个目录`/docker_repo`。它将被挂载到仓库上，作为我们的Docker仓库的持久存储。

```bash
sudo mkdir /docker_repo
sudo chmod -R 777 /docker_repo
```

* 启动Docker仓库：

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

* 将仓库添加到节点

如果您已安装了`Docker`和`cri-dockerd`（**选项1**），您可以更新每个节点的配置：

```bash
sudo vi /etc/docker/daemon.json
```

```
## 添加以下配置
"insecure-registries": ["<Your_registry_server_IP>:5000"]
```

然后重新启动Docker服务

```bash
sudo systemctl restart docker
```

如果您已安装了`containerd`（**选项2**），您可以更新每个节点的配置：

```bash
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."<Your_registry_server_IP>:5000"]
      endpoint = ["http://<Your_registry_server_IP>:5000"]

[plugins."io.containerd.grpc.v1.cri".registry.configs]
  [plugins."io.containerd.grpc.v1.cri".registry.configs."<Your_registry_server_IP>:5000".tls]
      insecure_skip_verify = true                                                               
```

然后重新启动`containerd`服务

```bash
sudo systemctl restart containerd
```

**\<Your\_registry\_server\_IP>:** 您的仓库的内网IP地址。

最后，您可以通过以下命令检查安装情况：

```bash
docker system info
```

![2](https://github.com/lagrangedao/go-computing-provider/assets/102578774/4cfc1981-3fca-415c-948f-86c496915cff)

### 创建Kubernetes集群

要创建Kubernetes集群，您可以使用`kubeadm`等容器管理工具。可以按照以下步骤进行操作：

* [安装kubeadm工具箱](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/)。
* [使用kubeadm创建Kubernetes集群](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/)。

### 安装网络插件

Calico是一个开源的**容器网络和网络安全解决方案**，适用于容器、虚拟机和本机主机工作负载。Calico支持多种平台，包括**Kubernetes**、OpenShift、Mirantis Kubernetes Engine（MKE）、OpenStack和裸金属服务。

要安装Calico，您可以按照以下步骤进行操作，更多信息可以在[这里](https://docs.tigera.io/calico/3.25/getting-started/kubernetes/quickstart)找到。

**步骤1**：安装Tigera Calico operator和资源定义

```bash
kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.1/manifests/tigera-operator.yaml
```

**步骤2**：通过创建必要的资源来安装Calico

```bash
kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.1/manifests/custom-resources.yaml
```

**步骤3**：使用以下命令确认所有的Pod是否都在运行

```bash
watch kubectl get pods -n calico-system
```

**步骤4**：删除控制平面上的污点，以便可以在其上调度Pod。

```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
kubectl taint nodes --all node-role.kubernetes.io/master-
```

如果安装正确，您可以通过`kubectl get po -A`命令看到结果。

![3](https://github.com/lagrangedao/go-computing-provider/assets/102578774/91ef353f-72af-41b2-82e8-061b92bfb999)

**注意：**

* 如果您是单主机Kubernetes集群，请记得删除污点标记，否则任务无法调度到它。

```bash
kubectl taint node ${nodeName}  node-role.kubernetes.io/control-plane:NoSchedule-
```

### 安装NVIDIA插件

如果您的Computing Provider希望提供GPU资源，则应安装NVIDIA插件，请按照以下步骤操作：

* [安装NVIDIA驱动程序](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#nvidia-drivers)。

> 建议NVIDIA Linux驱动程序版本应为470.xx+

* [为Kubernetes安装NVIDIA设备插件](https://github.com/NVIDIA/k8s-device-plugin#quick-start)。

如果您已经正确安装，可以通过以下命令查看结果 `kubectl get po -n kube-system`

![4](https://github.com/lagrangedao/go-computing-provider/assets/102578774/8209c589-d561-43ad-adea-5ecb52618909)

### 安装Ingress-nginx控制器

`ingress-nginx`是用于Kubernetes的Ingress控制器，使用`NGINX`作为反向代理和负载均衡器。您可以运行以下命令进行安装：

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.7.1/deploy/static/provider/cloud/deploy.yaml
```

如果您已经正确安装，可以通过以下命令查看结果：

* 运行 `kubectl get po -n ingress-nginx`

![5](https://github.com/lagrangedao/go-computing-provider/assets/102578774/f3c0585a-df19-4971-91fe-d03365f4edee)

* 运行 `kubectl get svc -n ingress-nginx`

![6](https://github.com/lagrangedao/go-computing-provider/assets/102578774/e3b3dadc-77c1-4dc0-843c-5b946e252b65)

## 安装和配置Nginx

安装`Nginx`服务到服务器

```bash
sudo apt update
sudo apt install nginx
```

为您的域名添加配置 假设您的域名是`*.example.com`

```bash
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
        server_name *.example.com;                                           # 需要根据您的域名更改
        return 301 https://$host$request_uri;
        #client_max_body_size 1G;
}
server {
        listen 443 ssl;
        listen [::]:443 ssl;
        ssl_certificate  /etc/letsencrypt/live/example.com/fullchain.pem;     # 需要配置SSL证书
        ssl_certificate_key  /etc/letsencrypt/live/example.com/privkey.pem;   # 需要配置SSL证书

        server_name *.example.com;                                            # 需要配置您的域名
        location / {
          proxy_pass http://127.0.0.1:<port>;  	# 需要配置与ingress-nginx-controller服务端口80相对应的内网端口
          proxy_set_header Host $http_host;
          proxy_set_header Upgrade $http_upgrade;
          proxy_set_header Connection $connection_upgrade;
       }
}
```

**注意：**

* `server_name`: 通用域名
* `ssl_certificate` 和 `ssl_certificate_key`: 用于https的证书。
* `proxy_pass`: 端口应该是与`ingress-nginx-controller`服务端口80相对应的内网端口

重新加载`Nginx`配置

```bash
sudo nginx -s reload
```

将“catch-all（通配符）子域名（\*.example.com）”映射到公共IP地址

## 安装Hardware resource-exporter

`resource-exporter`插件是为了持续收集节点资源，Computing Provider将资源报告给Lagrange Auction Engine以匹配空间需求。要获取计算任务，集群中的每个节点都必须安装该插件。只需运行以下命令：

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

如果您已经正确安装，可以通过以下命令查看结果： `kubectl get po -n kube-system`

![7](https://github.com/lagrangedao/go-computing-provider/assets/102578774/38b0e15f-5ff9-4edc-a313-d0f6f4a0bda8)

## 安装Redis service

安装`redis-server`

```bash
sudo apt update
sudo apt install redis-server
```

运行Redis服务：

```bash
systemctl start redis-server.service
```

## 部署和配置Computing Provider

### 5.1 部署Computing Provider

首先，将代码克隆到本地：

```bash
git clone https://github.com/lagrangedao/go-computing-provider.git
cd go-computing-provider
git checkout v0.3.0
```

然后按照以下步骤部署Computing Provider：

```bash
make clean && make
make install
```

### 5.2 更新配置

Computing Provider的配置示例位于 `./go-computing-provider/config.toml.sample`

```
cp config.toml.sample config.toml
```

根据您的部署要求编辑必要的配置文件。这些文件可能包括Computing Provider组件、容器运行时、Kubernetes和其他服务的设置。

```toml
[API]
Port = 8085                                     # Web服务器监听的端口号
MultiAddress = "/ip4/<public_ip>/tcp/<port>"    # 用于libp2p的multiAddress
Domain = ""                                     # 域名

RedisUrl = "redis://127.0.0.1:6379"           # Redis服务器地址
RedisPassword = ""                            # Redis服务器访问密码

[LOG]
CrtFile = "/YOUR_DOMAIN_NAME_CRT_PATH/server.crt"	# 您的域名SSL .crt文件路径
KeyFile = "/YOUR_DOMAIN_NAME_KEY_PATH/server.key"   	# 您的域名SSL .key文件路径

[LAG]
ServerUrl = "https://api.lagrangedao.org"     # Lagrange的API地址
AccessToken = ""                              # Lagrange访问令牌，从“(https://lagrangedao.org  -> setting -> Access Tokens -> New token)”获取

[MCS]
ApiKey = ""                                   # 从"https://www.multichain.storage" -> setting -> Create API Key 获取
BucketName = ""                               # 从"https://www.multichain.storage" -> bucket -> Add Bucket 获取
Network = "polygon.mainnet"                   # 主网使用polygon.mainnet，测试网使用polygon.mumbai
FileCachePath = "/tmp"                        # 任务数据的缓存目录

[Registry]                                    
ServerAddress = ""                            # Docker容器镜像注册表地址，如果只有一个节点，可以忽略
UserName = ""                                 # 登录用户名，如果只有一个节点，可以忽略
Password = ""                                 # 登录密码，如果只有一个节点，可以忽略
```

## 安装AI Inference依赖项

对于计算提供者来说，部署AI inference 端点是必要的。但如果您不想支持此功能，可以跳过它。

```bash
export CP_PATH=xxx
./install.sh
```

## 启动Computing Provider

您可以使用以下命令运行 `computing-provider`

```bash
export CP_PATH=xxx
nohup computing-provider run >> cp.log 2>&1 & 
```

## Computing Provider的CLI

检查CP上当前运行的任务列表，使用 `-v` 显示任务的详细信息

```
computing-provider task list 
```

通过 `space_uuid` 获取特定任务的详细信息

```
computing-provider task get [space_uuid]
```

通过 `space_uuid` 删除任务

```
computing-provider task delete [space_uuid]
```

### 获取帮助

如有使用问题或问题，请通过[Discord频道](https://discord.gg/3uQUWzaS7U)与Swan团队联系，或在GitHub上打开新问题。

### 许可证

Apache


