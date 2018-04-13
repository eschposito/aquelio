package main

import (
	"./webvga"
	"fmt"
	//"sync/atomic"
	//"golang.org/x/exp/io/spi"
	//"time"
)

func main() {
	/*dev, err := spi.Open(&spi.Devfs{
		Dev:      "/dev/spidev0.0",
		Mode:     spi.Mode0,
		MaxSpeed: 300000, // speed of light ;)
	})
	if err != nil {
		fmt.Println(err)
	}
	defer dev.Close()


	err := dev.Tx(sendmsg, recvmsg)
	if err != nil {
		fmt.Println(err)
	} else {

	}*/
	fmt.Println("Starting up...")
	printchan, clickchan := webvga.Serve(30, "Funge!!!")
	printchan <- []byte("<3y 12f / $>ET VOILA'!") // OK to send
	fmt.Println("...done")
	//c:= <- printchan // not OK to receive
	//clickchan <- [2]byte{20,255} // not OK to send
	for {
		cp := <-clickchan // receive click position coordinates
		printchan <- []byte(fmt.Sprintf("<%dX %dY^/$>* X=%d,Y=%d", cp[1], cp[0], cp[1], cp[0]))
	}
}
