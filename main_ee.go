//go:build ee
// +build ee

package main

import (
	"github.com/gravitl/netmaker/ee"
)

func init() {
	ee.InitEE()
}
