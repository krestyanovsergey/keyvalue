package keyvalue

import "time"

type element struct {
	key   string
	value string
	//После этого времени элемент должен быть недоступен
	deleteAfter time.Time
	//Элементы хранятся в виде связанного списка
	next *element
}

func newElement(
	key string,
	value string,
	deleteAfter time.Time,
	next *element,
) *element {
	return &element{
		key:         key,
		value:       value,
		deleteAfter: deleteAfter,
		next:        next,
	}
}

// setValue меняет значение,
// при этом его время жизни должно быть продлено
func (elem *element) setValue(
	value string,
	deleteAfter time.Time,
) {
	elem.value = value
	elem.deleteAfter = deleteAfter
}

// setNext меняет следующую пару
func (elem *element) setNext(next *element) {
	elem.next = next
}

// isReadyForDeletion проверяет готов ли элемент к удалению
func (elem *element) isReadyForDeletion() bool {
	return time.Now().After(elem.deleteAfter)
}
