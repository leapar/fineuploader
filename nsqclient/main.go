package main

import (
	"github.com/nsqio/go-nsq"

	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// ConsumerHandler 消费者处理者
type ConsumerHandler struct{}

// HandleMessage 处理消息
func (*ConsumerHandler) HandleMessage(msg *nsq.Message) error {
//	fmt.Println(string(msg.Body))
	return nil
}

func Init(addr string) error {

	cfg := nsq.NewConfig()
    consumer, err := nsq.NewConsumer("file","client",cfg)
	if nil != err {
		return err
	}

	consumer.AddHandler(&ConsumerHandler{})

	if err := consumer.ConnectToNSQLookupd(addr); err != nil {
		fmt.Println("ConnectToNSQLookupd", err)
		panic(err)
	}



	return nil
}

func main() {
	chQuit := make(chan os.Signal, 1)
	signal.Notify(chQuit, syscall.SIGINT, syscall.SIGTERM)

	Init("127.0.0.1:4161")

	<- chQuit
}