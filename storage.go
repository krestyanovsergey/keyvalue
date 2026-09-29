package keyvalue

import (
	"hash/maphash"
	"time"
)

type Storage struct {
	buckets []*bucket
	seed    maphash.Seed
}

func NewStorage(initialBucketCount int) *Storage {
	return &Storage{
		buckets: newBucketSlice(initialBucketCount),
		seed:    maphash.MakeSeed(),
	}
}

// getBucketByKey вычисляет bucket на основе ключа
func (storage *Storage) getBucketByKey(key string) *bucket {
	hash := maphash.String(storage.seed, key)
	length := len(storage.buckets)
	index := hash % uint64(length)
	return storage.buckets[index]
}

func (storage *Storage) Put(
	key string,
	value string,
	//в секундах
	ttl int64,
) {

	bkt := storage.getBucketByKey(key)
	deleteAfter := time.Now().Add(time.Duration(ttl) * time.Second)
	created := bkt.put(key, value, deleteAfter)
	if created {
		//увеличить размер
	}
}

func (storage *Storage) Get(key string) (string, bool) {
	bkt := storage.getBucketByKey(key)
	return bkt.get(key)
}

func (storage *Storage) Delete(key string) {
	bkt := storage.getBucketByKey(key)
	deleted := bkt.delete(key)
	if deleted {
		//уменьшить размер
	}
}
