package keyvalue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewElement(t *testing.T) {
	deleteAfter := time.Now().Add(time.Hour)
	next := &element{key: "next"}

	elem := newElement("key", "value", deleteAfter, next)

	assert.Equal(t, "key", elem.key)
	assert.Equal(t, "value", elem.value)
	assert.True(t, deleteAfter.Equal(elem.deleteAfter))
	assert.Same(t, next, elem.next)
}

func TestNewElement_NilNext(t *testing.T) {
	elem := newElement("key", "value", time.Time{}, nil)

	assert.Nil(t, elem.next)
}

func TestElement_SetValue(t *testing.T) {
	oldDeadline := time.Now().Add(time.Minute)
	newDeadline := oldDeadline.Add(time.Hour)
	next := &element{key: "next"}
	elem := newElement("key", "old", oldDeadline, next)

	elem.setValue("new", newDeadline)

	assert.Equal(t, "new", elem.value)
	assert.True(t, newDeadline.Equal(elem.deleteAfter))
	// ключ и связь со следующим элементом не должны меняться
	assert.Equal(t, "key", elem.key)
	assert.Same(t, next, elem.next)
}

func TestElement_SetNext(t *testing.T) {
	first := &element{key: "first"}
	second := &element{key: "second"}
	elem := newElement("key", "value", time.Time{}, first)

	elem.setNext(second)
	assert.Same(t, second, elem.next)

	elem.setNext(nil)
	assert.Nil(t, elem.next)
}

func TestElement_IsReadyForDeletion(t *testing.T) {
	deadline := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	elem := newElement("key", "value", deadline, nil)

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"до дедлайна", deadline.Add(-time.Second), false},
		{"ровно в момент дедлайна", deadline, false},
		{"после дедлайна", deadline.Add(time.Nanosecond), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, elem.isReadyForDeletion(tt.now))
		})
	}
}

func TestElement_IsReadyForDeletion_ZeroDeadline(t *testing.T) {
	elem := newElement("key", "value", time.Time{}, nil)

	assert.True(t, elem.isReadyForDeletion(time.Now()))
}
