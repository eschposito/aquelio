#include "mbed.h"
Serial pc(USBTX, USBRX); // serial connection to PC over USB
RawSerial ser1(PC_4, D2, 115200); // tx, rx, baud
DigitalIn but(USER_BUTTON);
DigitalOut myled(LED2); // the only user LED
AnalogIn ainvpv(A0);   // voltage from PV panels
AnalogIn ainvgrid(A1); // voltage from ac grid
AnalogIn ainvntc(A4);  // normalized voltage across NTC temperature sensor
DigitalOut k1on(D11), k2on(D14), k3on(D6); //drivers for relay K1 (grid), K2 (PV), K3 (PV, in series with IGBT)
DigitalOut kx_hi(D12); // 12V supply for turn on of relays K1,K2,K3 (for keeping on it's just 5V)
DigitalOut igbt_on(D8);// IGBT in series with relay K3
DigitalOut aux1on(D3), aux2on(D4), aux3on(D5); // auxiliary outputs
AnalogIn ainvpe(A5);        // voltage from ground PE wire
AnalogIn ainvLswitch(PC_3);// voltage from L thermostat switch (note pin A3 has no analog in capabilities)
AnalogIn ainvNswitch(A2); // voltage from N thermostat switch
const float Bntc = 3971; // NTC B constant for temp range 25...80°C
const float Rpv= 9.6, Rgrid= 35.5; // PV and grid water heater element resistance in Ohm
int debugtime= 150; // if > 0, prints out data through USB serial
int gridtemp= 5;   // temp setting for grid heating, settable within 5..67°C
bool usepv= true, usegrid= true; // enable/disable use of PV and grid
bool vgriderr, visolerr; // voltage error flags: grid V, isolation V
bool ntctemperr, thswitcherr; // temp error flags: NTC temp, thermostat safety switch
float vpv, vpvmin, vpvmax, vgrid, vgrid2, vLswitch, vNswitch, vpe;
float pvcur, pvpow; // PV heater current (A) and power (W)
float gridcur, gridpow; // grid heater current (A) and power (W)
float wtemp, oldwtemp=99, veryoldwtemp=99; // water temperature in °C
char to_send[26]= "123456789ABCDEF0123456789", received; // chars sent to and received from Raspberry Pi
char stmsg[3]= "OK"; // status message to be sent to Raspberry
void turnPVon() // PV turn on sequence
{
    kx_hi= 1;
    wait(0.5);
    k3on= 1;
    wait(1);
    igbt_on= 1;
    wait(1);
    k2on= 1;
    wait(1);
    igbt_on= 0;
    kx_hi= 0;
    wait(1);
    k3on= 0;
    wait(0.5);
    myled= 1;
}

void turnPVoff() // PV turn off sequence
{
    kx_hi= 1;
    wait(0.5);
    k3on= 1;
    wait(1);
    igbt_on= 1;
    wait(1);
    k2on= 0;
    wait(1);
    igbt_on= 0;
    kx_hi= 0;
    wait(1);
    k3on= 0;
    wait(0.5);
    myled= 0; 
}

int main()
{
    // initialization section:
    igbt_on= 0;
    kx_hi= 0;
    k1on= 0;
    k2on= 0;
    k3on= 0;
    aux1on= 0;
    aux2on= 0;
    aux3on= 0;
    unsigned int lastk1on= 0, lastk2on= 0;
    unsigned int lowvpvcount= 0, vgridnogoodcount= 0;
    unsigned int butpresscount= 0;
    // infinite loop with cycle counter (count):
    for(unsigned int count=0; true; count++) {
        {   // this block calculates vpv, vpvmin, vpvmax, lowvpvcount
            vpvmin= 1000;
            vpvmax= 0;
            float sumvpv= 0;
            for (int i=1; i<=16; i++) {
                float v= 3.3f*ainvpv/3.01f*203.01f; // resistor bridge voltage calculation
                if (v<vpvmin) vpvmin= v;
                if (v>vpvmax) vpvmax= v;
                sumvpv+= v;
                wait(0.110625);
            }
            vpv= sumvpv/16; // average measured vpv value
            if (vpv < 2) { // time to go to sleep, it's dark!
                lowvpvcount+= 1;
            } else lowvpvcount= 0; // reset
        }
        {   // this block calculates vgrid, vgrid2, vgridnogoodcount, vgriderr
            float sumvsquared= 0, vpeak= 0;
            for (int i=1; i<=40; i++) { // finds ac grid rms voltage
                float v= ainvgrid;
                if (v>vpeak) vpeak= v;
                sumvsquared+= v*v;
                wait(0.0105);
            }
            vgrid= sqrt(sumvsquared/40)*3.3f/3.01f*203.01f/88*230;
            vgrid2= (vpeak*3.3f/3.01f*203.01f+1)/88*230/sqrt(2.0f); // another way to compute vgrid
            // we computed RMS grid voltage vgrid considering resistor bridge and 230V->88V transformer
            if ((vgrid2 < 175) || (vgrid2 > 265)) {
                vgridnogoodcount+= 1;
            } else vgridnogoodcount= 0; // reset
            vgriderr = (vgridnogoodcount >= 3);
        }
        {   // this block calculates wtemp, oldwtemp, veryoldwtemp, ntctemperr
            veryoldwtemp= oldwtemp;
            oldwtemp= wtemp;
            float v= ainvntc; // read NTC voltage analog input
            if (v<0.1f) { pc.printf("NTC short circuit\r\n"); wtemp= 99;}
            else if (v>0.9f) { pc.printf("NTC open circuit\r\n"); wtemp= 99;}
            else wtemp= -273.15 + 1/ (1/298.15 + 1/Bntc*log(v/(1-v)));
            if ((wtemp<0) || (wtemp>=99)) wtemp= 99; // there's something wrong...
            ntctemperr = (wtemp == 99);
        }
        {   // this block calculates vpe,vLswitch,vNswitch,visolerr,thswitcherr
            vpe= 3.3f*ainvpe/3.01f*403.01f;
            vLswitch= 3.3f*ainvLswitch/3.01f*13.01f;
            vNswitch= 3.3f*ainvNswitch/3.01f*13.01f;
            visolerr = (vpe > 12); // err if measured > 12 V from 0v to earth
            thswitcherr = (vNswitch < 1); // thermostat safety switch needs manual reset
        }
        // next, prepare to_send[] char array, for sending to Raspberry
        char pvstate, gridstate;
        if (k2on) {
            lastk2on= count;
            pvcur= vpv/Rpv;
            pvpow= vpv*pvcur;
            pvstate= 'S'; // in this moment, PV is heating
        } else {
            pvcur= 0;
            pvpow= 0;
            pvstate= 's'; // in this moment, PV is not heating
        }
        if (k1on) {
            lastk1on= count;
            gridcur= vgrid/Rgrid;
            gridpow= vgrid*gridcur;
            gridstate= 'G'; // in this moment, grid is heating
        } else {
            gridcur= 0;
            gridpow= 0;
            gridstate= 'g'; // in this moment, grid is not heating
        }
        // stmsg will signal either an error or OK:
        if (visolerr) {
            snprintf(stmsg, 3, "Ei");
        } else if (ntctemperr) {
            snprintf(stmsg, 3, "Et");
        } else if (thswitcherr) {
            snprintf(stmsg, 3, "Es");
        } else if (vgriderr) {
            snprintf(stmsg, 3, "Eg");
        } else snprintf(stmsg, 3, "OK");
        // what time is it? Read RTC:
        time_t unixsecs = time(NULL); // seconds since January 1st 1970
        struct tm* ct; // pointer to date & time tm struct
        // now read serial byte from Raspberry and interpret it:
        if (ser1.readable()) {
            received= ser1.getc();
            switch (received) {
            case 0:
                usepv= 0;
                usegrid= 0;
                break;
            case 1:
                usepv= 0;
                usegrid= 1;
                break;
            case 2:
                usepv= 1;
                usegrid= 0;
                break;
            case 3:
                usepv= 1;
                usegrid= 1;
                break;
            case 4: // send complete data to Raspberry, see below
                break;
            case 68: // set RTC time back approximately 10 years
                unixsecs-= 10 * 365 * 24 * 3600;
                set_time(unixsecs);
                break;
            case 69: // set RTC time forward approximately 10 years
                unixsecs+= 10 * 365 * 24 * 3600;
                set_time(unixsecs);
                break;
            case 70: // set RTC time back approximately 1 year
                unixsecs-= 365 * 24 * 3600;
                set_time(unixsecs);
                break;
            case 71: // set RTC time forward approximately 1 year
                unixsecs+= 365 * 24 * 3600;
                set_time(unixsecs);
                break;
            case 72: // set RTC time back approximately 1 month
                unixsecs-= 30 * 24 * 3600;
                set_time(unixsecs);
                break;
            case 73: // set RTC time forward approximately 1 month
                unixsecs+= 30 * 24 * 3600;
                set_time(unixsecs);
                break;
            case 74: // set RTC time back 1 day
                unixsecs-= 24 * 3600;
                set_time(unixsecs);
                break;
            case 75: // set RTC time forward 1 day
                unixsecs+= 24 * 3600;
                set_time(unixsecs);
                break;
            case 76: // set RTC time back 1 hour
                unixsecs-= 3600;
                set_time(unixsecs);
                break;
            case 77: // set RTC time forward 1 hour
                unixsecs+= 3600;
                set_time(unixsecs);
                break;
            case 78: // set RTC time back 1 minute
                unixsecs-= 60;
                set_time(unixsecs);
                break;
            case 79: // set RTC time forward 1 minute
                unixsecs+= 60;
                set_time(unixsecs);
                break;
            case 80: // set RTC time seconds back to 00
                ct= localtime(&unixsecs); // put actual date & time into tm struct pointed by ct
                unixsecs-= ct->tm_sec;
                set_time(unixsecs);
                break;
            case 81: // set RTC time seconds forward to 00
                ct= localtime(&unixsecs); // put actual date & time into tm struct pointed by ct
                unixsecs+= 60 - ct->tm_sec;
                set_time(unixsecs);
                break;
            default:
                if ((received>=5) && (received<=67)) gridtemp= received;
            }
        } else received= 255; // means nothing was received
        // compute parambyte:
        char parambyte= gridtemp-4;   // set least 6 bits (1..63 range, 0 is excluded)
        if (usegrid) parambyte+= 64; // add bit 7
        if (usepv) parambyte+= 128; // add bit 8
        // compute date and time:
        ct= localtime(&unixsecs); // write RTC time & date into ct*
        // compose string to_send:
        snprintf(to_send, 26, "%02d%x%02d%02d%02d%c%04.0f%c%04.0f%c%02.0f%s@", // '@' last char
                ct->tm_year%100, ct->tm_mon+1, ct->tm_mday, ct->tm_hour, ct->tm_min,
                pvstate, 10*vpv, gridstate, 10*vgrid2, parambyte, wtemp, stmsg);
        // and send serial data to Raspberry:
        if (received == 4) { // Raspberry requested complete data
            char line[100];
            snprintf(line, 100,
                "%x:T=%.1f %.1f<Vpv=%.1f<%.1f Vgr=%.1f_%.1f Ppv=%.0f Pgr=%.0f ",
                count, wtemp, vpvmin, vpv, vpvmax, vgrid, vgrid2, pvpow, gridpow);
            ser1.puts(line);
            snprintf(line, 100,
                "K^123i=%d%d%d%d%d A123=%d%d%d vLsw=%.1f vNsw=%.1f vPE=%.1f P%d R%d $%s",
                kx_hi.read(), k1on.read(), k2on.read(), k3on.read(), igbt_on.read(),
                aux1on.read(), aux2on.read(), aux3on.read(), vLswitch, vNswitch, vpe,
                to_send[10], received, to_send);
            ser1.puts(line);
        } else ser1.puts(to_send);
        // next, handle local debug mode, if required:
        if (but == 0) { // user button pressed
            butpresscount+= 1;
        } else butpresscount= 0;
        if (debugtime > 0) { // prints out data to USB
            debugtime--;     // and decrements debug cycles left
            pc.printf("%x:T=%.1f %.1f<Vpv=%.1f<%.1f Vgr=%.1f_%.1f Ppv=%.0f Pgr=%.0f\r\n",
                count, wtemp, vpvmin, vpv, vpvmax, vgrid, vgrid2, pvpow, gridpow);
            pc.printf("K^123i=%d%d%d%d%d A123=%d%d%d vLsw=%.1f vNsw=%.1f vPE=%.1f P%d R%d $%s\r\n",
                kx_hi.read(), k1on.read(), k2on.read(), k3on.read(), igbt_on.read(),
                aux1on.read(), aux2on.read(), aux3on.read(), vLswitch, vNswitch, vpe,
                to_send[10], received, to_send);
        } else if (butpresscount >= 3) debugtime= 150; // button pressed for at least 3 cycles
        // finally set the state of the actuators (relays):
        if (usepv) {
            if (k2on) {
                if (((wtemp >= 77) && (oldwtemp >= 77) && (veryoldwtemp >= 77))
                    || (lowvpvcount > 5) || visolerr || thswitcherr)
                    turnPVoff();
                    // vgriderr no problem for PV, ntctemperr included in wtemp check
            } else if ((wtemp<74) && (oldwtemp<74) && (veryoldwtemp<74)
                && (count-lastk2on > 100) && (vpv>115) && (vpe<5) && (vNswitch>4))
                turnPVon();
        } else if (k2on) turnPVoff();
        if (usegrid) {
            if (k1on) {
                if (((wtemp >= gridtemp) && (oldwtemp >= gridtemp) && (veryoldwtemp >= gridtemp))
                    || vgriderr || thswitcherr || (vLswitch<1)) // visolerr only for PV
                    k1on= 0; // turn off grid heating relay
            } else {
                int Tmin= gridtemp - 3;
                if ((wtemp<Tmin) && (oldwtemp<Tmin) && (veryoldwtemp<Tmin)
                    && (count-lastk1on > 100)
                    && !vgriderr && !thswitcherr && (vLswitch>4))
                    k1on= 1; // turn on grid heating relay
            }
        } else if (k1on) k1on= 0; // turn off grid heating relay
    } // end of infinite loop
}
