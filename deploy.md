### 1. Preparation requirements

*  Install k8s, please refer to https://docs.nvidia.com/datacenter/cloud-native/kubernetes/install-k8s.html

* Use docke to Install redis,  need to use custom configuration file.  Need to modify:

  `bind、protected-mode、notify-keyspace-events`

```bash
 bind 0.0.0.0
 protected-mode no
 notify-keyspace-events Ex
```

The following is the complete configuration file content of redis, which can directly replace the content of the local redis.conf：

```bash
# Redis configuration file example.

################################## NETWORK #####################################
bind 0.0.0.0
protected-mode no
port 6379
tcp-backlog 511
timeout 0
tcp-keepalive 300

################################# GENERAL #####################################
daemonize no
pidfile /var/run/redis_6379.pid
loglevel notice
logfile ""
databases 16
always-show-logo no
set-proc-title yes
proc-title-template "{title} {listen-addr} {server-mode}"

################################ SNAPSHOTTING  ################################
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb
rdb-del-sync-files no
dir ./

################################# REPLICATION #################################
replica-serve-stale-data yes
replica-read-only yes
repl-diskless-sync yes
repl-diskless-sync-delay 5
repl-diskless-sync-max-replicas 0
repl-diskless-load disabled
repl-disable-tcp-nodelay no
replica-priority 100
acllog-max-len 128

############################# LAZY FREEING ####################################
lazyfree-lazy-eviction no
lazyfree-lazy-expire no
lazyfree-lazy-server-del no
replica-lazy-flush no
lazyfree-lazy-user-del no
lazyfree-lazy-user-flush no

################################ THREADED I/O #################################
oom-score-adj no
oom-score-adj-values 0 200 800
disable-thp yes

############################## APPEND ONLY MODE ###############################
appendonly no
appendfilename "appendonly.aof"
appenddirname "appendonlydir"
appendfsync everysec
no-appendfsync-on-rewrite no

auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
aof-load-truncated yes
aof-use-rdb-preamble yes
aof-timestamp-enabled no

slowlog-log-slower-than 10000
slowlog-max-len 128
latency-monitor-threshold 0

############################# EVENT NOTIFICATION ##############################
notify-keyspace-events Ex


hash-max-listpack-entries 512
hash-max-listpack-value 64
list-max-listpack-size -2
list-compress-depth 0
set-max-intset-entries 512
zset-max-listpack-entries 128
zset-max-listpack-value 64
hll-sparse-max-bytes 3000
stream-node-max-bytes 4096
stream-node-max-entries 100
activerehashing yes
client-output-buffer-limit normal 0 0 0
client-output-buffer-limit replica 256mb 64mb 60
client-output-buffer-limit pubsub 32mb 8mb 60
hz 10
dynamic-hz yes
aof-rewrite-incremental-fsync yes
rdb-save-incremental-fsync yes
jemalloc-bg-thread yes

```

* Run redis service:

```bash
docker run -v /youpath/conf:/usr/local/etc/redis -p6379:6379 -d --name myredis redis redis-server /usr/local/etc/redis/redis.conf
```

**/youpath/conf:**   The local directory path where redis.conf is placed

### 2.  Compile go-computing-provider

* Clone the repository:

```bash
git clone https://github.com/lagrangedao/go-computing-provider.git
cd go-computing-provider
```

* Install dependencies:

```bash
go mod tidy
```

* Complie code

```BASH
go build -o computing-provider main.go
```

* Create a config.toml file by copying the config.toml.sample file

```bash
cp config.toml.sample config.toml
```

* updating the values, `vi config.toml`

```bash
[API]
Port = 8085                                   # The port number that the web server listens on
MultiAddress = "/ip4/127.0.0.1/tcp/8085"      # The multiAddress for libp2p
PublicNetworkIp = ""                          # The public network ip

OPENAI_API_KEY = ""
RedisUrl = "redis://127.0.0.1:6379"           # The redis server address
RedisPassword = ""                            # The redis server access password

[LAD]
ServerUrl = "https://api.lagrangedao.org"     # The lagrangedao.org API address
AccessToken = ""                              # Access token applied by lagrangedao.org

[MCS]
ApiKey = ""                                   # The MCS API_KEY
AccessToken = ""                              # The MCS API_KEY
Bucket = ""                                   # The MCS bucket name
Network = "polygon.mainnet"                   # polygon.mainnet for mainnet, polygon.mumbai for testnet
FileCachePath = ""                            # Cache directory of job task raw data

[Registry]
ServerAddress = "https://hub.docker.com/"     # The docker container image registry address
UserName = ""                                 # The login username
Password = ""                                 # The login password
```

* The k8s master node's port numbers，` 32750-32755`, must be mapped to the port number of the IP machine specified by `PublicNetworkIp`
* Move  `computing-provider`  and `config.toml` to the upper directory:

```bash
mv computing-provider config.toml ../
```

### 3. Start computing-provider

* Create a directory for the `${FileCachePath}` configuration item:

```bash
mkdir ${FileCachePath}
```

* Start cp service:

```bash
nohup ./computing-provider >> cp.log 2>&1 & 
```

### 

