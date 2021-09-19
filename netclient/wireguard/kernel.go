package wireguard

import (
	"os/exec"

	"github.com/gravitl/netmaker/netclient/ncutils"
	//homedir "github.com/mitchellh/go-homedir"
)

func setKernelDevice(ifacename string, address string) error {
	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	_, _ = ncutils.RunCmd("ip link delete dev "+ifacename, false)
	_, _ = ncutils.RunCmd(ipExec+" link add dev "+ifacename+" type wireguard", true)
	_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address+"/24", true)

	return nil
}
