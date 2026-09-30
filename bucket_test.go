package keyvalue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// now - фиксированный "текущий" момент для всех тестов
var now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func alive() time.Time   { return now.Add(time.Hour) }
func expired() time.Time { return now.Add(-time.Hour) }

// keys возвращает ключи в порядке списка (от головы к хвосту)
func keys(bkt *bucket) []string {
	var res []string
	for cur := bkt.head; cur != nil; cur = cur.next {
		res = append(res, cur.key)
	}
	return res
}

// assertConsistent проверяет, что size совпадает с реальной длиной списка
func assertConsistent(t *testing.T, bkt *bucket) {
	t.Helper()
	assert.Len(t, keys(bkt), bkt.size)
}

// newFilledBucket кладет ключи по порядку; голова списка - последний ключ
func newFilledBucket(ks ...string) *bucket {
	bkt := newBucket()
	for _, k := range ks {
		bkt.put(k, "v-"+k, alive())
	}
	return bkt
}

func TestNewBucket(t *testing.T) {
	bkt := newBucket()

	assert.Nil(t, bkt.head)
	assert.Equal(t, 0, bkt.size)
}

func TestNewBucketSlice(t *testing.T) {
	slice := newBucketSlice(3)

	assert.Len(t, slice, 3)
	for i, bkt := range slice {
		assert.NotNil(t, bkt)
		for j := i + 1; j < len(slice); j++ {
			assert.NotSame(t, bkt, slice[j])
		}
	}

	assert.Empty(t, newBucketSlice(0))
}

func TestBucket_Put(t *testing.T) {
	bkt := newBucket()

	// создание: новый узел становится головой
	assert.True(t, bkt.put("a", "1", alive()))
	assert.True(t, bkt.put("b", "2", alive()))
	assert.Equal(t, []string{"b", "a"}, keys(bkt))
	assert.Equal(t, 2, bkt.size)

	// обновление существующего узла в хвосте: размер не меняется
	assert.False(t, bkt.put("a", "3", alive()))
	assert.Equal(t, 2, bkt.size)
	assert.Equal(t, []string{"b", "a"}, keys(bkt))

	val, ok := bkt.get("a", now)
	assert.True(t, ok)
	assert.Equal(t, "3", val)

	assertConsistent(t, bkt)
}

func TestBucket_Put_OverExpired(t *testing.T) {
	bkt := newBucket()
	bkt.put("a", "old", expired())

	// просроченный узел еще в списке, поэтому это обновление, а не создание
	created := bkt.put("a", "new", alive())

	assert.False(t, created)
	assert.Equal(t, 1, bkt.size)

	// обновление продлевает время жизни
	val, ok := bkt.get("a", now)
	assert.True(t, ok)
	assert.Equal(t, "new", val)
}

func TestBucket_Get(t *testing.T) {
	bkt := newBucket()

	// пустая корзина
	val, ok := bkt.get("a", now)
	assert.False(t, ok)
	assert.Empty(t, val)

	bkt = newFilledBucket("a", "b", "c")

	// голова, середина, хвост
	for _, k := range []string{"a", "b", "c"} {
		val, ok = bkt.get(k, now)
		assert.True(t, ok, k)
		assert.Equal(t, "v-"+k, val)
	}

	// такого ключа нет
	val, ok = bkt.get("missing", now)
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestBucket_Get_Expired(t *testing.T) {
	bkt := newBucket()
	bkt.put("a", "1", expired())

	val, ok := bkt.get("a", now)

	assert.False(t, ok)
	assert.Empty(t, val)
	// get не удаляет просроченные узлы
	assert.Equal(t, 1, bkt.size)
	assertConsistent(t, bkt)
}

func TestBucket_Get_TTLBoundary(t *testing.T) {
	deadline := now.Add(time.Minute)
	bkt := newBucket()
	bkt.put("a", "1", deadline)

	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"до дедлайна", deadline.Add(-time.Nanosecond), true},
		{"ровно в момент дедлайна", deadline, true},
		{"после дедлайна", deadline.Add(time.Nanosecond), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := bkt.get("a", tt.at)
			assert.Equal(t, tt.want, ok)
		})
	}
}

func TestBucket_Delete(t *testing.T) {
	tests := []struct {
		name     string
		fill     []string
		key      string
		want     bool
		wantKeys []string
	}{
		{"пустая корзина", nil, "a", false, nil},
		{"голова", []string{"a", "b", "c"}, "c", true, []string{"b", "a"}},
		{"середина", []string{"a", "b", "c"}, "b", true, []string{"c", "a"}},
		{"хвост", []string{"a", "b", "c"}, "a", true, []string{"c", "b"}},
		{"единственный элемент", []string{"a"}, "a", true, nil},
		{"ключ не найден", []string{"a", "b"}, "x", false, []string{"b", "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bkt := newFilledBucket(tt.fill...)

			assert.Equal(t, tt.want, bkt.delete(tt.key))
			assert.Equal(t, tt.wantKeys, keys(bkt))
			assertConsistent(t, bkt)

			_, ok := bkt.get(tt.key, now)
			assert.False(t, ok)
		})
	}
}

func TestBucket_Delete_Expired(t *testing.T) {
	bkt := newBucket()
	bkt.put("a", "1", expired())

	// просроченный узел еще в списке, поэтому он удаляется
	assert.True(t, bkt.delete("a"))
	assert.Equal(t, 0, bkt.size)
	assert.Nil(t, bkt.head)
}

func TestBucket_DeleteExpired_Empty(t *testing.T) {
	bkt := newBucket()

	removed, total := bkt.deleteExpired(now)

	assert.Equal(t, 0, removed)
	assert.Equal(t, 0, total)
}

func TestBucket_DeleteExpired_NothingExpired(t *testing.T) {
	bkt := newFilledBucket("a", "b", "c")

	removed, total := bkt.deleteExpired(now)

	assert.Equal(t, 0, removed)
	assert.Equal(t, 3, total)
	assert.Equal(t, []string{"c", "b", "a"}, keys(bkt))
	assertConsistent(t, bkt)
}

func TestBucket_DeleteExpired_AllExpired(t *testing.T) {
	bkt := newBucket()
	bkt.put("a", "1", expired())
	bkt.put("b", "2", expired())

	removed, total := bkt.deleteExpired(now)

	assert.Equal(t, 2, removed)
	assert.Equal(t, 2, total)
	assert.Nil(t, bkt.head)
	assert.Equal(t, 0, bkt.size)
}

func TestBucket_DeleteExpired_Mixed(t *testing.T) {
	bkt := newBucket()
	// список после вставок: e, d, c, b, a, x
	bkt.put("x", "v", expired()) // хвост
	bkt.put("a", "v", alive())
	bkt.put("b", "v", expired()) // середина
	bkt.put("c", "v", alive())
	bkt.put("d", "v", expired()) // голова
	bkt.put("e", "v", expired()) // голова

	removed, total := bkt.deleteExpired(now)

	assert.Equal(t, 4, removed)
	assert.Equal(t, 6, total)
	assert.Equal(t, []string{"c", "a"}, keys(bkt))
	assert.Equal(t, 2, bkt.size)
	assertConsistent(t, bkt)
}

func TestBucket_DeleteExpired_TTLBoundary(t *testing.T) {
	bkt := newBucket()
	bkt.put("a", "v", now)

	// ровно в момент дедлайна узел еще жив
	removed, total := bkt.deleteExpired(now)
	assert.Equal(t, 0, removed)
	assert.Equal(t, 1, total)

	// сразу после дедлайна удаляется
	removed, total = bkt.deleteExpired(now.Add(time.Nanosecond))
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, bkt.size)
}

func TestBucket_Concurrent(t *testing.T) {
	bkt := newBucket()
	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i%10)
			bkt.put(key, "v", alive())
			bkt.get(key, now)
			bkt.deleteExpired(now)
			bkt.delete(key)
		}()
	}
	wg.Wait()

	assertConsistent(t, bkt)
}
