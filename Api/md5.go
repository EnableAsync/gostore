package wheel

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
)

func MD5String(data string) string {
	cmd5 := md5.New()
	cmd5.Write([]byte(data))
	md5data := cmd5.Sum([]byte(nil))
	return hex.EncodeToString(md5data)
}

func Md5(data []byte) string {
	cmd5 := md5.New()
	cmd5.Write(data)
	md5Data := cmd5.Sum([]byte(nil))
	return hex.EncodeToString(md5Data)
}

func Sha1String(data string) string {
	csha1 := sha1.New()
	csha1.Write([]byte(data))
	return hex.EncodeToString(csha1.Sum([]byte(nil)))
}

func Sha1(data []byte) string {
	csha1 := sha1.New()
	csha1.Write(data)
	return hex.EncodeToString(csha1.Sum([]byte(nil)))
}
