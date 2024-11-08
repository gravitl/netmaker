package servercfg

import (
	"testing"

	"github.com/matryer/is"
)

func TestValidateDomain(t *testing.T) {

	t.Run("", func(t *testing.T) {
		is := is.New(t)
		valid := validateDomain("netmaker.hosted")
		is.Equal(valid, true)
	})

	t.Run("", func(t *testing.T) {
		is := is.New(t)
		valid := validateDomain("ipv4test1.hosted")
		is.Equal(valid, true)
	})

	t.Run("", func(t *testing.T) {
		is := is.New(t)
		valid := validateDomain("ip_4?")
		is.Equal(valid, false)
	})

}
