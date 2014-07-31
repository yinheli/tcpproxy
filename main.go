package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

const bufSize = 4096
const nBuf = 2048

const (
	NO_TIMEOUT = iota
	SET_TIMEOUT
	TYPE_OUT
	TYPE_IN
)

var (
	port   = flag.String("port", "1234", "listen port, default 1234")
	target = flag.String("target", "", "target host and port. eg: 192.168.1.100:9001")
	t      = flag.Int("timeout", 60, "timeout (second), default 60s")
	debug  = flag.Bool("debug", false, "print debug message")
	sleep  = flag.Int("sleep", 0, "every transfer with sleep second(s)")

	timeout time.Duration

	leakybuf = NewLeakyBuf(nBuf, bufSize)
)

func main() {

	fmt.Println("\n  tcpproxy @author yinheli (me@yinheli.com), version: 1.0.1\n\n**********")

	flag.Parse()

	if *target == "" {
		flag.Usage()
		return
	}

	timeout = time.Duration(*t) * time.Second

	lis, err := net.Listen("tcp", "0.0.0.0:"+*port)
	if err != nil {
		log.Printf("listen failed, message:%v", err.Error())
		return
	}

	log.Printf("start proxy: listen:%v, target:%v, debug:%v, sleep:%v", *port, *target, *debug, *sleep)

	for {
		conn, err := lis.Accept()
		if err != nil {
			panic(fmt.Sprintf("accept error: %v", err.Error()))
			continue
		}

		go func() {
			defer func() {
				if x := recover(); x != nil {
					panic(fmt.Sprintf("handle exception: %v", x))
				}
			}()

			tconn, err := net.Dial("tcp", *target)
			if err != nil {
				log.Printf("Dial target error: %v", err.Error())
				return
			}

			log.Printf("start: %s<->%s", conn.RemoteAddr(), *target)
			go pipeThenClose(conn, tconn, SET_TIMEOUT, TYPE_OUT)
			pipeThenClose(tconn, conn, NO_TIMEOUT, TYPE_IN)
		}()
	}

}

func setReadTimeout(c net.Conn) {
	if timeout != 0 {
		c.SetReadDeadline(time.Now().Add(timeout))
	}
}

func pipeThenClose(src, dst net.Conn, timeoutOpt int, dt int) {
	defer dst.Close()
	buf := leakybuf.Get()
	defer leakybuf.Put(buf)

	for {
		if timeoutOpt == SET_TIMEOUT {
			setReadTimeout(src)
		}
		n, err := src.Read(buf)
		// read may return EOF with n > 0
		// should always process n > 0 bytes before handling error
		if n > 0 {
			data := buf[0:n]

			if dt == TYPE_IN && *sleep > 0 {
				log.Printf("get recive data, start do sleep %v second(s)", *sleep)
				time.Sleep(time.Second * time.Duration(*sleep))
			}

			if _, err = dst.Write(data); err != nil {
				break
			} else {
				if *debug {
					if dt == TYPE_OUT {
						log.Printf("send: %s <-> %s \n%v", src.RemoteAddr(), dst.RemoteAddr(), hex.Dump(data))
					} else {
						log.Printf("recive: %s <-> %s \n%v", src.RemoteAddr(), dst.RemoteAddr(), hex.Dump(data))
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
}
