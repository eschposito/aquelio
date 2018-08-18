package main

// tests webvga interface, and shows complete color set and char set
import (
	"fmt"

	"../webvga"
)

func main() {
	fmt.Println("Loading...")
	printchan, clickchan := webvga.Serve(30, []byte("Try clicking on the screen..."))
	printchan <- []byte("<7y 12f / >Colors:") // OK to send on channel, not to receive
	for i := 0; i <= 15; i++ {                // draws all colors
		printchan <- []byte(fmt.Sprint("<", i, "b", 15-i, "f>     "))
	}
	printchan <- []byte("<12f / >Blinking/flashing:")
	for i := 0; i <= 15; i++ { // draws all colors with blinking/flashing
		printchan <- []byte(fmt.Sprint("<", i, "b", i, "f> Hi! "))
	}
	printchan <- []byte("<12f / >Charset:")
	for i := 1; i <= 8; i++ {
		printchan <- []byte("<>0123456789")
	}
	for i := 0; i < 256; i++ { // draws all chars
		printchan <- []byte{'<', '>', byte(i)}
	}
	printchan <- []byte("</$>") // goes to newline and displays all
	fmt.Println("...done")
	for {
		cp := <-clickchan
		printchan <- []byte(fmt.Sprintf("You clicked: x=%d, y=%d", cp[1], cp[0]))
	}
}
