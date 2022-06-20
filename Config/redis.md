# Redis

> Guided by the offcial website: https://redis.io/docs/getting-started/installation/install-redis-on-mac-os/

## Prerequisites

First, make sure you have Homebrew installed. For the terminal, run:

```bash
$ brew --version
```

If this command fails, you can follow this post: https://github.com/Eminem-x/Learning/blob/main/Config/brew.md

## Installation

From the terminal, run:

```bash
brew install redis
```

This will install Redis on your system.

## Starting and stopping Redis in the foreground

To test your Redis installation, you can run the `redis-server` executable from the command line:

`redis-server`

If successful, you'll see the startup logs for Redis, and Redis will ne running in the foreground.

To stop Redis, enter `Ctrl-C`.

## Connect to Redis

Once Redis is running, you can test it by running `redis-cli`:

`redis-cli`

This will open the Redis REPL. Try running some conmands:

```bash
127.0.0.1:6379> lpush demos redis-macOS-demo
OK
127.0.0.1:6379> rpop demos
"redis-macOS-demo"
```