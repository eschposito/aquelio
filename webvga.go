package main

import (
	"crypto/tls"
	"strings"
	"log"
	"net"
	"time"
)

const loginForm = `<html>
<head>
<title>WebVGA login</title>
</head>
<body>
<form action="/" method="post">
ACCESS PASSWORD:<input type="password" name="ACCPWD"><br><br>
COMMAND PASSWORD:<input type="password" name="COMPWD"><br><br>
<input type="submit" value=" << GO! >> ">
</form>
</body>
</html>`

var vmem [2][2000]byte

func main() {
	log.SetFlags(log.Lshortfile)

	cer, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Println(err)
		return
	}

	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	ln, err := tls.Listen("tcp", ":25681", config)
	if err != nil {
		log.Println(err)
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}

func getReqText(conn net.Conn, bb *[]byte) (err error) {
	for t := 0; t <= 80; t++ { // loops until successfully receiving HTTPS request or for 8 sec
		_, err = conn.Read(*bb)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for { // this loop takes care of password login, then sends WebVGA page
		err := getReqText(conn, &buf)
		if err != nil {
			log.Println("READ ERROR! >> ", err)
			return
		} else {
			println("$ >> ", string(buf))
			switch {
			case string(buf[:6])=="GET / ":
				_, err := conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n" + loginForm))
				if err != nil {
					log.Println("WRITE ERROR! >> ", err)
					return
				}
			case string(buf[:7])=="POST / ":
				st:= strings.Split(string(buf), "\r\n\r\n")
				bodyst:= strings.Trim(st[len(st)-1], "\r\n")
				//Trim(bodysts string, cutset string)
				for {
					post := strings.Split(bodyst, "&")
					if err != nil {
						log.Println("LINEREAD ERROR! >> ", err)
						return
					}
				}
			default:
				_, err = conn.Write([]byte("HTTP/1.1 404 NOT FOUND\r\n\r\n"))
				return
			}
			
			
		}
	} // end of login section

}
