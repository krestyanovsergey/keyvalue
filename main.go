package keyvalue

import (
	"sync"
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

// Bucket хранит пары
type Bucket struct {
	Head *KeyValue
	Mu   sync.RWMutex
}

func NewBucket() *Bucket {
	return &Bucket{}
}

// Put добавляет новую пару или обновляет существующую.
// Возвращает true, если пара была создана,
// false если была обновлена существующая
func (bkt *Bucket) Put(
	key string,
	value string,
	deleteAfter time.Time,
) (created bool) {

	bkt.Mu.Lock()
	defer bkt.Mu.Unlock()

	for cur := bkt.Head; cur != nil; cur = cur.Next {
		//пара уже существует
		if cur.Key == key {
			//обновление пары
			cur.SetValue(value, deleteAfter)
			return false
		}
	}

	//новая пара становится головой
	bkt.Head = NewKeyValue(key, value, deleteAfter, bkt.Head)
	return true
}

// Get возвращает значение по ключу.
// Просроченные пары не отдаются (возвращается false),
// но и не удаляются
func (bkt *Bucket) Get(key string) (string, bool) {

	bkt.Mu.RLock()
	defer bkt.Mu.RUnlock()

	for cur := bkt.Head; cur != nil; cur = cur.Next {
		if cur.Key == key {
			//пара существует, но она протухла
			if cur.IsReadyForDeletion() {
				return "", false
			}
			return cur.Value, true
		}
	}

	return "", false
}

// Delete удаляет пару по ключу.
// Возвращает true, если пара была найдена и удалена.
func (bkt *Bucket) Delete(key string) (deleted bool) {
	bkt.Mu.Lock()
	defer bkt.Mu.Unlock()

	//если список пуст, то удалять нечего
	if bkt.Head == nil {
		return false
	}

	//если удалить нужно голову, то головой	становится следующий узел
	if bkt.Head.Key == key {
		bkt.Head = bkt.Head.Next
		return true
	}

	for prev, cur := bkt.Head, bkt.Head.Next; cur != nil; prev, cur = cur, cur.Next {
		if cur.Key == key {
			prev.Next, cur.Next = cur.Next, nil
			return true
		}
	}

	return false
}
