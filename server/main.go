package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	var port string
	flag.StringVar(&port, "p", "4004", "port number")
	flag.Parse()

	db := make(map[string]*net.TCPConn)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalln(err)
	}

	l, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	go func() {
		<-sig
		cancel()
		l.Close()
		os.Exit(0)
	}()

	fmt.Printf("waiting for clients on port %s...\n", port)

	for {
		c, err := l.AcceptTCP()
		if err != nil {
			fmt.Println(err)
			return
		}
		defer c.Close()

		uuid := randomString(10)
		(db)[uuid] = c
		c.Write([]byte(fmt.Sprintf("\n\nYour UserId is: %s\n", uuid)))
		c.Write([]byte("send START_MAIN-{UserId} to start a chat session.\n"))
		c.Write([]byte("e.g. -> START_MAIN-xxxxxxxxxx\n"))

		go cmdHandler(c, uuid, &db, ctx)
	}
}

func cmdHandler(c *net.TCPConn, userId string, db *map[string]*net.TCPConn, ctx context.Context) {
	fmt.Printf("client connected: %s\n", (*c).RemoteAddr().String())
	for {
		var data string
		var partyId string

		in, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println("err bufio.NewReader(c).ReadString('\n'): ", err)
		}
		in = strings.TrimSpace(in)

		if strings.Contains(in, "-") {
			d := strings.Split(in, "-")
			if len(d) == 2 {
				data = strings.Split(in, "-")[0]
				partyId = strings.Split(in, "-")[1]
			} else {
				c.Write([]byte("\ninvalid command\n"))
				continue
			}
		}

		if data == "START_PARTY" {
			fmt.Println("party started")
			(*c).Write([]byte("you may chat now...\n\n"))
			(*(*db)[partyId]).Write([]byte("you may chat now...\n\n"))
			break
		}

		if (*db)[partyId] == nil {
			(*c).Write([]byte("UserId not found.\n"))
			continue
		}

		if data == "START_MAIN" {
			fmt.Println("main started")
			(*c).Write([]byte(fmt.Sprintf("waiting for %s to join...\n", partyId)))
			(*(*db)[partyId]).Write([]byte(fmt.Sprintf("%s wants to initiate a chat session with you.\n", userId)))
			(*(*db)[partyId]).Write([]byte(fmt.Sprintf("send START_PARTY-%s to start...\n", userId)))
			chatHandler(c, userId, partyId, db, ctx)
			break
		}
	}
}

func chatHandler(c *net.TCPConn, userId string, partyId string, db *map[string]*net.TCPConn, ctx context.Context) {
	defer (*db)[partyId].Close()
	defer c.Close()
	defer delete(*db, userId)

	endChat := make(chan bool)

	go func() {
		<-ctx.Done()
		endChat <- true
	}()

	go func() {
		fmt.Printf("bridging %s to %s\n", userId, partyId)
		_, err := io.Copy((*db)[partyId], c)
		if err != nil {
			fmt.Println("err sending data to main: ", err)
			endChat <- true
		}
		(*db)[partyId].Write([]byte(fmt.Sprintf("oops! %s has left the chat\n", userId)))
		fmt.Println("main disconnected")
		endChat <- true
	}()

	go func() {
		fmt.Printf("bridging %s to %s\n", partyId, userId)
		_, err := io.Copy(c, (*db)[partyId])
		if err != nil {
			fmt.Println("err sending data to party: ", err)
			endChat <- true
		}
		c.Write([]byte(fmt.Sprintf("oops! %s has left the chat\n", partyId)))
		fmt.Println("party disconnected")
		endChat <- true
	}()

	<-endChat
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}
