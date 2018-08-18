#include "mbed.h"
// still incomplete, RTC code missing...
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
AnalogIn ainvLswitch(PC_3);// voltage from L thermostat switch (note pin A3 han no analog in capabilities)
AnalogIn ainvNswitch(A2); // voltage from N thermostat switch
const float Bntc = 3971; // NTC B constant for temp range 25...80°C
const float Rpv= 9.6;    // PV water heater element resistance in Ohm
const float Rgrid= 35.5; // grid water heater element resistance in Ohm
int debugtime= 0; // if > 0, prints out data through USB serial
int gridtemp= 50; // temperature setting for grid heating
bool usepv= true; // enable/disable use of PV
bool usegrid= false; // enable/disable use of grid
float vpv, vpvmin, vpvmax, vgrid, vLswitch, vNswitch, vpe;
float pvcur, pvpow; // PV heater current (A) and power (W)
float gridcur, gridpow; // grid heater current (A) and power (W)
float wtemp, oldwtemp=99, veryoldwtemp=99; // water temperature in °C
char to_send[16], received; // chars sent to and received from Raspberry

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
    unsigned int lowvpvcount= 0, hivgridcount= 0, lowvgridcount= 0;
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
            vpv= sumvpv/16;  // average measured vpv value
            if (vpv > 24) {
                lowvpvcount= 0; // reset
            } else lowvpvcount+= 1;
        }
        {   // this block calculates vgrid, lowvgridcount, hivgridcount
            float sumvsquared= 0;
            for (int i=1; i<=100; i++) { // finds ac grid rms voltage over 5 complete waveforms
                float v= 3.3f*ainvgrid/3.01f*203.01f; // resistor bridge calculation
                sumvsquared+= v*v;
                wait(0.001);
            }
            vgrid= sqrt(sumvsquared/100)/88*230; // RMS value, considering 230V->88V transformer
            if (vgrid < 175) {
                lowvgridcount+= 1;
            } else if (vgrid > 265) {
                hivgridcount+= 1;
            } else { // reset
                lowvgridcount= 0;
                hivgridcount= 0;
            }
        }
        {   // this block calculates wtemp, oldwtemp, veryoldwtemp
            veryoldwtemp= oldwtemp;
            oldwtemp= wtemp;
            float v= ainvntc; // read NTC voltage analog input
            if (v<0.1f) { pc.printf("NTC short circuit\n"); wtemp= 99;}
            else if (v>0.9f) { pc.printf("NTC open circuit\n"); wtemp= 99;}
            else wtemp= -273.15 + 1/ (1/298.15 + 1/Bntc*log(v/(1-v)));
            if ((wtemp<0) || (wtemp>99)) wtemp= 99; // there's something wrong...
        }
        {   // this block calculates vpe, vLswitch, vNswitch
            vpe= 3.3f*ainvpe/3.01f*403.01f;
            vLswitch= 3.3f*ainvLswitch/3.01f*13.01f;
            vNswitch= 3.3f*ainvNswitch/3.01f*13.01f;
        }
        // next, prepare to_send[] char array, for sending to Raspberry
        char *vpvstring, *vgridstring, *tempstring;
        if (k2on) {
            lastk2on= count;
            pvcur= vpv/Rpv;
            pvpow= vpv*pvcur;
            to_send[0]= 'S'; // in this moment, PV is heating
        } else {
            pvcur= 0;
            pvpow= 0;
            to_send[0]= 's'; // in this moment, PV is not heating
        }
        sprintf(vpvstring, "%4.0f", 10*vpv);
        for (int i=1; i<=4; i++) to_send[i]= vpvstring[i-1];
        if (k1on) {
            lastk1on= count;
            gridcur= vgrid/Rgrid;
            gridpow= vgrid*gridcur;
            to_send[5]= 'G'; // in this moment, grid is heating
        } else {
            gridcur= 0;
            gridpow= 0;
            to_send[5]= 'g'; // in this moment, grid is not heating
        }
        sprintf(vgridstring, "%4.0f", 10*vgrid);
        for (int i=6; i<=9; i++) to_send[i]= vgridstring[i-6];
        int parambyte= gridtemp-5; // set least 6 bits (0..63 range)
        if (usegrid) parambyte+= 64; // add bit 7
        if (usepv) parambyte+= 128; // add bit 8
        to_send[10]= parambyte;
        sprintf(tempstring, "%2.0f", wtemp);
        for (int i=11; i<=12; i++) to_send[i]= tempstring[i-11];
        char msgstring[4]= "OK!"; // for now, fixed ok message
        for (int i=13; i<=15; i++) to_send[i]= msgstring[i-13];
        // handle debug mode, if required:
        if (but == 0) { // user button pressed
            butpresscount+= 1;
        } else butpresscount= 0;
        if (debugtime > 0) { // prints out data to USB
            debugtime--;     // and decrements debug cycles left
            pc.printf("%d, usepv=%d T=%.1f Vpv=%.1f (min=%.1f max=%.1f) Vgrid=%.1f Ipv=%.2f Ppv=%.1f Pgr=%.1f\n",
                        count, usepv, wtemp, vpv, vpvmin, vpvmax, vgrid, pvcur, pvpow, gridpow);
            pc.printf("K^123i=%d%d%d%d%d A123=%d%d%d vLsw=%.1f vNsw=%.1f vPE=%.1f\n",
                        kx_hi.read(), k1on.read(), k2on.read(), k3on.read(), igbt_on.read(),
                        aux1on.read(), aux2on.read(), aux3on.read(), vLswitch, vNswitch, vpe);
        } else if (butpresscount >= 3) debugtime= 150; // button pressed for at least 3 cycles
        // now read from and write to serial 1:
        if (ser1.readable()) received= ser1.getc();
        ser1.puts(to_send);
        // and interpret received byte:
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
        default:
            if ((received>=5) && (received<=68)) gridtemp= received;
            break;
        }
        // finally set the state of the actuators (relays):
        if (usepv) {
            if (k2on) {
                if ((wtemp >= 77) || (lowvpvcount > 5) || (vNswitch<4))
                    turnPVoff();
            } else if ((wtemp<74) && (oldwtemp<74) && (veryoldwtemp<74)
                && (count-lastk2on > 100) && (vpv > 100) && (vNswitch>4))
                turnPVon();
        } else if (k2on) turnPVoff();
        if (usegrid) {
            if (k1on) {
                if ((wtemp >= gridtemp) || (lowvgridcount > 3)
                  || (hivgridcount > 3) || (vLswitch<4))
                    k1on= 0; // turn off grid heating relay
            } else {
                int Tmin= gridtemp - 3;
                if ((wtemp<Tmin) && (oldwtemp<Tmin) && (veryoldwtemp<Tmin)
                    && (count-lastk1on > 100) && (lowvgridcount== 0)
                    && (hivgridcount ==0) && (vLswitch>4))
                    k1on= 1; // turn on grid heating relay
            }
        } else if (k1on) k1on= 0; // turn off grid heating relay
    } // end of infinite loop
}
