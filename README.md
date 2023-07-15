# redis-tool

## Install

`go install github.com/bcap/redis-tool` 

## Run

`redis-tool -h` 

There are docker images as well: `docker run bcap/redis-tool -h`

## What

This is a tool to iterate redis keys in an efficient and safe manner. Current commands are:

* `print`: Print keys that match a certain pattern
* `count`: Count keys that match a certain pattern
* `delete:`: Delete keys that match a certain pattern

The pattern follows the spec of the [SCAN redis command](https://redis.io/commands/scan/)

Currently the only command that causes mutations is the `delete` command

## Examples:

Counting all keys that match pattern `test-key-000000*800`:

```
% go run . count -a localhost:6379 -p 'test-key-000000*800'
2023/07/15 17:46:00 [localhost:6379] processed 10 keys (~13.58 keys/s)
10
```

Printing the keys from the example above:

```
% go run . print -a localhost:6379 -p 'test-key-000000*800'
2023/07/15 17:47:39 [localhost:6379] processed 10 keys (~13.76 keys/s)
test-key-0000000800
test-key-0000008800
test-key-0000007800
test-key-0000001800
test-key-0000005800
test-key-0000003800
test-key-0000002800
test-key-0000009800
test-key-0000004800
test-key-0000006800
```

Deleting those keys:

Before deletion the tool does a few **safety measures** like:
1. Counts the number of keys that will be deleted
2. Writes the key names to a file
3. Asks for user confirmation. The user needs to type the redis address passed as input to confirm deletion. Confirmation takes a few seconds to be asked for, forcing the user to slow down.
4. Logs deleted key names to a file

```
% go run . delete -a localhost:6379 -p 'test-key-000000*800'
2023/07/15 17:48:35 Counting keys for deletion
2023/07/15 17:48:35 [localhost:6379] processed 10 keys (~12.63 keys/s)
2023/07/15 17:48:35 warning Deleting an estimate of 10 keys with pattern test-key-000000*800 in localhost:6379.
2023/07/15 17:48:35 Check /var/folders/t1/qlq0sfmd49l77c8dbr169yl80000gn/T/listed-redis-keys-for-deletion-localhost:6379-601525474 for selected keys.
2023/07/15 17:48:35 Keys being deleted will be logged to /var/folders/t1/qlq0sfmd49l77c8dbr169yl80000gn/T/deleted-redis-keys-localhost:6379-583810150
2023/07/15 17:48:35 Waiting 5s before asking for user confirmation
Type localhost:6379 to confirm: localhost:6379
2023/07/15 17:49:07 [localhost:6379] processed 10 keys (~9.77 keys/s)
2023/07/15 17:49:07 Deleted 10 keys in 1s24ms
```

# Contributing

## redis test data
- `./run-test-redis-server.sh` to start a local redis server through docker. Redis data is persisted at `.redis-data`
- `./fill-test-redis-server.sh` will create 1M keys in the local redis in the format `test-key-0000000001 test-value-0000000001`

## development
- `go run .` to run locally
- `make build` will build the docker image
- `make build-multi-arch` will build both linux/arm64 and linux/amd64 images

## release
- `release.sh` will publish a new version
