package keyvalue

import (
	"sync"
	"time"
)

// bucket хранит элементы
type bucket struct {
	head *element
	mu   sync.RWMutex
}

func newBucket() *bucket {
	return &bucket{}
}

// put добавляет новую пару или обновляет существующую.
// Возвращает true, если пара была создана,
// false если была обновлена существующая
func (bkt *bucket) put(
	key string,
	value string,
	deleteAfter time.Time,
) (created bool) {

	bkt.mu.Lock()
	defer bkt.mu.Unlock()

	for cur := bkt.head; cur != nil; cur = cur.next {
		//пара уже существует
		if cur.key == key {
			//обновление пары
			cur.setValue(value, deleteAfter)
			return false
		}
	}

	//новая пара становится головой
	bkt.head = newElement(key, value, deleteAfter, bkt.head)
	return true
}

// get возвращает значение по ключу.
// Просроченные пары не отдаются (возвращается false),
// но и не удаляются
func (bkt *bucket) get(key string) (string, bool) {

	bkt.mu.RLock()
	defer bkt.mu.RUnlock()

	for cur := bkt.head; cur != nil; cur = cur.next {
		if cur.key == key {
			//пара существует, но она протухла
			if cur.isReadyForDeletion() {
				return "", false
			}
			return cur.value, true
		}
	}

	return "", false
}

// delete удаляет пару по ключу.
// Возвращает true, если пара была найдена и удалена.
func (bkt *bucket) delete(key string) (deleted bool) {
	bkt.mu.Lock()
	defer bkt.mu.Unlock()

	//если список пуст, то удалять нечего
	if bkt.head == nil {
		return false
	}

	//если удалить нужно голову, то головой становится следующий узел
	if bkt.head.key == key {
		bkt.head = bkt.head.next
		return true
	}

	for prev, cur := bkt.head, bkt.head.next; cur != nil; prev, cur = cur, cur.next {
		if cur.key == key {
			prev.next, cur.next = cur.next, nil
			return true
		}
	}

	return false
}
