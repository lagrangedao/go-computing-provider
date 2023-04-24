# go-computing-provider

A Swan service provider application using the Gin web framework.

## Prerequisites

- Go 1.16 or higher

## Installation

1. Clone the repository:

```bash
git clone https://github.com/lagrangedao/go-computing-provider.git
cd go-computing-provider
```

2. Install dependencies:

```shell
go mod tidy
```

3. Create a .env file by copying the .env_sample file and updating the values.

```shell
cp .env_sample .env
```
4. start a redis stack

```shell
docker run -d --name redis-stack-server -p 6379:6379 redis/redis-stack-server:latest
```
## Running the Application

To run the application, use the go run command:

```shell
go run main.go
```

The server will start listening on 0.0.0.0:8085.

### License

This project is licensed under the MIT License - see the LICENSE file for details.
