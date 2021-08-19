package functions

import (
	"io/ioutil"
	"os"
)

func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func SetDNSDir() error {
        dir, err := os.Getwd()
        if err != nil {
                return err
        }
        _, err = os.Stat(dir + "/config/dnsconfig")
        if os.IsNotExist(err) {
                os.Mkdir(dir+"/config/dnsconfig", 0744)
        } else if err != nil {
                PrintUserLog("","couldnt find or create /config/dnsconfig",0)
                return err
        }
		_, err = os.Stat(dir + "/config/dnsconfig/Corefile")
        if os.IsNotExist(err) {
			err = SetCorefile("example.com")
			if err != nil {
				PrintUserLog("",err.Error(),0)
			}
		}
		_, err = os.Stat(dir + "/config/dnsconfig/netmaker.hosts")
        if os.IsNotExist(err) {
			_, err = os.Create(dir + "/config/dnsconfig/netmaker.hosts")
			if err != nil {
				PrintUserLog("",err.Error(),0)
			}
		}		
	return nil
}

func SetCorefile(domains string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	_, err = os.Stat(dir + "/config/dnsconfig")
	if os.IsNotExist(err) {
		os.Mkdir(dir+"/config/dnsconfig", 744)
	} else if err != nil {
		PrintUserLog("","couldnt find or create /config/dnsconfig",0)
		return err
	}

	corefile := domains + ` {
    reload 15s
    hosts /root/dnsconfig/netmaker.hosts {
	fallthrough	
    }
    forward . 8.8.8.8 8.8.4.4
    log
}
`
	corebytes := []byte(corefile)

	err = ioutil.WriteFile(dir+"/config/dnsconfig/Corefile", corebytes, 0644)
	if err != nil {
		return err
	}
	return err
}
