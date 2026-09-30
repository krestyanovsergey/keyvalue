package keyvalue

import (
	"hash/maphash"
	"math/rand/v2"
	"time"
)

type Storage struct {
	buckets []*bucket
	seed    maphash.Seed

	//COLLECTOR

	//Как часто будут выполняться раунды очистки
	collectorInterval time.Duration

	//КОНФИГ РАУНДА

	//Максимальное количество итераций в раунде
	collectorMaxIterations int
	//Какой процент корзин очищается на каждой итерации
	collectorBatchPercent float64
	//Сколько мусора должно быть в текущей итерации, чтобы запустить следующую
	collectorThresholdPercent float64
}

func NewStorage(config Config) *Storage {
	storage := &Storage{
		buckets:                   newBucketSlice(config.InitialBucketCount),
		seed:                      maphash.MakeSeed(),
		collectorInterval:         time.Duration(config.CollectorInterval) * time.Millisecond,
		collectorMaxIterations:    config.CollectorMaxIterations,
		collectorBatchPercent:     config.CollectorBatchPercent,
		collectorThresholdPercent: config.CollectorThresholdPercent,
	}

	go storage.startCleaner()

	return storage
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
	ttl int,
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

//СБОРЩИК МУСОРА

// cleanRandomBucket очищает случайную корзину
func (storage *Storage) cleanRandomBucket() (removed, total int) {
	length := len(storage.buckets)
	index := rand.IntN(length)
	return storage.buckets[index].deleteExpired()
}

// cleanRound раунд очистки
func (storage *Storage) cleanRound() {
	length := len(storage.buckets)
	batch := int(float64(length) * storage.collectorBatchPercent)

	for range storage.collectorMaxIterations {
		var removed, total int

		for range batch {
			r, t := storage.cleanRandomBucket()
			removed += r
			total += t
		}

		//Если бы все ведра были пусты или в них было не так много мусора
		if total == 0 || float64(removed)/float64(total) < storage.collectorThresholdPercent {
			//Нет смысла делать следующую итерацию
			return
		}
	}
}

// startCleaner запуск очистки
func (storage *Storage) startCleaner() {
	ticker := time.NewTicker(storage.collectorInterval)
	defer ticker.Stop()
	for range ticker.C {
		storage.cleanRound()
	}
}
