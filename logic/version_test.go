package logic

import (
	"testing"

	"github.com/matryer/is"
)

func TestVersion(t *testing.T) {
	t.Run("valid version", func(t *testing.T) {
		is := is.New(t)
		valid := IsVersionComptatible("v0.17.1-testing")
		is.Equal(valid, true)
	})
	t.Run("dev version", func(t *testing.T) {
		is := is.New(t)
		valid := IsVersionComptatible("dev")
		is.Equal(valid, true)
	})
	t.Run("invalid version", func(t *testing.T) {
		is := is.New(t)
		valid := IsVersionComptatible("v0.14.2-refactor")
		is.Equal(valid, false)
	})
	t.Run("no version", func(t *testing.T) {
		is := is.New(t)
		valid := IsVersionComptatible("testing")
		is.Equal(valid, false)
	})
	t.Run("incomplete version", func(t *testing.T) {
		is := is.New(t)
		valid := IsVersionComptatible("0.18")
		is.Equal(valid, true)
	})
}
