package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	//"github.com/luismesas/goPi/spi"
	"golang.org/x/exp/io/spi"
	//"time"
)

var vmem [4][4000]byte // 4 sectors: 1 is written to while 1 is displayed
var readindex uint32 = 0 // atomic access first index of displayed vmem sector

func vramHandler(w http.ResponseWriter, req *http.Request) {
	ind := atomic.LoadUint32(&readindex)
	w.Write(vmem[ind&3][0:4000]) // sends data of sector to be displayed
}

func clickHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200) // empty OK HTTP response
}

func main() {
	axspw, err := ioutil.ReadFile("axspw.txt")
	if err != nil {
		fmt.Println("File read error: ", err)
	}
	exepw, err := ioutil.ReadFile("exepw.txt")
	if err != nil {
		fmt.Println("File read error: ", err)
	}
	http.HandleFunc("/vram~"+string(axspw)+"/", vramHandler)
	http.HandleFunc("/vram~"+string(exepw)+"/", vramHandler)
	http.HandleFunc("/click~"+string(exepw)+"/", clickHandler)
	http.Handle("/", http.FileServer(http.Dir("pub")))
	err = http.ListenAndServeTLS(":25681", "cert.pem", "key.pem", nil)
	if err != nil {
		fmt.Println("ListenAndServeTLS error: ", err)
	}
}

func useSpi() {
dev, err := spi.Open(&spi.Devfs{
    Dev:      "/dev/spidev0.1",
    Mode:     spi.Mode3,
    MaxSpeed: 500000,
})
if err != nil {
    panic(err)
}
defer dev.Close()
}