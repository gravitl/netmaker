//go:build !linux
// +build !linux

package local

//"github.com/davecgh/go-spew/spew"

/*

These functions are not used. These should only be called by Linux (see routes_linux.go). These routes return nothing if called.

*/

func routeExists(iface, address, mask string) bool {
	return false
}

func SetRoute(iface, newAddress, oldAddress, mask string) error {
	return nil
}

func DeleteRoute(iface, address) error {
	return nil
}
