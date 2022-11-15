package packet

import (
	"crypto/md5"
	"fmt"
)

var udpHeaderLen = 8

func ProcessPacketBeforeSending(buf []byte, n int, srckey, dstKey string) ([]byte, int, string, string) {

	srcKeymd5 := md5.Sum([]byte(srckey))
	dstKeymd5 := md5.Sum([]byte(dstKey))
	if n > len(buf)-len(srcKeymd5)-len(dstKeymd5) {
		buf = append(buf, srcKeymd5[:]...)
		buf = append(buf, dstKeymd5[:]...)
	} else {
		copy(buf[n:n+len(srcKeymd5)], srcKeymd5[:])
		copy(buf[n+len(srcKeymd5):n+len(srcKeymd5)+len(dstKeymd5)], dstKeymd5[:])
	}
	n += len(srcKeymd5)
	n += len(dstKeymd5)

	return buf, n, fmt.Sprintf("%x", srcKeymd5), fmt.Sprintf("%x", dstKeymd5)
}

func ExtractInfo(buffer []byte, n int) (int, string, string) {
	data := buffer[:n]
	if len(data) < 32 {
		return 0, "", ""
	}
	srcKeyHash := data[n-32 : n-16]
	dstKeyHash := data[n-16:]
	n -= 32
	return n, fmt.Sprintf("%x", srcKeyHash), fmt.Sprintf("%x", dstKeyHash)
}
