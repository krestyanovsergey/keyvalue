package keyvalue

type Config struct {
	InitialBucketCount        int     //Начальное количество корзин
	CollectorInterval         int     //Как часто будут выполняться раунды очистки (в миллисекундах)
	CollectorMaxIterations    int     //Максимальное количество итераций в раунде
	CollectorBatchPercent     float64 //Какой процент корзин очищается на каждой итерации
	CollectorThresholdPercent float64 //Сколько мусора должно быть в текущей итерации, чтобы запустить следующую
}
