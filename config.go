package keyvalue

type Config struct {
	InitialBucketCount     int //Начальное количество корзин
	CollectorInterval      int //Как часто будут выполняться раунды очистки (в миллисекундах)
	CollectorMaxIterations int //Максимальное количество итераций в раунде

	CollectorBatchRatio     float64 //Какая доля корзин очищается на каждой итерации
	CollectorThresholdRatio float64 //Сколько мусора должно быть в текущей итерации, чтобы запустить следующую
}
