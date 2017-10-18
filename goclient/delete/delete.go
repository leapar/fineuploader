package delete

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type Deleter struct {
	host string
}

func New(host string) *Deleter {
	return &Deleter{
		host:host,
	}
}

func (s* Deleter) Delete(fuid string) int{

	//body := bytes.NewBuffer(reader.buf.Bytes())
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://%s/upload/%s",s.host,fuid),nil)//"HTTP://127.0.0.1:8081/chunksdone
	if err != nil {
		///verbose("bosun connect error: %v", err)
		return -1
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		//verbose("bosun relay error: %v", err)
		return -1
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s -- %v\n", string(buf), err)
	}


	resp.Body.Close()
	//verbose("bosun relay success")

	return resp.StatusCode
}
