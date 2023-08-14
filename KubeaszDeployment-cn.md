## 准备环境

### 安装golang 1.19.7
```
wget -c https://golang.org/dl/go1.19.7.linux-amd64.tar.gz -O - | sudo tar -xz -C /usr/local
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc && source ~/.bashrc
```

### 安装k8s环境

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

