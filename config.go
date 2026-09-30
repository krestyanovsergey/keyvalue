package keyvalue

import (
	"errors"
	"time"
)

type Config struct {
	InitialBucketCount      int           //Начальное количество корзин
	CollectorInterval       time.Duration //Как часто будут выполняться раунды очистки
	CollectorMaxIterations  int           //Максимальное количество итераций в раунде
	CollectorBatchRatio     float64       //Какая доля корзин очищается на каждой итерации
	CollectorThresholdRatio float64       //Сколько мусора должно быть в текущей итерации, чтобы запустить следующую
}

func (config *Config) Validate() error {
	var errs []error

	if config.InitialBucketCount < 1 {
		errs = append(errs, errors.New("InitialBucketCount must be greater than zero"))
	}

	if config.CollectorInterval < 1 {
		errs = append(errs, errors.New("CollectorInterval must be greater than zero"))
	}

	if config.CollectorMaxIterations < 1 {
		errs = append(errs, errors.New("CollectorMaxIterations must be greater than zero"))
	}

	if config.CollectorBatchRatio <= 0 || config.CollectorBatchRatio > 1 {
		errs = append(errs, errors.New("CollectorBatchRatio must be > 0 and <= 1"))
	}

	if config.CollectorThresholdRatio <= 0 || config.CollectorThresholdRatio > 1 {
		errs = append(errs, errors.New("CollectorThresholdRatio must be > 0 and <= 1"))
	}

	return errors.Join(errs...)
}
