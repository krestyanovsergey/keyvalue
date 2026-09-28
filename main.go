package keyvalue

import (
	"time"
)

type KeyValue struct {
	Key   string
	Value string
	//после этого времени пара должна быть удалена
	DeleteAfter time.Time
	//пары хранятся в виде связанного списка
	Next *KeyValue
}

func NewKeyValue(
	key string,
	value string,
	deleteAfter time.Time,
	next *KeyValue,
) *KeyValue {
	return &KeyValue{
		Key:         key,
		Value:       value,
		DeleteAfter: deleteAfter,
		Next:        next,
	}
}

// SetValue меняет значение,
// при этом его время жизни должно быть продлено
func (kv *KeyValue) SetValue(
	value string,
	deleteAfter time.Time,
) {
	kv.Value = value
	kv.DeleteAfter = deleteAfter
}

// SetNext меняет следующую пару
func (kv *KeyValue) SetNext(next *KeyValue) {
	kv.Next = next
}

// IsReadyForDeletion готова ли пара к удалению,
// если да, то ее необходимо удалить
func (kv *KeyValue) IsReadyForDeletion() bool {
	return time.Now().After(kv.DeleteAfter)
}
