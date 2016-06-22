package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

const bufSize = 8192
const nBuf = 6144

const (
	NO_TIMEOUT = iota
	SET_TIMEOUT
	TYPE_OUT
	TYPE_IN

	SLEEP_LOC_BEFORE = "before"
	SLEEP_LOC_AFTER  = "after"
	SLEEP_LOC_BOTH   = "both"
)

var (
	port     = flag.String("port", "1234", "listen port, default 1234")
	target   = flag.String("target", "", "target host and port. eg: 192.168.1.100:9001")
	t        = flag.Int("timeout", 60, "timeout (second), default 60s")
	debug    = flag.Bool("debug", false, "print debug message")
	format   = flag.String("format", "hex", "debug output format, `hex` or `string`")
	sleep    = flag.Int("sleep", 0, "every transfer with sleep second(s)")
	sleepLoc = flag.String("sleepLoc", SLEEP_LOC_AFTER, "sleep position, before: before send to backend server; after: after receive data from backend; both: all")

	timeout time.Duration

	leakybuf = NewLeakyBuf(nBuf, bufSize)
)

func main() {

	fmt.Println("\n  tcpproxy v1.0.2  @author yinheli")

	flag.Parse()

	if *target == "" || (*sleepLoc != SLEEP_LOC_AFTER && *sleepLoc != SLEEP_LOC_BEFORE && *sleepLoc != SLEEP_LOC_BOTH) {
		flag.Usage()
		fmt.Println()
		return
	}

	fmt.Println("  **********\n")

	if *target == "localhost:"+*port || *target == "127.0.0.1:"+*port {
		fmt.Println("** fuck self ?!!! **\n\n")
		return
	}

	timeout = time.Duration(*t) * time.Second

	lis, err := net.Listen("tcp", "0.0.0.0:"+*port)
	if err != nil {
		log.Printf("listen failed, message:%v", err.Error())
		return
	}

	log.Printf("start proxy: listen:%v, target:%v, timeout:%v, debug:%v, sleep:%v, sleepLoc:%v", *port, *target, *t, *debug, *sleep, *sleepLoc)

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("accept error: %v", err.Error())
			continue
		}

		go func() {
			defer func() {
				if x := recover(); x != nil {
					log.Printf("handle exception: %v", x)
				}
			}()

			tconn, err := net.Dial("tcp", *target)
			if err != nil {
				log.Printf("target error: %v", err.Error())
				conn.Close()
				return
			}
			start := time.Now()
			log.Printf("start: %s <-> %s", conn.RemoteAddr(), *target)
			go pipeThenClose(conn, tconn, SET_TIMEOUT, TYPE_OUT)
			pipeThenClose(tconn, conn, NO_TIMEOUT, TYPE_IN)
			log.Printf("end: %s <-> %s, time:%vms\n-----\n", conn.RemoteAddr(), *target, time.Now().Sub(start).Nanoseconds()/1e6)
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
		start := time.Now()
		if timeoutOpt == SET_TIMEOUT {
			setReadTimeout(src)
		}
		n, err := src.Read(buf)

		// read may return EOF with n > 0
		// should always process n > 0 bytes before handling error
		if n > 0 {
			data := buf[0:n]

			if *sleep > 0 {
				if (*sleepLoc == SLEEP_LOC_AFTER || *sleepLoc == SLEEP_LOC_BOTH) && dt == TYPE_IN {
					log.Printf("recive data, start do sleep %v second(s)", *sleep)
					time.Sleep(time.Second * time.Duration(*sleep))
				}

				if (*sleepLoc == SLEEP_LOC_BEFORE || *sleepLoc == SLEEP_LOC_BOTH) && dt == TYPE_OUT {
					log.Printf("before send data, start do sleep %v second(s)", *sleep)
					time.Sleep(time.Second * time.Duration(*sleep))
				}
			}

			if _, err = dst.Write(data); err != nil {
				break
			} else {
				if *debug {
					timeEscape := time.Now().Sub(start).Nanoseconds() / 1e6
					if dt == TYPE_OUT {
						log.Printf("send: %s <-> %s, time:%vms \n%v\n", src.RemoteAddr(), dst.RemoteAddr(), timeEscape, formatContent(data))
					} else {
						log.Printf("recive: %s <-> %s, time:%vms \n%v\n", src.RemoteAddr(), dst.RemoteAddr(), timeEscape, formatContent(data))
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
}

func formatContent(data []byte) string {
	if *format == "hex" {
		return hex.Dump(data)
	}

	return string(data)
}
