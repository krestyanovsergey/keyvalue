package keyvalue

import (
	"context"
	"hash/maphash"
	"sync"
	"time"
)

type Storage struct {
	buckets []*bucket
	seed    maphash.Seed

	//COLLECTOR

	//Корзины очищаются последовательно (кольцевая очередь)
	collectorCursor int

	//Как часто будут выполняться раунды очистки
	collectorInterval time.Duration

	//КОНФИГ РАУНДА

	//Максимальное количество итераций в раунде
	collectorMaxIterations int
	//Какая доля корзин очищается на каждой итерации
	collectorBatchRatio float64
	//Сколько мусора должно быть в текущей итерации, чтобы запустить следующую
	collectorThresholdRatio float64

	//Остановить collector
	cancel context.CancelFunc

	//Чтобы Stop дождался завершения startCleaner
	wg sync.WaitGroup
}

func NewStorage(config Config) (*Storage, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	storage := &Storage{
		buckets:                 newBucketSlice(config.InitialBucketCount),
		seed:                    maphash.MakeSeed(),
		collectorInterval:       config.CollectorInterval,
		collectorMaxIterations:  config.CollectorMaxIterations,
		collectorBatchRatio:     config.CollectorBatchRatio,
		collectorThresholdRatio: config.CollectorThresholdRatio,
		cancel:                  cancel,
	}

	storage.wg.Add(1)
	go func() {
		defer storage.wg.Done()
		storage.startCleaner(ctx)
	}()

	return storage, nil
}

// getBucketByKey вычисляет bucket на основе ключа
func (storage *Storage) getBucketByKey(key string) *bucket {
	hash := maphash.String(storage.seed, key)
	length := len(storage.buckets)
	index := hash % uint64(length)
	return storage.buckets[index]
}

// Put сохраняет пару на время ttl.
// Запрос с ttl <= 0 невалиден и игнорируется
func (storage *Storage) Put(
	key string,
	value string,
	ttl time.Duration,
) {

	if ttl <= 0 {
		return
	}

	bkt := storage.getBucketByKey(key)
	deleteAfter := time.Now().Add(ttl)
	bkt.put(key, value, deleteAfter)
}

func (storage *Storage) Get(key string) (string, bool) {
	bkt := storage.getBucketByKey(key)
	return bkt.get(key, time.Now())
}

func (storage *Storage) Delete(key string) {
	bkt := storage.getBucketByKey(key)
	bkt.delete(key)
}

//СБОРЩИК МУСОРА

// cleanRound раунд очистки
func (storage *Storage) cleanRound() {
	//единый момент времени для всего раунда
	now := time.Now()

	length := len(storage.buckets)
	//за каждую итерацию должна быть очищена как минимум одна корзина
	batch := max(int(float64(length)*storage.collectorBatchRatio), 1)

	for range storage.collectorMaxIterations {
		var removed, total int

		for range batch {
			r, t := storage.buckets[storage.collectorCursor].deleteExpired(now)
			storage.collectorCursor++
			if storage.collectorCursor == length {
				storage.collectorCursor = 0
			}
			removed += r
			total += t
		}

		//Если бы все ведра были пусты или в них было не так много мусора
		if total == 0 || float64(removed)/float64(total) < storage.collectorThresholdRatio {
			//Нет смысла делать следующую итерацию
			return
		}
	}
}

// startCleaner запуск очистки
func (storage *Storage) startCleaner(ctx context.Context) {
	ticker := time.NewTicker(storage.collectorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			storage.cleanRound()
		case <-ctx.Done():
			return
		}
	}

}

// Stop останавливает сборку мусора
func (storage *Storage) Stop() {
	storage.cancel()
	storage.wg.Wait()
}
