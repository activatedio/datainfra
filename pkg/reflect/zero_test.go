package reflect_test

import (
	"testing"

	"github.com/activatedio/datainfra/pkg/reflect"
	"github.com/stretchr/testify/assert"
)

type Dummy struct{}

func TestZeroInterface(t *testing.T) {
	a := assert.New(t)
	a.Equal(&Dummy{}, reflect.ZeroInterface[*Dummy]())
}

func TestNilInterface(t *testing.T) {
	a := assert.New(t)
	a.Nil(reflect.NilInterface[*Dummy]())
}
