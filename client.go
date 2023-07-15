package main

import (
	"context"
	"net"
	"regexp"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	*redis.Client

	Cluster bool
	Network string
	Addr    string
}

type ClusterClient struct {
	*redis.ClusterClient
}

type UnifiedClient struct {
	Single    *Client
	Cluster   *ClusterClient
	IsCluster bool
}

var notClusterPattern = regexp.MustCompile(`cluster_enabled:\s*0`)

func NewClient(options *redis.Options) *Client {
	return WrapClient(redis.NewClient(options))
}

func WrapClient(client *redis.Client) *Client {
	result := Client{
		Client: client,
	}
	result.AddHook(&result)
	return &result
}

func (c *Client) IsCluster(ctx context.Context) (bool, error) {
	info, err := c.Info(ctx, "cluster").Result()
	if err != nil {
		return false, err
	}
	return !notClusterPattern.MatchString(info), nil
}

func (c *Client) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		c.Network = network
		c.Addr = addr
		return next(ctx, network, addr)
	}
}

func (c *Client) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		return next(ctx, cmd)
	}
}

func (c Client) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(ctx, cmds)
	}
}

func NewClusterClient(options *redis.ClusterOptions) *ClusterClient {
	return &ClusterClient{
		ClusterClient: redis.NewClusterClient(options),
	}
}

func (c *ClusterClient) ForEachShard(ctx context.Context, fn func(ctx context.Context, client *Client) error) error {
	return c.ClusterClient.ForEachShard(ctx, func(ctx context.Context, client *redis.Client) error {
		return fn(ctx, WrapClient(client))
	})
}

func (c *ClusterClient) ForEachMaster(ctx context.Context, fn func(ctx context.Context, client *Client) error) error {
	return c.ClusterClient.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
		return fn(ctx, WrapClient(client))
	})
}

func (c *ClusterClient) ForEachSlave(ctx context.Context, fn func(ctx context.Context, client *Client) error) error {
	return c.ClusterClient.ForEachSlave(ctx, func(ctx context.Context, client *redis.Client) error {
		return fn(ctx, WrapClient(client))
	})
}

func NewUnifiedClient(ctx context.Context, addr string) (UnifiedClient, error) {
	client := NewClient(&redis.Options{
		Addr: addr,
	})

	isCluster, err := client.IsCluster(ctx)
	if err != nil {
		return UnifiedClient{}, err
	}

	result := UnifiedClient{
		Single:    client,
		IsCluster: isCluster,
	}

	if isCluster {
		result.Cluster = NewClusterClient(
			&redis.ClusterOptions{
				Addrs: []string{addr},
			},
		)
	}

	return result, nil
}
