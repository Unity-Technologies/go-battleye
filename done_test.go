package battleye

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoneIsDone(t *testing.T) {
	done := newDone()
	assert.Equal(t, false, done.IsDone())
	done.Done()
	assert.Equal(t, true, done.IsDone())
}

func TestDoneTwice(t *testing.T) {
	done := newDone()
	assert.Equal(t, false, done.IsDone())
	done.Done()
	assert.Equal(t, true, done.IsDone())
	done.Done()
	assert.Equal(t, true, done.IsDone())
}

func TestDoneC(t *testing.T) {
	done := newDone()
	wait := make(chan struct{})
	var v struct{}
	var ok bool
	go func() {
		v, ok = <-done.C()
		close(wait)
	}()

	done.Done()
	<-wait
	var empty struct{}
	assert.Equal(t, false, ok)
	assert.Equal(t, empty, v)
}
