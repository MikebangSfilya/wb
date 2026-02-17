package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisRepository(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redis.Run(ctx, "docker.io/redis:7.2-alpine")
	require.NoError(t, err)

	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)

	natPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	r, err := New(ctx, host, natPort.Port(), "", 0, nil)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	t.Run("Set and Get value", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		key := "user:1"
		value := TestStruct{Name: "John", Age: 30}

		err := r.Set(ctx, key, value, time.Minute)
		require.NoError(t, err)

		var result TestStruct
		err = r.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("Get missing key returns ErrCacheMiss", func(t *testing.T) {
		var result string
		err := r.Get(ctx, "non-existent-key", &result)

		assert.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("Delete key", func(t *testing.T) {
		key := "to-delete"
		value := "data"

		err := r.Set(ctx, key, value, time.Minute)
		require.NoError(t, err)

		err = r.Delete(ctx, key)
		assert.NoError(t, err)

		var result string
		err = r.Get(ctx, key, &result)
		assert.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("Value expires after TTL", func(t *testing.T) {
		key := "short-lived-key"
		value := "temp-data"
		ttl := time.Millisecond * 200

		err := r.Set(ctx, key, value, ttl)
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 300)

		var result string
		err = r.Get(ctx, key, &result)
		assert.ErrorIs(t, err, ErrCacheMiss)
	})
}
