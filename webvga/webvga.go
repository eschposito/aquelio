package webvga

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

var vmem [4][4000]byte     // 4 sectors: 1 is written to while 1 is displayed
var readindex uint32       // atomic access first index of displayed vmem sector
var curpos = 0             // current cursor position, from 0 to 2000 included
var defcolors byte         // default background and foreground colors
var printchan chan []byte  // channel used for sending print requests
var clickchan chan [2]byte // channel for getting click coordinates
var indy int               // start index of y (row) coord in click request path

func vramHandler(w http.ResponseWriter, req *http.Request) {
	ind := atomic.LoadUint32(&readindex)
	w.Write(vmem[ind&3][0:4000]) // sends data of sector to be displayed
}

func clickHandler(w http.ResponseWriter, req *http.Request) {
	pstr := req.URL.Path[indy : indy+5]                                     // path string slice containing click coords
	var clicpos [2]byte                                                     // row,column click coordinates
	clicpos[0] = 10*(byte(pstr[0])-byte('0')) + (byte(pstr[1]) - byte('0')) // Y (row)
	clicpos[1] = 10*(byte(pstr[3])-byte('0')) + (byte(pstr[4]) - byte('0')) // X (col)
	clickchan <- clicpos
	w.WriteHeader(200) // empty OK HTTP response
}

func Serve(defcols byte, greeting []byte) (chan<- []byte, <-chan [2]byte) {
	defcolors = defcols                // initialization section
	printchan = make(chan []byte, 10)  // remembers up to 10 print calls
	clickchan = make(chan [2]byte, 10) // remembers up to 10 clicks
	for i := range vmem[0] {
		if i >= 2000 {
			vmem[0][i] = defcols
		} else if i < len(greeting) {
			vmem[0][i] = greeting[i]
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
	indy = len(exepw) + 8 // start index of y (row) coord in click request
	if err != nil {
		fmt.Println("File read error: ", err)
	} // end of initialization section
	http.HandleFunc("/vram~"+string(axspw)+"/", vramHandler)
	http.HandleFunc("/vram~"+string(exepw)+"/", vramHandler)
	http.HandleFunc("/click~"+string(exepw)+"/", clickHandler)
	http.Handle("/priv_"+string(axspw)+"/", http.FileServer(http.Dir("priv")))
	http.Handle("/priv_"+string(exepw)+"/", http.FileServer(http.Dir("priv")))
	http.Handle("/verypriv_"+string(exepw)+"/", http.FileServer(http.Dir("verypriv")))
	http.Handle("/", http.FileServer(http.Dir("pub")))
	go http.ListenAndServeTLS(":25681", "cert.pem", "key.pem", nil)
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
		} // discard request if too long
		// set default formatting:
		var row = byte(curpos/80 + 1) // row, between 1 and 26
		var col = byte(curpos%80 + 1) // column, between 1 and 80
		var bgcolr = defcolors >> 4   // default background color
		var fgcolr = defcolors & 15   // default foreground color
		newline := true               // defaults to true when there's no formatting code
		scroll := true                // defaults to true when there's no formatting code
		showchanges := true           // defaults to true when there's no formatting code
		keepoldcolors := false        // if true, prints chars without modifying colors
		if txt[0] == byte('<') {      // formatting code present enclosed in <>
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
					} else if num == 200 {
						keepoldcolors = true // B with no number before it means: keep old colors
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
			for ch := 0; ch < txtstartpos; ch++ { // scroll up chars before text
				vmem[wrind][ch] = vmem[wrind][ch+scrollbychars]
				vmem[wrind][ch+2000] = vmem[wrind][ch+2000+scrollbychars]
			}
			for ch := txtendpos; ch < 2000; ch++ { // insert new whitespace after text
				vmem[wrind][ch] = byte(' ')
				vmem[wrind][ch+2000] = defcolors
			}
		}
		vind := txtstartpos // vind: vmem index for write operation
		if vind < 0 {
			vind = 0
			txtind -= txtstartpos
		}
		if txtendpos > 2000 {
			txtendpos = 2000
		}
		for vind < txtendpos { // loops over actual text to print
			if scroll || (txt[txtind] != 0) {
				vmem[wrind][vind] = txt[txtind]
			} // if char code is 0 (and scroll is false) leaves same char
			if scroll || !keepoldcolors {
				vmem[wrind][vind+2000] = txtcolors
			} // if keepoldcolors is true (and scroll is false) leaves same colors
			vind++
			txtind++
		}
		curpos = newcurpos
		if showchanges { // updates readindex and makes new vmem write copy
			atomic.StoreUint32(&readindex, writeindex)
			writeindex = readindex + 1
			vmem[writeindex&3] = vmem[readindex&3]
		}
	}
}

func Vram() [4000]byte { // returns a copy of the current value of vram
	ind := atomic.LoadUint32(&readindex)
	return vmem[ind&3]
}
