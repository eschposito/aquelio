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

/*func gettotals(t time.Time) (hswh, hgwh, dswh, dgwh, mswh, mgwh, yswh, ygwh, totswh, totgwh float64) {
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
}*/

func readtempfile() [7][24]byte {
	ff, err := os.Open("priv/gridtempsettings.txt")
	ffScanner := bufio.NewScanner(ff)
	var gridt [7][24]byte
	if err != nil {
		fmt.Println("Error opening grid temp settings file:", err)
		for i := 0; i < 7; i++ {
			for j := 0; j < 24; j++ {
				gridt[i][j] = 5 // can't read file: initialize all values to 5°C
			}
		}
		return gridt
	}
	ffScanner.Scan() // read and discard first line
	ffScanner.Scan() // read and discard second line
	for linea := 0; linea < 7; linea++ {
		ffScanner.Scan() // ff lines 3-9 contain grid temp settings Sunday-Monday
		fmt.Sscan(ffScanner.Text(), &gridt[linea][0], &gridt[linea][1], &gridt[linea][2], &gridt[linea][3],
			&gridt[linea][4], &gridt[linea][5], &gridt[linea][6], &gridt[linea][7],
			&gridt[linea][8], &gridt[linea][9], &gridt[linea][10], &gridt[linea][11],
			&gridt[linea][12], &gridt[linea][13], &gridt[linea][14], &gridt[linea][15],
			&gridt[linea][16], &gridt[linea][17], &gridt[linea][18], &gridt[linea][19],
			&gridt[linea][20], &gridt[linea][21], &gridt[linea][22], &gridt[linea][23])
		for i := 0; i < 24; i++ {
			if gridt[linea][i] < 5 {
				gridt[linea][i] = 5
			} else if gridt[linea][i] > 67 {
				gridt[linea][i] = 67
			}
		}
	}
	ff.Close()
	return gridt
}

func writetempfile(gridt [7][24]byte) {
	ff, err := os.OpenFile("priv/gridtempsettings.txt", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error writing grid temp settings file:", err)
		return
	}
	_, err = ff.WriteString("GRID TEMP SETTINGS (line 1: time; lines 2->7: Sun->Mon grid temps in °C)\n")
	_, err = ff.WriteString("00 01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 17 18 19 20 21 22 23\n")
	for linea := 0; linea < 7; linea++ {
		for i := 0; i < 24; i++ {
			_, err = ff.WriteString(fmt.Sprintf("%2d ", gridt[linea][i]))
		}
		ff.WriteString("\n")
	}
	if err != nil {
		fmt.Println("Error writing grid temp settings file:", err)
	}
	ff.Close()
}

func printsetscreen(pchan chan<- []byte, gridt [7][24]byte) {
	///// draws settings screen \\\\\
	// clear screen:
	for i := 1; i <= 26; i++ {
		pchan <- []byte("                                                                                ")
	}
	// print temp settings:
	pchan <- []byte("<1X 1Y/>GRID TEMPERATURE SETTINGS (°C):")
	pchan <- []byte("h-> 00 01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 17 18 19 20 21 22 23")
	pchan <- []byte("Sun")
	pchan <- []byte("Mon")
	pchan <- []byte("Tue")
	pchan <- []byte("Wed")
	pchan <- []byte("Thu")
	pchan <- []byte("Fri")
	pchan <- []byte("Sat")
	for i := 0; i < 7; i++ {
		for j := 0; j < 24; j++ {
			pchan <- []byte(fmt.Sprintf("<%dY %dX>%2d", i+3, j*3+5, gridt[i][j]))
		}
	}
	// print date and time:
	pchan <- []byte("<11Y 1X/>Date and time:  yyyy-mm-dd hh:mm:[00]") // date&time will update at evey iteration
	pchan <- []byte("** Note: date and time are changed in real time **")
	pchan <- []byte("** Click [00] to zero seconds to closest minute **")
	// print bottom keys:
	pchan <- []byte("<14Y 1X/>  ")
	pchan <- []byte("++++    ----    EXIT    EXIT        QUIT")
	pchan <- []byte("++++    ----    &&&&    DONT        FROM")
	pchan <- []byte("++++    ----    SAVE    SAVE        APP!")
	// finally, print instructions and display all:
	pchan <- []byte("<19Y 1X $>^^ Usage: select value to change then click keys ^^")
}

func printscreen(pchan chan<- []byte, usepv, usegrid, heatnow bool, degC byte, msg []byte) {
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
	pchan <- []byte("<2x>set: 77")
	// grid temp setting legend:
	pchan <- []byte("<4y 20x 7f/>Grid temp")
	pchan <- []byte(fmt.Sprintf("<20x 7f>set: %2d", degC))
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
	for i := 1; i <= 24; i++ {
		pchan <- []byte("<30x 0b 10f>         :                                         ")
	}
	// status line (line 25):
	pchan <- append([]byte("<25y 1x 0b 10f>"), 202)
	pchan <- append([]byte("<0b 9f> "), 200)
	pchan <- append([]byte("<0b 9f>Aquelio"), 200)
	pchan <- []byte("<0b 10f> Status: " + string(msg) + "    ")
	pchan <- append([]byte("<1b 4f>"), 31)         // 31: down pointing triangle
	pchan <- append([]byte("<1b 10f>y-m-d@h"), 30) // 30: up pointing triangle
	pchan <- append([]byte("<1b 4f> "), 31)
	pchan <- append([]byte("<1b 10f>*Sun"), 30)
	pchan <- append([]byte("<1b 4f> "), 31)
	pchan <- append([]byte("<1b 10f>~Grid"), 30)
	pchan <- append([]byte("<74x 1b 4f>"), 31)
	pchan <- append([]byte("<1b 10f>+Temp"), 30)
	if usepv {
		pchan <- []byte("<25y 39x 14b 10f>*Sun") // [*Sun] backlighting on
	} else {
		pchan <- []byte("<25y 39x 1b 14f>*Sun") // [*Sun] backlighting off
	}
	if usegrid {
		pchan <- []byte("<25y 46x 7b 10f>~Grid") // [~Grid] backlighting on
	} else {
		pchan <- []byte("<25y 46x 1b 7f>~Grid") // [~Grid] backlighting off
	}
	if heatnow {
		pchan <- []byte("<25y 75x 12b 10f>+Temp") // [+Temp] backlighting on
	} else {
		pchan <- []byte("<25y 75x 1b 12f>+Temp") // [+Temp] backlighting off
	}
	// bottom kWh readings legend:
	pchan <- []byte("<19y 1x/>   * kWh:")
	pchan <- []byte("< / >kWh last:")
	pchan <- []byte("<7f/>   ~ kWh:")
	pchan <- []byte("<7f >kWh last:")
	// finally, display all:
	pchan <- []byte("<$>")
}

func newscreendata(printchan chan<- []byte, year, month, day, hour int64) {
	// INCOMPLETE: only works for hourly values (if hour >= 0), and does not graph Wh and temp values
	// reads data from logfile, then
	// redraws right side of entire screen except status (last) line
	file, err := os.Open("priv/datafile.txt")
	defer file.Close()
	fileScanner := bufio.NewScanner(file)
	if err != nil {
		fmt.Println("Error opening datafile:", err)
		return
	}
	var timestamp, sunwh, gridwh, temp [25]int64
	var tstamp, deg, swh, gwh int64 // same as above, but read from file line
	var ln int                      // screen line #: ln=1: 1st line, ln=24: last line before status line
	if hour >= 0 {                  // hourly diagram up to hour, where 0 <= hour <= 24
		for ln = 24; ln >= 0; ln-- {
			if hour == 0 {
				hour = 24
				day--
				if day == 0 {
					month--
					if month == 0 {
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
			timestamp[ln] = hour + 100*day + 10000*month + 1000000*year // => yyyymmddhh
			hour--
		}
		_ = fileScanner.Scan() // read and discard first line with headers
		for ln = 1; ln < 25; ln++ {
			for tstamp <= timestamp[ln] { // tstamp=0 at first, then is read from file
				if tstamp == timestamp[ln] {
					sunwh[ln] = swh
					gridwh[ln] = gwh
					temp[ln] = deg
				}
				if fileScanner.Scan() { // read next line if available and process it
					_, err = fmt.Sscan(fileScanner.Text(), &tstamp, &swh, &gwh, &deg)
					if err != nil {
						fmt.Println("Error parsing datafile:", err)
						return
					}
				} else {
					break // EOF, no more lines to read
				}
			}
		}
		for ln = 1; ln < 25; ln++ { // now print to screen lines 1 to 24
			t := fmt.Sprint(timestamp[ln])
			tstr := t[:4] + "-" + t[4:6] + "-" + t[6:8] + " " + t[8:]
			lntxt := fmt.Sprintf("<31x %dy b>%s: *%4d ~%4d Wh, %2d°C", ln, tstr, sunwh[ln], gridwh[ln], temp[ln])
			printchan <- []byte(lntxt)
		}
		// finally, display all:
		printchan <- []byte("<$>")
	} else if day >= 0 { // daily diagram up to day

	} else if month >= 0 { // monthly diagram up to month

	} else { // yearly diagram up to year

	}
}

func main() {
	fmt.Println("Starting up...")
	printchan, clickchan := webvga.Serve(30, []byte("Loading..."))
	com := &serial.Config{Name: "/dev/serial0", Baud: 115200, ReadTimeout: 15 * time.Second}
	ser, err := serial.OpenPort(com)
	if err != nil {
		fmt.Println("Serial opening error:", err)
	}
	usepv := true            // enable use of PV
	usegrid := true          // enable use of grid
	heatnow := false         // true if manual heating [+°C] was requested
	buf := make([]byte, 320) // buffer of chars read from Nucleo
	const Rpv = 9.6          // ohm rating of PV heater resistor
	const Rgrid = 35.5       // ohm rating of grid heater resistor
	const startyear = 1970   // means startyear <= yyyy <= startyear+99
	var timest int64 = (startyear + 200) * 1000000
	// timest: date & time combination from Nucleo RTC, here initialized with a higher value
	var vpv, vgrid float64                 // PV and grid voltage readings from Nucleo
	var temp, wd int                       // water temperature reading from Nucleo and weekday
	var yyyy, yy, mo, dd, hh, mi int64     // date & time read from Nucleo RTC
	var oldyyyy, oldmo, olddd, oldhh int64 // old values from previous iteration
	var sunwatt, gridwatt float64          // instant power from sun and grid
	var hswh, hgwh float64                 // hourly energy (Wh) from sun and grid
	var dswh, dgwh float64                 // daily energy (Wh) from sun and grid
	var mswh, mgwh float64                 // monthly energy (Wh) from sun and grid
	var yswh, ygwh float64                 // yearly energy (Wh) from sun and grid
	var totswh, totgwh float64             // total energy (Wh) from sun and grid
	var pvheating, gridheating bool        // PV and grid heating actual on/off status
	var stusepv, stusegrid bool            // Nucleo settings for usepv and usegrid
	var stgridtemp byte                    // set temp (5..67°C) for grid heating and Nucleo setting
	var getmoreinfo bool                   // if true, reads and displays more info from Nucleo
	var msgstr []byte                      // 3 char status/message from Nucleo
	var screentime int                     // time counter for settings screen
	var numsel byte                        // number of °C to add to selected temperature setting
	var xsel, ysel byte                    // coordinates of selected character in settings screen
	var hsel byte                          // selected hour index in settings screen
	var gridtemp [7][24]byte               // hourly table of grid temp settings (5..67°C), now initialize them:
	gridtemp = readtempfile()
	newgridtemp := gridtemp // new values of grid temps, not yet saved
	printscreen(printchan, usepv, usegrid, heatnow, 0, []byte("---"))
	type ScreenType int
	const (
		NormalScreen ScreenType = iota
		SettingsScreen
	)
	screen := NormalScreen
	t := time.Now() // t and oldt are Raspberry time, only used to calculate time intervals
	oldt := t
	fmt.Println("...done")
	for { // infinite loop
		///////////// GET SCREEN CLICKS: \\\\\\\\\\\\\
		var xclick, yclick byte             // click coords, set to 0
		var deltasunwh, deltagridwh float64 // energy added in this cycle, set to 0
		var clickserbyte byte               // (set to 0) byte to be transmitted due to user clicking in settings screen
		switch screen {
		case SettingsScreen:
			printchan <- []byte(fmt.Sprintf("<B 11Y 17X $>%4d-%02d-%02d %2d:%02d", yyyy, mo, dd, hh, mi)) // print date&time at each iteration
			select {
			case cp := <-clickchan: // receive click position coordinates y,x
				yclick, xclick = cp[0], cp[1]
				screentime = 0 // reset timer when user clicks
				switch yclick {
				case 3, 4, 5, 6, 7, 8, 9: // grid temp settings rows, in order from Sunday to Monday
					switch xclick {
					case 5, 8, 11, 14, 17, 20, 23, 26, 29, 32, 35, 38, 41, 44, 47, 50, 53, 56, 59, 62, 65, 68, 71, 74: // high digit
						if (xsel > 0) && (ysel > 0) { // remove blinking from old cursor position
							printchan <- append([]byte(fmt.Sprintf("<1B 14F %dX %dY>", xsel, ysel)), 0)
						}
						xsel, ysel = xclick, yclick
						printchan <- append([]byte(fmt.Sprintf("<14B 14F %dX %dY>", xsel, ysel)), 0) // blink!
						hsel = (xsel - 5) / 3                                                        // selected hour index (0-23)
						numsel = 10                                                                  // how much to add or subtract to selected temperature
					case 6, 9, 12, 15, 18, 21, 24, 27, 30, 33, 36, 39, 42, 45, 48, 51, 54, 57, 60, 63, 66, 69, 72, 75: // low digit
						if (xsel > 0) && (ysel > 0) { // remove blinking from old cursor position
							printchan <- append([]byte(fmt.Sprintf("<1B 14F %dX %dY>", xsel, ysel)), 0)
						}
						xsel, ysel = xclick, yclick
						printchan <- append([]byte(fmt.Sprintf("<14B 14F %dX %dY>", xsel, ysel)), 0) // blink!
						hsel = (xsel - 6) / 3                                                        // selected hour index (0-23)
						numsel = 1                                                                   // how much to add or subtract to selected temperature
					}
				case 11: // date and time row
					switch xclick {
					case 19, 20, 23, 26, 29, 32: // change date & time by +/- 10yrs, 1yr, 1month, 1day, 1 hr, 1min
						if (xsel > 0) && (ysel > 0) { // remove blinking from old cursor position
							printchan <- append([]byte(fmt.Sprintf("<1B 14F %dX %dY>", xsel, ysel)), 0)
						}
						xsel, ysel = xclick, yclick
						printchan <- append([]byte(fmt.Sprintf("<14B 14F %dX %dY>", xsel, ysel)), 0) // blink!
					case 35, 36: // reset seconds to 00, rounding to closest whole minute
						if (xsel > 0) && (ysel > 0) { // remove blinking from old cursor position
							printchan <- append([]byte(fmt.Sprintf("<1B 14F %dX %dY>", xsel, ysel)), 0)
						}
						xsel, ysel = 0, 0 // reset selection position
						clickserbyte = 80 // serial code to tell Nucleo to set time to closest 00 seconds
					}
				case 15, 16, 17: // bottom row of buttons (pressing these buttons does't change xsel,ysel)
					switch xclick {
					case 1, 2, 3, 4: // + button
						switch ysel {
						case 3, 4, 5, 6, 7, 8, 9: // grid temp settings rows, in order from Sunday to Monday
							newgridtemp[ysel-3][hsel] += numsel
							if newgridtemp[ysel-3][hsel] < 5 {
								newgridtemp[ysel-3][hsel] = 5
							} else if newgridtemp[ysel-3][hsel] > 67 {
								newgridtemp[ysel-3][hsel] = 67
							} // and now, reprint temperature setting:
							printchan <- []byte(fmt.Sprintf("<B %dY %dX>%2d", ysel, hsel*3+5, newgridtemp[ysel-3][hsel]))
						case 11: // date and time row
							switch xsel {
							case 19:
								clickserbyte = 69 // serial code for +10 years
							case 20:
								clickserbyte = 71 // serial code for +1 year
							case 23:
								clickserbyte = 73 // serial code for +1 month
							case 26:
								clickserbyte = 75 // serial code for +1 day
							case 29:
								clickserbyte = 77 // serial code for +1 hour
							case 32:
								clickserbyte = 79 // serial code for +1 minute
							}
						}
					case 9, 10, 11, 12: // - button
						switch ysel {
						case 3, 4, 5, 6, 7, 8, 9: // grid temp settings rows, in order from Sunday to Monday
							newgridtemp[ysel-3][hsel] -= numsel
							if newgridtemp[ysel-3][hsel] < 5 {
								newgridtemp[ysel-3][hsel] = 5
							} else if newgridtemp[ysel-3][hsel] > 67 {
								newgridtemp[ysel-3][hsel] = 67
							} // and now, reprint temperature setting:
							printchan <- []byte(fmt.Sprintf("<B %dY %dX>%2d", ysel, hsel*3+5, newgridtemp[ysel-3][hsel]))
						case 11: // date and time row
							switch xsel {
							case 19:
								clickserbyte = 68 // serial code for -10 years
							case 20:
								clickserbyte = 70 // serial code for -1 year
							case 23:
								clickserbyte = 72 // serial code for -1 month
							case 26:
								clickserbyte = 74 // serial code for -1 day
							case 29:
								clickserbyte = 76 // serial code for -1 hour
							case 32:
								clickserbyte = 78 // serial code for -1 minute
							}
						}
					case 17, 18, 19, 20: // exit and save
						gridtemp = newgridtemp
						writetempfile(gridtemp)
						printscreen(printchan, usepv, usegrid, heatnow, gridtemp[wd][hh], msgstr) // redraw entire screen
						newscreendata(printchan, yyyy, mo, dd, hh)                                // add screen data
						screen = NormalScreen
					case 25, 26, 27, 28: // exit without saving
						printscreen(printchan, usepv, usegrid, heatnow, gridtemp[wd][hh], msgstr) // redraw entire screen
						newscreendata(printchan, yyyy, mo, dd, hh)                                // add screen data
						screen = NormalScreen
					case 37, 38, 39, 40: // quit
						fmt.Println("EXITING FROM APPLICATION...")
						return // return from main: quit from app
					}
				default: // clicked outside useful rows
					if (xsel > 0) && (ysel > 0) { // remove highlighting from old cursor position
						printchan <- append([]byte(fmt.Sprintf("<1B 14F %dX %dY>", xsel, ysel)), 0)
					}
					xsel, ysel = 0, 0 // reset selection to no selection
				}
			default: // no clicks for a long time: go back to normal screen
				if screentime > 150 {
					printscreen(printchan, usepv, usegrid, heatnow, gridtemp[wd][hh], msgstr) // redraw entire screen
					newscreendata(printchan, yyyy, mo, dd, hh)                                // add screen data
					screen = NormalScreen
					screentime = 0
				} else {
					screentime++
				}
			}
		case NormalScreen:
			for i := 1; i <= 10; i++ {
				select {
				case cp := <-clickchan: // receive click position coordinates y,x
					if cp[0] == 25 { // clicked on last line (status line)
						switch cp[1] {
						case 1: // clicked menu icon
							// draw settings screen then set screen variable:
							newgridtemp = gridtemp
							printsetscreen(printchan, gridtemp)
							xsel, ysel = 0, 0
							screen = SettingsScreen
						case 3: // clicked on water icon preceding Aquelio
							getmoreinfo = false
							printscreen(printchan, usepv, usegrid, heatnow, gridtemp[wd][hh], msgstr) // redraw entire screen
							newscreendata(printchan, yyyy, mo, dd, hh)                                // add screen data
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
							printchan <- []byte("<25y 39x 1b 14f>*Sun") // [*Sun] backlighting off
						case 43: // *Sun start icon
							usepv = true
							printchan <- []byte("<25y 39x 14b 10f>*Sun") // [*Sun] backlighting on
						case 45: // ~Grid stop icon
							usegrid = false
							printchan <- []byte("<25y 46x 1b 7f>~Grid") // [~Grid] backlighting off
						case 51: // ~Grid start icon
							usegrid = true
							printchan <- []byte("<25y 46x 7b 10f>~Grid") // [~Grid] backlighting on
						case 74: // +Temp stop icon
							heatnow = false
							printchan <- []byte("<25y 75x 1b 12f>+Temp") // [+Temp] backlighting off
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
		}
		//////////// READ DATA FROM NUCLEO: \\\\\\\\\\\\\
		bytesread := 0
		ch := byte('$') // ch will be the last char read, hopefully '@'
		for (bytesread < 25) || ((ch != '@') && (bytesread < 200)) {
			n, _ := ser.Read(buf[bytesread:])
			if n > 0 {
				bytesread += n        // update read char count
				ch = buf[bytesread-1] // last char read
			} else {
				fmt.Println("Serial read timeout!")
				break
			}
		}
		oldt = t                  // oldt and t are Raspberry time, only used to calculate dt
		t = time.Now()            // looks up time after every serial read operation
		dt := t.Sub(oldt).Hours() // time interval (h) since last calculation
		if getmoreinfo {          // displays buffer contents on screen
			printchan <- []byte("<21y 1x>")
			printchan <- buf[:bytesread]
		}
		if (bytesread < 25) || (ch != '@') {
			continue // couldn't read from Nucleo, skip to next loop iteration
		} //////////// else, go on
		////////// INTERPRET DATA READ FROM NUCLEO: \\\\\\\\\\\\
		var deciV int
		err = nil
		rec := buf[bytesread-16 : bytesread]    // get last 16 char
		rtc := buf[bytesread-25 : bytesread-16] // date & time info
		for i := 0; i < 9; i++ {                // replace spaces with 0's
			if rtc[i] == byte(' ') {
				rtc[i] = byte('0')
			}
		}
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
		_, err = fmt.Sscan(string(rec[11:13]), &temp) // water temperature in °C
		if err != nil {
			temp = 99
		}
		msgstr = rec[13:]
		if string(msgstr) != "OK@" {
			// "OK@" is the usual value for msgstr
			fmt.Println("Nucleo not OK")
		}
		_, err = fmt.Sscanf(string(rtc), "%2d%1x%2d%2d%2d", &yy, &mo, &dd, &hh, &mi)
		if err == nil {
			yyyy = (startyear/100)*100 + yy
			if yyyy < startyear {
				yyyy += 100
			}
		}
		newtimest := 1000000*yyyy + 10000*mo + 100*dd + hh // updated value of timest
		datestr := fmt.Sprintf("%d/%d/%d", dd, mo, yyyy)
		d, err := time.Parse("2/1/2006", datestr)
		if err != nil {
			fmt.Println("Error parsing date:", err)
			fmt.Println("rtc= ", string(rtc))
			fmt.Println("rec= ", string(rec))
		}
		wd = int(d.Weekday()) // weekday corresponding to RTC time read from Nucleo
		if pvheating {
			sunwatt = vpv * vpv / Rpv
			deltasunwh = sunwatt * dt
			hswh += deltasunwh
			dswh += deltasunwh
			mswh += deltasunwh
			yswh += deltasunwh
			totswh += deltasunwh
		} else {
			sunwatt = 0
		}
		if gridheating {
			gridwatt = vgrid * vgrid / Rgrid
			deltagridwh = gridwatt * dt
			hgwh += deltagridwh
			dgwh += deltagridwh
			mgwh += deltagridwh
			ygwh += deltagridwh
			totgwh += deltagridwh
		} else {
			gridwatt = 0
		}
		////////// COMPUTE SERIAL BYTE AND SEND IT TO NUCLEO: \\\\\\\\\\\\
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
			} else if stgridtemp != gridtemp[wd][hh] {
				serbyte = gridtemp[wd][hh]
			} else {
				serbyte = 200 // everything OK, nothing to communicate
			}
		}
		if (serbyte == 200) && getmoreinfo {
			serbyte = 4 // tell Nucleo to send complete data
		}
		if clickserbyte != 0 { // give priority to clickserbyte, if present
			ser.Write([]byte{clickserbyte})
		} else {
			ser.Write([]byte{serbyte}) // send to Nucleo
		}
		////////// CHECK IF HOUR HAS CHANGED, LOG AND PRINT TO SCREEN: \\\\\\\\\\\\
		if newtimest > timest { // hour has changed (and this is not the first iteration)
			// first append line to datafile with last hour's data:
			ln := fmt.Sprintf("%d %4d %4d %2d\n", timest+1, int(hswh), int(hgwh), temp) //+1 so hour 0-23 becomes 1-24
			appendStringToFile("priv/datafile.txt", ln)
			if screen == NormalScreen { // update screen data unless settings screen is displayed
				newscreendata(printchan, oldyyyy, oldmo, olddd, oldhh+1)
			}
			hswh = 0
			hgwh = 0
			if dd != olddd {
				dswh = 0
				dgwh = 0
				if mo != oldmo {
					mswh = 0
					mgwh = 0
					if yyyy != oldyyyy {
						yswh = 0
						ygwh = 0
					}
				}
			}
		}
		timest = newtimest // update all time variables
		oldhh = hh
		olddd = dd
		oldmo = mo
		oldyyyy = yyyy
		if screen == NormalScreen { // print data to screen:
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
		}
	} // end of infinite loop
}
