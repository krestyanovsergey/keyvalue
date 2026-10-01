package keyvalue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Дедлайны считаются от реального времени, потому что Storage использует time.Now()
func futureDeadline() time.Time { return time.Now().Add(time.Hour) }
func pastDeadline() time.Time   { return time.Now().Add(-time.Hour) }

// newTestStorage создает Storage с интервалом сборщика в час,
// чтобы фоновая горутина не мешала тестам, которые вызывают cleanRound напрямую
func newTestStorage(t *testing.T, modify func(*Config)) *Storage {
	t.Helper()

	cfg := validConfig()
	cfg.CollectorInterval = time.Hour
	if modify != nil {
		modify(&cfg)
	}

	storage, err := NewStorage(cfg)
	require.NoError(t, err)
	t.Cleanup(storage.Stop)

	return storage
}

func bucketSize(bkt *bucket) int {
	bkt.mu.RLock()
	defer bkt.mu.RUnlock()
	return bkt.size
}

// nodeCount считает узлы во всех корзинах (безопасно при работающем сборщике)
func nodeCount(storage *Storage) int {
	total := 0
	for _, bkt := range storage.buckets {
		total += bucketSize(bkt)
	}
	return total
}

// fillBucket кладет в корзину заданное число просроченных и живых узлов
func fillBucket(bkt *bucket, prefix string, expiredN, aliveN int) {
	for i := range expiredN {
		bkt.put(fmt.Sprintf("%s-expired-%d", prefix, i), "v", pastDeadline())
	}
	for i := range aliveN {
		bkt.put(fmt.Sprintf("%s-alive-%d", prefix, i), "v", futureDeadline())
	}
}

func TestNewStorage(t *testing.T) {
	cfg := validConfig()
	cfg.InitialBucketCount = 8

	storage, err := NewStorage(cfg)
	require.NoError(t, err)
	t.Cleanup(storage.Stop)

	assert.Len(t, storage.buckets, 8)
	assert.Equal(t, cfg.CollectorInterval, storage.collectorInterval)
	assert.Equal(t, cfg.CollectorMaxIterations, storage.collectorMaxIterations)
	assert.Equal(t, cfg.CollectorBatchRatio, storage.collectorBatchRatio)
	assert.Equal(t, cfg.CollectorThresholdRatio, storage.collectorThresholdRatio)
	assert.NotNil(t, storage.cancel)
}

func TestNewStorage_InvalidConfig(t *testing.T) {
	cfg := validConfig()
	cfg.InitialBucketCount = 0

	storage, err := NewStorage(cfg)

	assert.ErrorContains(t, err, "InitialBucketCount")
	assert.Nil(t, storage)
}

func TestStorage_GetBucketByKey(t *testing.T) {
	storage := newTestStorage(t, func(c *Config) { c.InitialBucketCount = 4 })

	// один и тот же ключ всегда попадает в одну и ту же корзину
	assert.Same(t, storage.getBucketByKey("a"), storage.getBucketByKey("a"))

	// ключи распределяются по всем корзинам, и корзина всегда принадлежит хранилищу
	used := make(map[*bucket]bool)
	for i := range 1000 {
		bkt := storage.getBucketByKey(fmt.Sprintf("key-%d", i))
		assert.Contains(t, storage.buckets, bkt)
		used[bkt] = true
	}
	assert.Len(t, used, 4)
}

func TestStorage_PutGet(t *testing.T) {
	storage := newTestStorage(t, nil)

	storage.Put("a", "1", time.Hour)
	storage.Put("b", "2", time.Hour)

	val, ok := storage.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "1", val)

	val, ok = storage.Get("b")
	assert.True(t, ok)
	assert.Equal(t, "2", val)
}

func TestStorage_Put_Overwrite(t *testing.T) {
	storage := newTestStorage(t, nil)

	storage.Put("a", "old", time.Hour)
	storage.Put("a", "new", time.Hour)

	val, ok := storage.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "new", val)
	assert.Equal(t, 1, nodeCount(storage))
}

func TestStorage_Put_InvalidTTLIgnored(t *testing.T) {
	storage := newTestStorage(t, nil)
	storage.Put("existing", "old", time.Hour)

	for _, ttl := range []time.Duration{0, -time.Second} {
		storage.Put("new", "v", ttl)
		storage.Put("existing", "new", ttl)
	}

	// несуществующий ключ не появился
	_, ok := storage.Get("new")
	assert.False(t, ok)

	// существующая пара осталась нетронутой
	val, ok := storage.Get("existing")
	assert.True(t, ok)
	assert.Equal(t, "old", val)
	assert.Equal(t, 1, nodeCount(storage))
}

func TestStorage_Get_Missing(t *testing.T) {
	storage := newTestStorage(t, nil)

	val, ok := storage.Get("missing")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestStorage_Get_Expired(t *testing.T) {
	storage := newTestStorage(t, nil)
	storage.getBucketByKey("a").put("a", "1", pastDeadline())

	val, ok := storage.Get("a")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestStorage_Delete(t *testing.T) {
	storage := newTestStorage(t, nil)
	storage.Put("a", "1", time.Hour)
	storage.Put("b", "2", time.Hour)

	storage.Delete("a")

	_, ok := storage.Get("a")
	assert.False(t, ok)

	val, ok := storage.Get("b")
	assert.True(t, ok)
	assert.Equal(t, "2", val)
}

func TestStorage_Delete_MissingAndExpired(t *testing.T) {
	storage := newTestStorage(t, nil)
	storage.getBucketByKey("expired").put("expired", "1", pastDeadline())

	assert.NotPanics(t, func() {
		storage.Delete("missing")
		storage.Delete("expired")
	})
	assert.Equal(t, 0, nodeCount(storage))
}

func TestStorage_CleanRound(t *testing.T) {
	tests := []struct {
		name       string
		buckets    int
		batchRatio float64
		maxIter    int
		threshold  float64
		// на каждую корзину {просроченных, живых}
		fill       [][2]int
		wantSizes  []int
		wantCursor int
	}{
		{
			name:    "полный круг очищает все корзины",
			buckets: 4, batchRatio: 1, maxIter: 1, threshold: 1,
			fill:       [][2]int{{2, 1}, {2, 1}, {2, 1}, {2, 1}},
			wantSizes:  []int{1, 1, 1, 1},
			wantCursor: 0,
		},
		{
			name:    "батч меньше круга",
			buckets: 4, batchRatio: 0.5, maxIter: 1, threshold: 1,
			fill:       [][2]int{{1, 0}, {1, 0}, {1, 0}, {1, 0}},
			wantSizes:  []int{0, 0, 1, 1},
			wantCursor: 2,
		},
		{
			name:    "минимум одна корзина за итерацию",
			buckets: 4, batchRatio: 0.1, maxIter: 1, threshold: 1,
			fill:       [][2]int{{1, 0}, {1, 0}, {1, 0}, {1, 0}},
			wantSizes:  []int{0, 1, 1, 1},
			wantCursor: 1,
		},
		{
			name:    "останавливается, если мусора меньше порога",
			buckets: 4, batchRatio: 0.25, maxIter: 3, threshold: 0.5,
			fill:       [][2]int{{1, 3}, {2, 0}, {0, 0}, {0, 0}},
			wantSizes:  []int{3, 2, 0, 0},
			wantCursor: 1,
		},
		{
			name:    "останавливается на пустой корзине",
			buckets: 4, batchRatio: 0.25, maxIter: 3, threshold: 0.5,
			fill:       [][2]int{{0, 0}, {2, 0}, {0, 0}, {0, 0}},
			wantSizes:  []int{0, 2, 0, 0},
			wantCursor: 1,
		},
		{
			name:    "продолжает, если доля мусора равна порогу",
			buckets: 4, batchRatio: 0.25, maxIter: 2, threshold: 0.5,
			fill:       [][2]int{{1, 1}, {2, 0}, {2, 0}, {0, 0}},
			wantSizes:  []int{1, 0, 2, 0},
			wantCursor: 2,
		},
		{
			name:    "ограничена максимальным числом итераций",
			buckets: 4, batchRatio: 0.25, maxIter: 2, threshold: 0.5,
			fill:       [][2]int{{1, 0}, {1, 0}, {1, 0}, {0, 0}},
			wantSizes:  []int{0, 0, 1, 0},
			wantCursor: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := newTestStorage(t, func(c *Config) {
				c.InitialBucketCount = tt.buckets
				c.CollectorBatchRatio = tt.batchRatio
				c.CollectorMaxIterations = tt.maxIter
				c.CollectorThresholdRatio = tt.threshold
			})
			for i, f := range tt.fill {
				fillBucket(storage.buckets[i], fmt.Sprintf("b%d", i), f[0], f[1])
			}

			storage.cleanRound()

			for i, want := range tt.wantSizes {
				assert.Equal(t, want, bucketSize(storage.buckets[i]), "корзина %d", i)
			}
			assert.Equal(t, tt.wantCursor, storage.collectorCursor)

			// живые пары никогда не удаляются
			for i, f := range tt.fill {
				for j := range f[1] {
					key := fmt.Sprintf("b%d-alive-%d", i, j)
					_, ok := storage.buckets[i].get(key, time.Now())
					assert.True(t, ok, key)
				}
			}
		})
	}
}

func TestStorage_CleanRound_CursorWraps(t *testing.T) {
	storage := newTestStorage(t, func(c *Config) {
		c.InitialBucketCount = 4
		c.CollectorBatchRatio = 0.5
		c.CollectorMaxIterations = 1
		c.CollectorThresholdRatio = 1
	})
	for i, bkt := range storage.buckets {
		fillBucket(bkt, fmt.Sprintf("b%d", i), 1, 0)
	}

	storage.cleanRound()
	assert.Equal(t, 2, storage.collectorCursor)
	assert.Equal(t, 2, nodeCount(storage))

	// второй раунд доходит до конца и возвращается к началу
	storage.cleanRound()
	assert.Equal(t, 0, storage.collectorCursor)
	assert.Equal(t, 0, nodeCount(storage))
}

func TestStorage_Collector_CleansInBackground(t *testing.T) {
	storage := newTestStorage(t, func(c *Config) {
		c.InitialBucketCount = 4
		c.CollectorInterval = 5 * time.Millisecond
		c.CollectorBatchRatio = 1
		c.CollectorMaxIterations = 1
	})
	for i, bkt := range storage.buckets {
		fillBucket(bkt, fmt.Sprintf("b%d", i), 3, 0)
	}

	assert.Eventually(t, func() bool {
		return nodeCount(storage) == 0
	}, 2*time.Second, 5*time.Millisecond)
}

func TestStorage_Stop(t *testing.T) {
	storage := newTestStorage(t, func(c *Config) {
		c.InitialBucketCount = 4
		c.CollectorInterval = 5 * time.Millisecond
		c.CollectorBatchRatio = 1
		c.CollectorMaxIterations = 1
	})

	done := make(chan struct{})
	go func() {
		storage.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop не завершился")
	}

	// повторный Stop безопасен
	assert.NotPanics(t, storage.Stop)

	// после Stop сборщик больше не работает
	fillBucket(storage.buckets[0], "late", 3, 0)
	assert.Never(t, func() bool {
		return nodeCount(storage) == 0
	}, 50*time.Millisecond, 5*time.Millisecond)

	// но хранилище остается рабочим
	storage.Put("a", "1", time.Hour)
	val, ok := storage.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "1", val)
}

func TestStorage_Concurrent(t *testing.T) {
	storage := newTestStorage(t, func(c *Config) {
		c.InitialBucketCount = 4
		c.CollectorInterval = time.Millisecond
		c.CollectorBatchRatio = 0.5
		c.CollectorMaxIterations = 2
	})

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i%10)
			storage.Put(key, "v", time.Hour)
			storage.Put(key+"-short", "v", time.Millisecond)
			storage.Get(key)
			storage.Delete(key)
		}()
	}
	wg.Wait()
	storage.Stop()

	for _, bkt := range storage.buckets {
		assert.Len(t, keys(bkt), bkt.size)
	}
}
