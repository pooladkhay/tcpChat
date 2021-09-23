package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("please server address as host:port")
		return
	}

	addr, err := net.ResolveTCPAddr("tcp", arguments[1])
	if err != nil {
		log.Fatalln(err)
	}

	c, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		fmt.Println("exiting...")
		c.Close()
	}()

	go io.Copy(c, os.Stdin)
	go io.Copy(os.Stdin, c)

	<-sig
}
