package keyvalue

import (
	"sync"
	"time"
)

// bucket хранит элементы
type bucket struct {
	head *element
	mu   sync.RWMutex
	//Количество узлов в списке (включая просроченные)
	size int
}

func newBucket() *bucket {
	return &bucket{}
}

// newBucketSlice создает n bucket
func newBucketSlice(n int) []*bucket {
	slice := make([]*bucket, n)

	for i := range n {
		slice[i] = newBucket()
	}

	return slice
}

// put добавляет новую пару или обновляет существующую.
// Возвращает true, если создан новый узел,
// false если обновлен существующий (в том числе просроченный)
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
	bkt.size++
	return true
}

// get возвращает значение по ключу.
// Пары, просроченные на момент now, не отдаются (возвращается false),
// но и не удаляются
func (bkt *bucket) get(key string, now time.Time) (string, bool) {

	bkt.mu.RLock()
	defer bkt.mu.RUnlock()

	for cur := bkt.head; cur != nil; cur = cur.next {
		if cur.key == key {
			//пара существует, но она протухла
			if cur.isReadyForDeletion(now) {
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
		bkt.size--
		return true
	}

	for prev, cur := bkt.head, bkt.head.next; cur != nil; prev, cur = cur, cur.next {
		if cur.key == key {
			prev.next = cur.next
			bkt.size--
			return true
		}
	}

	return false
}

// deleteExpired удаляет элементы, просроченные на момент now.
// Возвращает сколько элементов было удалено и сколько их было всего до удаления.
func (bkt *bucket) deleteExpired(now time.Time) (removed, total int) {

	bkt.mu.Lock()
	defer bkt.mu.Unlock()

	total = bkt.size

	for bkt.head != nil && bkt.head.isReadyForDeletion(now) {
		bkt.head = bkt.head.next
		bkt.size--
		removed++
	}

	if bkt.head == nil {
		return removed, total
	}

	for prev, cur := bkt.head, bkt.head.next; cur != nil; {
		if cur.isReadyForDeletion(now) {
			prev.next = cur.next
			bkt.size--
			removed++
			cur = prev.next
			continue
		}
		prev, cur = cur, cur.next
	}

	return removed, total
}
