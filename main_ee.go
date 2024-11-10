//go:build ee
// +build ee

package main

import (
	"github.com/gravitl/netmaker/pro"
	_ "go.uber.org/automaxprocs"
)

func init() {
	pro.InitPro()
}
