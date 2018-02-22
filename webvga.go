package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	//"time"
)

var vmem [4][4000]byte

func vramHandler(w http.ResponseWriter, req *http.Request) {
    //w.Header().Set("Content-Type", "text/plain")
    w.Write(vmem[0][0:4000])
    // fmt.Fprintf(w, "This is an example server.\n")
    // io.WriteString(w, "This is an example server.\n")
}

func clickHandler(w http.ResponseWriter, req *http.Request) {
    //w.Header().Set("Content-Type", "text/plain")
    //w.Write(vmem[0][0:4000])
    // fmt.Fprintf(w, "This is an example server.\n")
    // io.WriteString(w, "This is an example server.\n")
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
    http.HandleFunc("/vram~" + string(axspw) + "/", vramHandler)
	http.HandleFunc("/vram~" + string(exepw) + "/", vramHandler)
	http.HandleFunc("/click~" + string(exepw) + "/", clickHandler)
	http.Handle("/", http.FileServer(http.Dir("pub")))
    err = http.ListenAndServeTLS(":25681", "cert.pem", "key.pem", nil)
    if err != nil {
        fmt.Println("ListenAndServeTLS error: ", err)
    }
}
