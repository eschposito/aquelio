package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"time"

	"./webvga"

	"github.com/tarm/serial"
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

func main() {
	fmt.Println("Starting up...")
	printchan, clickchan := webvga.Serve(30, []byte("Loading..."))
	printchan <- []byte("<7y 12f / >Charset:") // OK to send on channel, not to receive
	for i := 0; i < 256; i++ {
		printchan <- []byte{byte('<'), byte('>'), byte(i)}
	}
	printchan <- []byte("<$>")
	fmt.Println("...done")
	com := &serial.Config{Name: "/dev/serial0", Baud: 115200, ReadTimeout: 15 * time.Second}
	ser, err := serial.OpenPort(com)
	if err != nil {
		fmt.Println("Serial opening error:", err)
	}
	buf := make([]byte, 160)          // buffer of chars read from Nucleo
	const Rpv, Rgrid = 9.6, 35.5      // ohm rating of heater resistors
	var vpv, vgrid, temp, oldtemp int // voltage and temperature readings from Nucleo
	var sunwatt, gridwatt float64     // instant power from sun and grid
	var hswh, hgwh float64            // hourly energy (Wh) from sun and grid
	var dswh, dgwh float64            // daily energy (Wh) from sun and grid
	var mswh, mgwh float64            // monthly energy (Wh) from sun and grid
	var yswh, ygwh float64            // yearly energy (Wh) from sun and grid
	var totswh, totgwh float64        // total energy (Wh) from sun and grid
	var pvheating, gridheating bool   // PV and grid heating actual on/off status
	var usepv, stusepv bool           // enable use of PV (and previous value) and Nucleo setting
	var usegrid, stusegrid bool       // enable use of grid (and previous value) and Nucleo setting
	var stgridtemp byte               // set temp (5..68°C) for grid heating and Nucleo setting
	//var hourchange bool             // true if clock hour has just changed
	var heatnow bool      // true if manual heating [+°C] was requested
	var msgstr []byte     // 3 char status/message from Nucleo
	var gridtemp [24]byte // hourly table of grid temp settings (5..68°C), now initialize them:
	ff, err := os.Open("priv/gridtempsettings.txt")
	ffScanner := bufio.NewScanner(ff)
	if err != nil {
		fmt.Println("Error opening grid temp settings file:", err)
		return
	}
	ffScanner.Scan() // read and discard first line
	ffScanner.Scan() // second line contains grid temp settings data
	for i := 0; i < 24; i++ {
		fmt.Sscan(ffScanner.Text(), &gridtemp[i])
		if gridtemp[i] < 5 {
			gridtemp[i] = 5
		} else if gridtemp[i] > 68 {
			gridtemp[i] = 68
		}
	}
	ff.Close()
	// here system clock should be updated with RTC
	t := time.Now()
	oldt := t
	year, month, day := t.Date()
	hour, minute, _ := t.Clock()
	newscreen(printchan, hour, day, int(month), year) // print lines 1..24
	hswh, hgwh, dswh, dgwh, mswh, mgwh, yswh, ygwh, totswh, totgwh = gettotals(t)
	for {
		var togglesun, togglegrid, toggleheat bool // set to false
		var dsunwh, dgridwh float64                // energy added in this cycle, set to 0
		for i := 1; i <= 10; i++ {
			select {
			case cp := <-clickchan: // receive click position coordinates y,x
				if cp[0] == 25 { // clicked on last line (status line)
					switch cp[1] {
					case 66, 67, 68: // clicked on [*]
						togglesun = true
					case 69, 70, 71: // clicked on [~]
						togglegrid = true
					case 72, 73, 74, 75, 76: // clicked on [+°C]
						toggleheat = true
					}
				}
			default:
				break // exit inner for loop if there are no clicks
			}
		}
		bytesread := 0
		for bytesread < 16 {
			n, _ := ser.Read(buf[bytesread:])
			if n > 0 {
				bytesread += n
			} else {
				fmt.Println("Serial read timeout!")
				break
			}
		}
		if bytesread >= 16 {
			err = nil
			rec := buf[bytesread-16 : bytesread] // get last 16 chars
			//fmt.Println("Msg read from serial:", string(rec))
			switch rec[0] {
			case byte('s'):
				pvheating = false
			case byte('S'):
				pvheating = true
			default:
				err = errors.New("Unexpected char in serial record")
			}
			_, err = fmt.Sscan(string(rec[1:5]), vpv) // PV voltage in deciVolts
			if err != nil {
				vpv = 0
			}
			switch rec[5] {
			case byte('g'):
				gridheating = false
			case byte('G'):
				gridheating = true
			default:
				err = errors.New("Unexpected char in serial record")
			}
			_, err = fmt.Sscan(string(rec[6:10]), vgrid) // grid voltage in deciVolts
			if err != nil {
				vgrid = 0
			}
			stparambyte := rec[10]
			stusepv = (stparambyte & 128) == 128         // Nucleo PV eabled? (T/F)
			stusegrid = (stparambyte & 64) == 64         // Nucleo grid eabled? (T/F)
			stgridtemp = (stparambyte & 63) + 5          // Nucleo grid temp setting 5..68°C
			oldtemp = temp                               // keep old temp for writing to datafile
			_, err = fmt.Sscan(string(rec[11:13]), temp) // water temperature in °C
			if err != nil {
				temp = 0
			}
			msgstr = rec[13:]
			if string(msgstr) != "OK!" {
				// "OK!" is the usual value for msgstr
			}
			oldt = t
			t = time.Now()            // looks up time after every serial read operation
			dt := t.Sub(oldt).Hours() // time interval (h) since last calculation
			if pvheating {
				sunwatt = float64(vpv*vpv) / 100 / Rpv
				dsunwh = sunwatt * dt
			} else {
				sunwatt = 0
			}
			if gridheating {
				gridwatt = float64(vgrid*vgrid) / 100 / Rgrid
				dgridwh = gridwatt * dt
			} else {
				gridwatt = 0
			}
		}
		if toggleheat {
			heatnow = !heatnow
		}
		if togglesun {
			usepv = !usepv
		}
		if togglegrid {
			usegrid = !usegrid
		}
		hh, min, _ := t.Clock() // new time
		var serbyte byte        // set to 0
		if heatnow {            // temporarily overrides usepv & usegrid and sets temp to 68
			if temp >= 68 { // if water is hot, back to normal operation
				heatnow = false
			} else if !stusepv || !stusegrid {
				serbyte = 3 // tell Nucleo to use both PV and grid
			} else if stgridtemp != 68 {
				serbyte = 68 // tell Nucleo to set temp to max (68°C)
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
		ser.Write([]byte{serbyte})
		if hh != hour { // hh is the latest reading, hour is the preceding one
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
		}
		// increase sun and grid energy counters:
		hswh += dsunwh
		dswh += dsunwh
		mswh += dsunwh
		yswh += dsunwh
		totswh += dsunwh
		hgwh += dgridwh
		dgwh += dgridwh
		mgwh += dgridwh
		ygwh += dgridwh
		totgwh += dgridwh
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
		printchan <- []byte("<0b 2f $>" + ln25)
	}
}
