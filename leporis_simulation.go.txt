package main

//$ : lines removed in the simulation version
import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"time"

	"./webvga"
	//$ "github.com/tarm/serial"
)

func appendStringToFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	if err != nil {
		return err
	}
	return nil
}

func gettotals(t time.Time) (hswh, hgwh, dswh, dgwh, mswh, mgwh, yswh, ygwh, totswh, totgwh float64) {
	// INCOMPLETE: for now only reads latest values of totswh, totgwh
	file, err := os.Open("priv/datafile.txt")
	defer file.Close()
	if err != nil {
		fmt.Println("Error opening datafile:", err)
		return
	}
	var tstamp, deg int
	fileScanner := bufio.NewScanner(file)
	fileScanner.Scan()       // discard first line
	for fileScanner.Scan() { // read next line if available and process it
		_, err = fmt.Sscan(fileScanner.Text(), &tstamp, &deg, &totswh, &totgwh)
		if err != nil {
			fmt.Println("Error parsing datafile:", err)
			return
		}
	}
	return
}

func printscreen(pchan chan<- []byte, usepv, usegrid, heatnow bool) {
	///// draws complete screen \\\\\
	// clear screen:
	for i := 1; i <= 26; i++ {
		pchan <- []byte("                                                                                ")
	}
	// sun and grid indicators, and graph legend:
	pchan <- []byte("<11x 1y>*Sun  ")
	pchan <- []byte("<7f>~Grid ")
	pchan <- []byte("<10f/>Hourly")
	pchan <- []byte("<10x 14b 1f>   0W")
	pchan <- []byte("<17x 7b 1f>   0W")
	pchan <- []byte("<10f/> values>")
	pchan <- append([]byte("<23x 10f>(Wh,"), 176, 'C', ')')
	// vertical arrows:
	pchan <- append([]byte("<3y 11x>"), 218)
	pchan <- append([]byte("<18x 7f/>"), 218)
	pchan <- append([]byte("<11x>"), 210)
	pchan <- append([]byte("<18x 7f/>"), 210)
	pchan <- append([]byte("<11x>"), 25)
	pchan <- append([]byte("<18x 7f>"), 25)
	// temp indicator
	pchan <- []byte("<4y 13x 12f/>Temp")
	pchan <- append([]byte("<13x 12b 1f>--"), 176, 'C')
	// sun temp setting legend:
	pchan <- []byte("<12y 2x/>Sun temp")
	pchan <- []byte("<6x>set:")
	// grid temp setting legend:
	pchan <- []byte("<4y 20x 7f/>Grid temp")
	pchan <- []byte("<20x 7f>set:")
	// water heater drawing:
	pchan <- append([]byte("<6y 11x 15f/>"), 248, 222, 222, 222, 222, 222, 222, 129)
	for i := 1; i <= 9; i++ {
		pchan <- append([]byte("<11x 15f>"), 227)
		pchan <- []byte(fmt.Sprint("<9b 0f>__", 5*i+30, "__"))
		pchan <- append([]byte("<15f/>"), 227)
	}
	pchan <- append([]byte("<11x 15f />"), 132, 222, 142, 222, 222, 142, 222, 135)
	pchan <- []byte("<12x 4f>H")
	pchan <- append([]byte("<4b 15f>"), 210)
	pchan <- []byte("<15x 9f>C")
	pchan <- append([]byte("<9b 15f>"), 210)
	// right side of screen with black background:
	pchan <- []byte("<1y>") // top line
	for i := 1; i <= 23; i++ {
		pchan <- []byte("<30x 0b 10f>         :                                         ")
	}
	// status line (line 25):
	pchan <- append([]byte("<25y 1x 0b 10f>"), 202)
	pchan <- append([]byte("<0b 9f> "), 200)
	pchan <- append([]byte("<0b 9f>Aquelio"), 200)
	pchan <- []byte("<0b 10f> Status:        ")
	pchan <- append([]byte("<0b 4f>"), 31)         // 31: down pointing triangle
	pchan <- append([]byte("<0b 10f>y-m-d@h"), 30) // 30: up pointing triangle
	pchan <- append([]byte("<0b 4f> "), 31)
	pchan <- append([]byte("<0b 10f>*Sun"), 30)
	pchan <- append([]byte("<0b 4f> "), 31)
	pchan <- append([]byte("<0b 10f>~Grid"), 30)
	pchan <- append([]byte("<74x 0b 4f>"), 31)
	pchan <- append([]byte("<0b 10f>+Temp"), 30)
	if usepv {
		pchan <- []byte("<25y 39x 14b 10f>*Sun") // [*Sun] backlighting on
	} else {
		pchan <- []byte("<25y 39x 0b 14f>*Sun") // [*Sun] backlighting off
	}
	if usegrid {
		pchan <- []byte("<25y 46x 7b 10f>~Grid") // [~Grid] backlighting on
	} else {
		pchan <- []byte("<25y 46x 0b 7f>~Grid") // [~Grid] backlighting off
	}
	if heatnow {
		pchan <- []byte("<25y 75x 12b 10f>+Temp") // [+Temp] backlighting on
	} else {
		pchan <- []byte("<25y 75x 0b 12f>+Temp") // [+Temp] backlighting off
	}
	// bottom kWh readings legend:
	pchan <- []byte("<19y 1x/>   * kWh:")
	pchan <- []byte("< / >kWh last:")
	pchan <- []byte("<7f/>   ~ kWh:")
	pchan <- []byte("<7f >kWh last:")
	// finally, display all:
	pchan <- []byte("<$>")
}

func newscreen(printchan chan<- []byte, hour, day, month, year int) {
	// INCOMPLETE: only works for hourly values (if hour >= 0), and does not graph Wh values
	// reads data from logfile, then
	// redraws entire screen except status (last) line
	file, err := os.Open("priv/datafile.txt")
	defer file.Close()
	fileScanner := bufio.NewScanner(file)
	if err != nil {
		fmt.Println("Error opening datafile:", err)
		return
	}
	var timestamp, sunwh, gridwh, temp [26]int
	var tstamp, deg, swh, gwh int // same as above, but read from file line
	var ln int                    // screen line #: ln=1: 1st line, ln=24: last line before status line
	if hour >= 0 {                // hourly diagram up to hour
		for ln = 24; ln >= 0; ln-- {
			timestamp[ln] = hour + 100*day + 10000*month + 1000000*year // => yyyymmddhh
			hour--
			if hour < 0 {
				hour = 23
				day--
				if day < 1 {
					month--
					if month < 1 {
						month = 12
						year--
					}
					switch month {
					case 2:
						if year%4 == 0 {
							day = 29
						} else {
							day = 28
						}
					case 4, 6, 9, 11:
						day = 30
					default:
						day = 31
					}
				}
			}
		}
		_ = fileScanner.Scan() // read and discard first line with headers
		for ln = 0; ln < 25; ln++ {
			for tstamp <= timestamp[ln] { // tstamp=0 at first, then is read from file
				if tstamp == timestamp[ln] {
					temp[ln] = deg
				}
				sunwh[ln] = swh
				gridwh[ln] = gwh
				if fileScanner.Scan() { // read next line if available and process it
					var min int // minute
					_, err = fmt.Sscan(fileScanner.Text(), &tstamp, &min, &deg, &swh, &gwh)
					if err != nil {
						fmt.Println("Error parsing logfile:", err)
						return
					}
				} else {
					break // EOF, no more lines to read
				}
			}
			sunwh[ln+1] = sunwh[ln]
			gridwh[ln+1] = gridwh[ln]
		}
		printchan <- []byte("<1x 1y>") // cursor at 1,1
		for ln = 1; ln < 25; ln++ {    // now print to screen lines 1 to 24
			t := fmt.Sprint(timestamp[ln])
			tstr := t[:4] + "-" + t[4:6] + "-" + t[6:8] + " " + t[8:]
			swh = sunwh[ln] - sunwh[ln-1]
			gwh = gridwh[ln] - gridwh[ln-1]
			lntxt := fmt.Sprintf("%s: *%4d ~%4d Wh", tstr, swh, gwh)
			x := 30 // cursor pos in line
			for ; x <= temp[ln]; x++ {
				if x > 77 {
					break
				}
				lntxt += "="
			}
			for ; x <= 77; x++ {
				lntxt += " "
			}
			if temp[ln] == 0 {
				lntxt += "   " // temp of 0°C means it was not recorded
			} else {
				lntxt += fmt.Sprintf("%2d°", temp[ln])
			}
			printchan <- []byte(lntxt)
		}
	} else if day >= 0 { // daily diagram up to day

	} else if month >= 0 { // monthly diagram up to month

	} else { // yearly diagram up to year

	}
}

///////////////////$ GLOBALS:
var keyboardchan = make(chan []byte, 10)
var rtc = []byte("yymddhhmm")
var rec = []byte("s0000g0000P99OK@")
var mustanswer = false

func yesMaam() { // gets user keyboard input and sends it to yesSer
	reader := bufio.NewReader(os.Stdin)
	for { // keeps running until user quits
		text, _ := reader.ReadString('\n')
		inp := []byte(text)
		switch inp[0] {
		case 'S', 's', 'G', 'g', 'T', 't', 'P', 'p':
			keyboardchan <- inp
		case 'Q', 'q': // quit and close all
			keyboardchan <- []byte("quit")
			return
		}
	}
}

func yesSer() bool { // simulates serial string from Nucleo
	// 1) update rtc:
	t := time.Now()
	year, month, day := t.Date()
	hour, minute, _ := t.Clock()
	rtc[1] = byte(year%10 + 48)
	decade := year / 10
	rtc[0] = byte(decade%10 + 48)
	if month < 10 {
		rtc[2] = byte(month) + 48 // '1'...'9'
	} else {
		rtc[2] = byte(month) + 55 // 'A'...'F'
	}
	rtc[4] = byte(day%10 + 48)
	daydec := day / 10
	rtc[3] = byte(daydec%10 + 48)
	rtc[6] = byte(hour%10 + 48)
	hourdec := hour / 10
	rtc[5] = byte(hourdec%10 + 48)
	rtc[8] = byte(minute%10 + 48)
	mindec := minute / 10
	rtc[7] = byte(mindec%10 + 48)
	// 2) update rec:
	select {
	case keybd := <-keyboardchan: // get user keyboard input
		switch keybd[0] {
		case 'S', 's': // change sun data (eg 'S1100')
			for i := 0; i < 5; i++ {
				rec[i] = keybd[i]
			}
			mustanswer = true
		case 'G', 'g': // change grid data (eg 'g2300')
			for i := 0; i < 5; i++ {
				rec[i+5] = keybd[i]
			}
			mustanswer = true
		case 'P', 'p': // change parambyte (eg 'p080')
			rec[10] = 100*(keybd[1]-48) + 10*(keybd[2]-48) + keybd[3] - 48
			mustanswer = true
		case 'T', 't': // temperature (eg 'T48')
			for i := 0; i < 2; i++ {
				rec[i+11] = keybd[i+1]
			}
			mustanswer = true
		case 'q':
			return false // close all and quit
		}
	default: // use old value of rec if channel is empty
	}
	time.Sleep(2 * time.Second)
	return true
}

func main() {
	fmt.Println("Starting up...")
	printchan, clickchan := webvga.Serve(30, []byte("Loading..."))
	/*$ com := &serial.Config{Name: "/dev/serial0", Baud: 115200, ReadTimeout: 15 * time.Second}
	ser, err := serial.OpenPort(com)
	if err != nil {
		fmt.Println("Serial opening error:", err)
	} */             //$
	usepv := true    // enable use of PV
	usegrid := true  // enable use of grid
	heatnow := false // true if manual heating [+°C] was requested
	printscreen(printchan, usepv, usegrid, heatnow)
	fmt.Println("...done")
	buf := make([]byte, 320)         // buffer of chars read from Nucleo
	const startyear = 2020           // means startyear <= yyyy <= startyear+99
	const Rpv = 9.6                  // ohm rating of PV heater resistor
	const Rgrid = 35.5               // ohm rating of grid heater resistor
	var vpv, vgrid float64           // PV and grid voltage readings from Nucleo
	var temp, oldtemp int            // water temperature reading from Nucleo
	var yyyy, yy, mo, dd, hh, mi int // date & time read from Nucleo RTC
	var sunwatt, gridwatt float64    // instant power from sun and grid
	var hswh, hgwh float64           // hourly energy (Wh) from sun and grid
	var dswh, dgwh float64           // daily energy (Wh) from sun and grid
	var mswh, mgwh float64           // monthly energy (Wh) from sun and grid
	var yswh, ygwh float64           // yearly energy (Wh) from sun and grid
	var totswh, totgwh float64       // total energy (Wh) from sun and grid
	var pvheating, gridheating bool  // PV and grid heating actual on/off status
	var stusepv, stusegrid bool      // Nucleo settings for usepv and usegrid
	var stgridtemp byte              // set temp (5..67°C) for grid heating and Nucleo setting
	var getmoreinfo bool             // if true, reads and displays more info from Nucleo
	var msgstr []byte                // 3 char status/message from Nucleo
	var gridtemp [7][24]byte         // hourly table of grid temp settings (5..67°C), now initialize them:
	ff, err := os.Open("priv/gridtempsettings.txt")
	ffScanner := bufio.NewScanner(ff)
	if err != nil {
		fmt.Println("Error opening grid temp settings file:", err)
		return
	}
	ffScanner.Scan() // read and discard first line
	// from second line on, read grid temp settings data
	for d := 0; d < 7; d++ {
		ffScanner.Scan()
		for i := 0; i < 24; i++ {
			fmt.Sscan(ffScanner.Text(), &gridtemp[d][i])
			if gridtemp[d][i] < 5 {
				gridtemp[d][i] = 5
			} else if gridtemp[d][i] > 67 {
				gridtemp[d][i] = 67
			}
		}
	}
	ff.Close()
	t := time.Now()
	oldt := t
	//year, month, day := t.Date()
	//hour, minute, _ := t.Clock()
	fmt.Println(oldtemp) // remove this!
	//fmt.Println(year, month, day, hour, minute)
	//newscreen(printchan, hour, day, int(month), year) // print lines 1..24
	//hswh, hgwh, dswh, dgwh, mswh, mgwh, yswh, ygwh, totswh, totgwh = gettotals(t)
	go yesMaam()
	for { // infinite loop (unless user quits)
		var xclick, yclick byte             // click coords
		var deltasunwh, deltagridwh float64 // energy added in this cycle, set to 0
		for i := 1; i <= 10; i++ {
			select {
			case cp := <-clickchan: // receive click position coordinates y,x
				if cp[0] == 25 { // clicked on last line (status line)
					switch cp[1] {
					case 1: // clicked menu icon
					case 3: // clicked on water icon preceding Aquelio
						getmoreinfo = false
						printscreen(printchan, usepv, usegrid, heatnow) // redraw entire screen
					case 9: // clicked on letter i of Aquelio
						getmoreinfo = true
						printchan <- []byte("<25y 9x 9b 9f>i") // flash 'i'
					case 28: // time down icon
					case 29: // time 'Y'
					case 31: // time 'M'
					case 33: // time 'D'
					case 35: // time 'H'
					case 36: // time up icon
					case 38: // *Sun stop icon
						usepv = false
						printchan <- []byte("<25y 39x 0b 14f>*Sun") // [*Sun] backlighting off
					case 43: // *Sun start icon
						usepv = true
						printchan <- []byte("<25y 39x 14b 10f>*Sun") // [*Sun] backlighting on
					case 45: // ~Grid stop icon
						usegrid = false
						printchan <- []byte("<25y 46x 0b 7f>~Grid") // [~Grid] backlighting off
					case 51: // ~Grid start icon
						usegrid = true
						printchan <- []byte("<25y 46x 7b 10f>~Grid") // [~Grid] backlighting on
					case 74: // +Temp stop icon
						heatnow = false
						printchan <- []byte("<25y 75x 0b 12f>+Temp") // [+Temp] backlighting off
					case 80: // +Temp start icon
						heatnow = true
						printchan <- []byte("<25y 75x 12b 10f>+Temp") // [+Temp] backlighting on
					}
				} else { // clicked anywhere else on screen, save coords:
					yclick, xclick = cp[0], cp[1] // but only for last click
				}
			default:
				break // exit inner for loop if there are no clicks
			}
		}
		if xclick+yclick == 0 {
			xclick += yclick
		} // nonsense, to be removed!!!!
		bytesread := 0
		ch := byte('$') // ch will be the last char read, hopefully '@'
		/*$ for (bytesread < 25) || ((ch != '@') && (bytesread < 200)) {
			n, _ := ser.Read(buf[bytesread:])
			if n > 0 {
				bytesread += n        // update read char count
				ch = buf[bytesread-1] // last char read
			} else {
				fmt.Println("Serial read timeout!")
				break
			}
		} $*/bytesread = 25
		ch = '@'
		if !yesSer() { // end main loop and quit
			return
		} /////$

		oldt = t
		t = time.Now()            // looks up time after every serial read operation
		dt := t.Sub(oldt).Hours() // time interval (h) since last calculation
		if getmoreinfo {          // displays buffer contents on screen
			printchan <- []byte("<21y 1x>")
			printchan <- buf[:bytesread]
		}
		if (bytesread < 25) || (ch != '@') {
			continue // couldn't read from Nucleo, skip to next loop iteration
		} //////////// else, go on:
		var deciV int
		err = nil
		//$ rec := buf[bytesread-16 : bytesread]    // get last 16 char
		//$ rtc := buf[bytesread-25 : bytesread-16] // date & time info
		switch rec[0] {
		case byte('s'):
			pvheating = false
		case byte('S'):
			pvheating = true
		default:
			err = errors.New("Unexpected char in serial record")
		}
		_, err = fmt.Sscan(string(rec[1:5]), &deciV) // PV voltage in deciVolts
		if err != nil {
			vpv = 0
		} else {
			vpv = float64(deciV) / 10
		}
		switch rec[5] {
		case byte('g'):
			gridheating = false
		case byte('G'):
			gridheating = true
		default:
			err = errors.New("Unexpected char in serial record")
		}
		_, err = fmt.Sscan(string(rec[6:10]), &deciV) // grid voltage in deciVolts
		if err != nil {
			vgrid = 0
		} else {
			vgrid = float64(deciV) / 10
		}
		stparambyte := rec[10]
		stusepv = (stparambyte & 128) == 128          // Nucleo PV enabled? (T/F)
		stusegrid = (stparambyte & 64) == 64          // Nucleo grid enabled? (T/F)
		stgridtemp = (stparambyte & 63) + 4           // Nucleo grid temp setting 5..67°C
		oldtemp = temp                                // keep old temp for writing to datafile
		_, err = fmt.Sscan(string(rec[11:13]), &temp) // water temperature in °C
		if err != nil {
			temp = 99
		}
		if getmoreinfo {
			printchan <- []byte("<1y 30x>")
			printchan <- []byte(string(rec[1:5]))
			printchan <- []byte(fmt.Sprintf("<1y 40x>%4.0f V; parambyte= %d", vpv, rec[10]))
			printchan <- []byte("<2y 30x>")
			printchan <- []byte(string(rec[11:13]))
			printchan <- []byte(fmt.Sprintf("<2y 40x>%d degC", temp))
			printchan <- []byte("<3y 30x>")
			printchan <- []byte(string(rec[6:10]))
			printchan <- []byte(fmt.Sprintf("<3y 40x>%4.0f Vca", vgrid))
		}

		msgstr = rec[13:]
		if string(msgstr) != "OK@" {
			// "OK@" is the usual value for msgstr
			fmt.Println("Nucleo not OK")
		}
		_, err = fmt.Sscanf(string(rtc), "%2d%1x%2d%2d%2d", &yy, &mo, &dd, &hh, &mi)
		if err == nil {
			yyyy = startyear/100 + yy
			if yyyy < startyear {
				yyyy += 100
			}
		}
		if pvheating {
			sunwatt = vpv * vpv / Rpv
			deltasunwh = sunwatt * dt
		} else {
			sunwatt = 0
		}
		if gridheating {
			gridwatt = vgrid * vgrid / Rgrid
			deltagridwh = gridwatt * dt
		} else {
			gridwatt = 0
		}
		var serbyte byte // set to 0
		if heatnow {     // temporarily overrides usepv & usegrid and sets temp to 67
			if temp >= 67 { // if water is hot, back to normal operation
				heatnow = false
				printchan <- []byte("<25y 75x 0b 12f>+Temp") // [+Temp] backlighting off
			} else if !stusepv || !stusegrid {
				serbyte = 3 // tell Nucleo to use both PV and grid
			} else if stgridtemp != 67 {
				serbyte = 67 // tell Nucleo to set temp to max (67°C)
			} else {
				serbyte = 200 // everything OK, nothing to communicate
			}
		}
		if !heatnow { // then serbyte is still 0
			if (stusepv != usepv) || (stusegrid != usegrid) {
				if usepv {
					serbyte += 2
				}
				if usegrid {
					serbyte++
				}
			} else if stgridtemp != gridtemp[hh] {
				serbyte = gridtemp[hh]
			} else {
				serbyte = 200 // everything OK, nothing to communicate
			}
		}
		if (serbyte == 200) && getmoreinfo {
			serbyte = 4 // tell Nucleo to send complete data
		}
		//$ ser.Write([]byte{serbyte})
		if mustanswer {
			fmt.Println("serbyte=", serbyte)
			mustanswer = false
		}
		/*if hh != hour { // hh is the latest reading, hour is the preceding one
			// first append line to datafile with last hour's data:
			ln := fmt.Sprintf("%d %2d %d %d/n", hour+100*day+1e4*int(month)+1e6*year, oldtemp, int(totswh), int(totgwh))
			appendStringToFile("priv/datafile.txt", ln)
			// then scroll up screen by one line:
			printchan <- []byte("<26y/^>")
			//hourchange = true
			hour = hh
			minute = min
			hswh = 0
			hgwh = 0
			yy, mm, dd := t.Date() // new date
			if dd != day {
				day = dd
				dswh = 0
				dgwh = 0
				if mm != month {
					month = mm
					mswh = 0
					mgwh = 0
					if yy != year {
						year = yy
						yswh = 0
						ygwh = 0
					}
				}
			}
		}*/
		// increase sun and grid energy counters:
		hswh += deltasunwh
		dswh += deltasunwh
		mswh += deltasunwh
		yswh += deltasunwh
		totswh += deltasunwh
		hgwh += deltagridwh
		dgwh += deltagridwh
		mgwh += deltagridwh
		ygwh += deltagridwh
		totgwh += deltagridwh
		// print data to screen:
		if pvheating {
			printchan <- []byte(fmt.Sprintf("<2y 10x 14b 1f>%4.0fW", sunwatt))
		} else {
			printchan <- []byte(fmt.Sprintf("<2y 10x 14b 1f>%4.0fV", vpv))
		}
		if gridheating {
			printchan <- []byte(fmt.Sprintf("<2y 17x 7b 1f>%4.0fW", gridwatt))
		} else {
			printchan <- []byte(fmt.Sprintf("<2y 17x 7b 1f>%4.0fV", vgrid))
		}
		// with last print request, show all:
		printchan <- []byte(fmt.Sprintf("<5y 13x 12b 1f $>%2d", temp))
		/*
			// print lines 24 and 25; first build line 24:
			ln24 := fmt.Sprintf("%d%d: *%4d ~%4d Wh ", hour/10, hour%10, int(hswh), int(hgwh))
			x := 20 // cursor pos in line
			for ; x <= temp; x++ {
				if x > 77 {
					break
				}
				ln24 += "="
			}
			for ; x <= 77; x++ {
				ln24 += " "
			}
			if temp == 0 { // temp of 0°C means it was not recorded
				ln24 += "   "
			} else {
				ln24 += fmt.Sprintf("%2d°", temp)
			}
			// now build line 25:
			ts := fmt.Sprint(hour + 100*day + 1e4*int(month) + 1e6*year)
			tstring := ts[:4] + "-" + ts[4:6] + "-" + ts[6:8] + " " + ts[8:]
			ln25 := fmt.Sprintf("%s:%d%d *%4d ~%4d W =Aquelio= [D]:*%7d ~%7dkWh[*][~][+°C]|^^=", tstring, minute/10, minute%10, int(sunwatt), int(gridwatt), int(dswh), int(dgwh))
			// finally print:
			printchan <- []byte("<24y 1x>" + ln24)
			printchan <- []byte("<0b 2f $>" + ln25)*/

	} // end of infinite loop
}
