package utils

import (
	"os"
	"math"
	"crypto/md5"
	"io"
	"fmt"
)

func Checksum(path string ,chuncksize uint64) string {


	file, err := os.Open(path)

	if err != nil {
		panic(err.Error())
	}

	defer file.Close()

	// calculate the file size
	info, _ := file.Stat()

	filesize := info.Size()

	blocks := uint64(math.Ceil(float64(filesize) / float64(chuncksize)))

	hash := md5.New()

	for i := uint64(0); i < blocks; i++ {
		blocksize := int(math.Min(float64(chuncksize), float64(filesize-int64(i*chuncksize))))
		buf := make([]byte, blocksize)

		file.Read(buf)
		io.WriteString(hash, string(buf)) // append into the hash
	}
	var bytes []byte
	bytes = hash.Sum(nil)
	retStr := fmt.Sprintf("%x",bytes)
	fmt.Printf("checksum is %s \n",retStr )

	return retStr
}
