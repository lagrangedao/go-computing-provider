## 安装golang 1.19.7
```
wget -c https://golang.org/dl/go1.19.7.linux-amd64.tar.gz -O - | sudo tar -xz -C /usr/local
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc && source ~/.bashrc
```

## 安装k8s环境

本流程使用 [kubeasz 3.5.0](https://github.com/easzlab/kubeasz.git) 作为k8s部署工具链，kubeasz依赖于ansible部署基础k8s集群.

- 下载 [ezdown](https://github.com/easzlab/kubeasz/releases/download/3.5.0/ezdown)
```wget https://github.com/easzlab/kubeasz/releases/download/3.5.0/ezdown```
- 下载kubeasz代码、二进制以及容器镜像
  - 国内执行 ```./ezdown -D```
  - 海外执行 ```./ezdown -D -m standard```
- 容器运行kubeasz
```./ezdown -S```
- 使用默认配置安装aio集群
```docker exec -it kubeasz ezctl start-aio```
  - 如果安装失败，查看kubeasz镜像排除错误
  ```docker logs -f kubeasz```
  - 重新安装aio集群
  ```docker exec -it kubeasz ezctl setup default all```
- 安装成功后验证集群信息
  - ```source ~/.bashrc```
  - 验证集群版本 ```kubectl version```
  - 验证节点就绪 (Ready) 状态 ```kubectl get node```
  - 验证集群pod状态，默认已安装网络插件、coredns、metrics-server等 ```kubectl get pod -A```
  - 验证集群服务状态 ```kubectl get svc -A```
- 清除集群
  - 销毁集群 ```docker exec -it kubeasz ezctl destroy default```
  - 重启节点清理残留虚拟网卡、路由等信息

**上述命令行都在宿主机执行**

安装过程可能遇到的错误 `Error running ProxyServer` 解决方法
- 重新安装ipset ```apt install ipset```
- 重新安装k8s集群

**使用其他账户管理集群**

以上安装步骤在root账号下完成，如后续要使用其他账号进行集群管理，以testuser为例，可执行以下步骤：

```bash
cp  -r /root/.kube /home/testuser/
chown -R testuser:testuser /home/testuser/.kube
echo "exporter PATH=$PATH:/etc/kubeasz/bin/" >> /home/testuser/.bashrc
su testuser
source  $HOME/.bashrc
```

## 安装nvidia显卡驱动

```bash
wget https://developer.download.nvidia.com/compute/cuda/12.2.1/local_installers/cuda_12.2.1_535.86.10_linux.runsudo sh cuda_12.2.1_535.86.10_linux.run
```

## 安装NVIDIA Plugin

安装NVIDIA Plugin有以下前置要求：

NVIDIA drivers ~= 384.81

nvidia-docker >= 2.0 || nvidia-container-toolkit >= 1.7.0 (>= 1.11.0 to use integrated GPUs on Tegra-based systems)

nvidia-container-runtime configured as the default low-level runtime

Kubernetes version >= 1.10

### 安装 nvidia-container-toolkit

```bash
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/libnvidia-container.list
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
```

### 配置 containerd

使用[kubeasz 3.5.0](https://github.com/easzlab/kubeasz.git) 部署的k8s集群，容器运行时默认为containerd, 编辑以下配置文件设置nvidia-container-rumtime成为默认的low-level运行时，通常配置文件为/etc/containerd/config.toml。

```bash
version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "nvidia"

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
          privileged_without_host_devices = false
          runtime_engine = ""
          runtime_root = ""
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
            BinaryName = "/usr/bin/nvidia-container-runtime"
```

编辑完成后重启containerd:

```bash
sudo systemctl restart containerd
```

### 在kubernetes中开启GPU支持

在集群中GPU节点配置了以上选项后，可以部署以下daemonset开启gpu支持：

```bash
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.1/nvidia-device-plugin.yml
```

## 安装ingress-nginx controller

```kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.7.1/deploy/static/provider/cloud/deploy.yaml```

## 安装宿主机上的nginx服务

```
sudo apt update
sudo apt install nginx
```

### 从[letsencrypt](https://letsencrypt.osfipin.com/user-0408/order/apply)获取免费https证书

**注意：**```本证书有效期为3个月，3个月到期后需要续签，如果不想手动续签可以部署cert-manager服务自动续签，cert-manager服务部署流程不包含在本文档范畴```
- 申请证书时输入域名后需要将泛域名及包含根域选中 ```填写的域名需要在域名服务商提前购买```
- 输入域名后依次点击下一步，选择RSA加密算法与Let's Encrypt渠道后点击提交申请
- 申请后选择DNS验证方式进行验证 ```需前往域名服务商配置域名解析```
- 等待验证成功后即可下载证书

### 获取k8s集群内的ingress-nginx-controller端口

```kubectl get svc -A```

![image](https://github.com/kikakkz/go-computing-provider/assets/13128505/facee510-7a6b-4a14-8bd4-31da150b674a)

其中，80和443分别对应稍后的nginx配置文件中的服务端口。


### 配置宿主机上的nginx服务

我们的域名为```infcomputing.net```，域名配置文件存储在```/etc/nginx/conf.d/infcomputing.net```

```
map $http_upgrade $connection_upgrade {
  default upgrade;
  ''      close;
}

server {
  listen 80;
  listen [::]:80;
  server_name *.infcomputing.net;  // 这里修改为自己的域名
  return 301 https://$host$request_uri;
}

server {
 listen 443 ssl;
 listen [::]:443 ssl;
 ssl_certificate /etc/letsencrypt/live/infcomputing.net/fullchain.crt; // 这里修改为从letsencrypt申请的证书存储路径
 ssl_certificate_key /etc/letsencrypt/live/infcomputing.net/private.pem; // 这里修改为从letsencrypt申请的证书存储路径
 server_name *.infcomputing.net;  // 这里修改为自己的域名
 location / {
   proxy_pass http://127.0.0.1:31094;  // 这里修改为上一步骤获取的ingress-nginx-controller svc的端口
   proxy_set_header Host $http_host;
   proxy_set_header Upgrade $http_upgrade;
   proxy_set_header Connection $connection_upgrade;
 }
}

```

### 公网访问设置

将宿主机上的nginx 80和443端口映射到公网IP的80和443端口，并在域名服务商处设置dns解析到公网IP。可以通过telnet检查端口连通性：

```
telnet infcomputing.net 80
telnet infcomputing.net 443
```

## 安装resource-exporter

编辑 ```resource-exporter.yaml``` 文件，内容为

```
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
```

执行 ```kubectl apply -f resource-exporter.yaml```

## 安装redis-server

```
sudo apt update
sudo apt install redis-server
```

### 为redis-server添加密码

编辑 ```/etc/redis/redis.conf```，搜索 ```requirepass```，去掉注释，并将内容替换为自己的redis密码。

### 启动redis服务

```
sudo systemctl enable redis-server
sudo systemctl restart redis-server
```

## 编译和运行Lagrange Computing Provider

```
git clone https://github.com/lagrangedao/go-computing-provider.git
cd go-computing-provider
git checkout mars-testnet
go mod tidy
go build -o computing-provider main.go
cp config.toml.sample config.toml

```

### 修改配置文件

```
[API]
Port = 8085                                     # Computing Provider在运行机器上监听的端口
MultiAddress = "/ip4/115.42.169.230/tcp/8080"   # Computing Provider暴露到公网供peer连通的地址，这个地址里面的IP和端口需要映射到Computing Provider运行的主机和端口
Domain = "infcomputing.net"                     # The domain name
NodeName = "Crypto More Lagrange 1"             # The computing-provider node name

RedisUrl = "redis://127.0.0.1:6379"           # The redis server address
RedisPassword = "LagMyPassw0rd"               # The redis server access password

[LAG]
ServerUrl = "https://api.lagrangedao.org"     # The lagrangedao.org API address
AccessToken = "HgAdx1wCGY"                    # Lagrange access token, acquired from “https://lagrangedao.org  -> setting -> Access Tokens -> New token”

[MCS]
ApiKey = "5GQEjBewkNqDVRXLi6WWq5"                 # Acquired from "https://www.multichain.storage" -> setting -> Create API Key
AccessToken = "lDKNm1UEJClBBuyltWb8or0dLlRQeycx"  # Acquired from "https://www.multichain.storage" -> setting -> Create API Key
BucketName = "lagrange-mars-testnet"              # Acquired from "https://www.multichain.storage" -> bucket -> Add Bucket
Network = "polygon.mainnet"                       # 这里固定为polygon.mainnet
FileCachePath = "/mnt/nvme0n1p1/lagrange-mars-testnet"  # Cache directory of job data

[Registry]
ServerAddress = ""                            # The docker container image registry address, if only a single node, you can ignore
UserName = ""                                 # The login username, if only a single node, you can ignore
Password = ""                                 # The login password, if only a single node, you can ignore
```

**注意:**
```
1  端口映射不对不能联通，在[Provider Status](https://provider.lagrangedao.org/provider-status)看到节点的uptime将为0，当前5分钟统计一次
2  MCS的Network当前固定为polygon.mainnet
3  LAG和MCS的key官网流程都比较顺畅，因此我们没有专门额外写步骤
```

### 执行Computing Provider

建议创建service文件执行，或执行在tmux窗口中。

```./computing-provider```

### 检查运行状态

[Provider Status](https://provider.lagrangedao.org/provider-status)
