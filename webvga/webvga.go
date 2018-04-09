package webvga

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

var vmem [4][4000]byte   // 4 sectors: 1 is written to while 1 is displayed
var readindex uint32 = 0 // atomic access first index of displayed vmem sector
var curpos = 0           // current cursor position, from 0 to 2000 included
var defcolors byte       // default background and foreground colors
var printchan chan []byte
var clickchan chan [2]byte

func vramHandler(w http.ResponseWriter, req *http.Request) {
	ind := atomic.LoadUint32(&readindex)
	w.Write(vmem[ind&3][0:4000]) // sends data of sector to be displayed
}

func clickHandler(w http.ResponseWriter, req *http.Request) {
	//clickchan <-
	w.WriteHeader(200) // empty OK HTTP response
}

func Serve(defcols byte, greeting string) (chan<- []byte, <-chan [2]byte) {
	defcolors = defcols                // initialization section
	printchan = make(chan []byte, 10)  // remembers up to 10 print calls
	clickchan = make(chan [2]byte, 10) // remembers up to 10 clicks
	for i, _ := range vmem[0] {
		if i >= 2000 {
			vmem[0][i] = defcols
		} else if i < len(greeting) {
			vmem[0][i] = byte(greeting[i])
		} else {
			vmem[0][i] = byte(' ')
		}
	}
	for i := 1; i <= 3; i++ {
		vmem[i] = vmem[0]
	}
	axspw, err := ioutil.ReadFile("axspw.txt")
	if err != nil {
		fmt.Println("File read error: ", err)
	}
	exepw, err := ioutil.ReadFile("exepw.txt")
	if err != nil {
		fmt.Println("File read error: ", err)
	} // end of initialization section
	http.HandleFunc("/vram~"+string(axspw)+"/", vramHandler)
	http.HandleFunc("/vram~"+string(exepw)+"/", vramHandler)
	http.HandleFunc("/click~"+string(exepw)+"/", clickHandler)
	http.Handle("/", http.FileServer(http.Dir("pub")))
	err = http.ListenAndServeTLS(":25681", "cert.pem", "key.pem", nil)
	if err != nil {
		fmt.Println("ListenAndServeTLS error: ", err)
	}
	go printer() // printer handles all print requests
	return printchan, clickchan
}

func printer() {
	for { // receive next print request:
		txt := <-printchan
		txtlen := len(txt)
		txtind := 0
		if txtlen > 2500 {
			continue
		} // ignore request if too long
		// set default formatting:
		var row = byte(curpos/80 + 1)    // row, between 1 and 26
		var col = byte(curpos%80 + 1)    // column, between 1 and 80
		var bgcolr byte = defcolors >> 4 // default background color
		var fgcolr byte = defcolors & 15 // default foreground color
		newline := true                  // defaults to true when there's no formatting code
		scroll := true                   // defaults to true when there's no formatting code
		showchanges := true              // defaults to true when there's no formatting code
		if txt[0] == byte('<') {         // formatting code present enclosed in <>
			// NOTE: if txt starts with '<', it MUST contain formatting code
			// if you just want to print to screen something that starts with <
			// then put (even empty) formatting code before it, for example
			// to print "<<HELLO!>>" use: printchan <- []byte("<><<HELLO!>>")
			// or to show it immediately: printchan <- []byte("<$><<HELLO!>>")
			newline = false     // defaults to false if there's formatting code
			scroll = false      // defaults to false if there's formatting code
			showchanges = false // defaults to false if there's formatting code
			var num byte = 200  // for reading numeric values in formatting code
			for txtind = 1; txtind < txtlen; txtind++ {
				if txt[txtind] == byte('>') { // end of formatting code
					break
				}
				switch txt[txtind] {
				case byte('0'), byte('1'), byte('2'), byte('3'), byte('4'),
					byte('5'), byte('6'), byte('7'), byte('8'), byte('9'):
					if num == 200 {
						num = txt[txtind] - byte('0') // 1st digit
					} else if num >= 10 {
						num = 100 // number too big
					} else {
						num = num*10 + txt[txtind] - byte('0') // adds digit
					}
				case byte('Y'), byte('y'):
					if num >= 1 && num <= 26 {
						row = num
					}
					num = 200 // non numeric char
				case byte('X'), byte('x'):
					if num >= 1 && num <= 80 {
						col = num
					}
					num = 200 // non numeric char
				case byte('B'), byte('b'):
					if num <= 15 {
						bgcolr = num
					}
					num = 200 // non numeric char
				case byte('F'), byte('f'):
					if num <= 15 {
						fgcolr = num
					}
					num = 200 // non numeric char
				case byte('/'):
					newline = true
					num = 200 // non numeric char
				case byte('^'):
					scroll = true
					num = 200 // non numeric char
				case byte('$'):
					showchanges = true
					num = 200 // non numeric char
				default:
					num = 200 // non numeric char
				}
			}
			if row == 26 {
				col = 1
			} // greatest possible cursor position
			txtind++ // now index of first char to print, if present
		}
		if txtind > txtlen {
			continue
		} // stop: there was no closing '>'
		txtstartpos := 80*int(row) + int(col) - 81
		txtendpos := txtstartpos + txtlen - txtind
		txtcolors := bgcolr<<4 + fgcolr
		newcurpos := txtendpos
		nextlinepos := (txtendpos + 79) / 80 * 80
		if (nextlinepos == txtstartpos) && newline {
			nextlinepos += 80
		}
		if newline {
			newcurpos = nextlinepos
		}
		writeindex := readindex + 1
		wrind := writeindex & 3             // modular vmem write index
		if scroll && (nextlinepos > 2000) { // go ahead and scroll
			scrollbychars := nextlinepos - 2000
			txtstartpos -= scrollbychars
			txtendpos -= scrollbychars
			newcurpos -= scrollbychars
			for ch := 0; ch < 2000; ch++ {
				if ch < txtstartpos {
					vmem[wrind][ch] = vmem[wrind][ch+scrollbychars]
					vmem[wrind][ch+2000] = vmem[wrind][ch+2000+scrollbychars]
				} else if ch >= txtendpos {
					vmem[wrind][ch] = byte(' ')
					vmem[wrind][ch+2000] = defcolors
				}
			}
		}
		for ch := txtstartpos; txtind < txtlen; txtind++ { // loops over actual text to print
			if ch >= 2000 {
				break
			}
			if ch >= 0 {
				vmem[wrind][ch] = txt[txtind]
				vmem[wrind][ch+2000] = txtcolors
			}
			ch++
		}
		curpos = newcurpos
		if showchanges {
			atomic.StoreUint32(&readindex, writeindex)
		} // updates readindex
	}
}

func Println(txt []byte) {

}

func PrintAt(row, col, bgcolr, fgcolr byte, scroll bool, txt []byte) {

}

func PrintlnAt(row, col, bgcolr, fgcolr byte, scroll bool, txt []byte) {

}

func Update() { // updates vram to show changes on screen
	printchan <- []byte("<$>")
}

func Vram() [4000]byte { // returns a copy of the current value of vram
	ind := atomic.LoadUint32(&readindex)
	return vmem[ind&3]
}