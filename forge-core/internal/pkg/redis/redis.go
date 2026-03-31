package redis

import (
	"context"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, addr, password string) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	slog.Info("redis connected", "addr", addr)
	return client, nil
}
