/*
MPC4725 based "function generator" for testing purposes
Also acts as example how MCP4725 is controlled.

I will add more features later while my main project develops

*/

package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"syscall"
	"time"
)

type MCP4725 struct {
	I2CHandle *os.File //Set this at startup
	Address   byte
}

func SelectI2CSlave(f *os.File, address byte) error {
	//i2c_SLAVE := 0x0703
	_, _, errorcode := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(), 0x0703, uintptr(address), 0, 0, 0)
	if errorcode != 0 {
		return fmt.Errorf("Select I2C slave errcode %v", errorcode)
	}
	return nil
}

func (p *MCP4725) SetDac(value12bit uint16) error {
	errSel := SelectI2CSlave(p.I2CHandle, p.Address)
	if errSel != nil {
		return errSel
	}
	n, err := p.I2CHandle.Write([]byte{byte((value12bit >> 8) & 0xF), byte(value12bit & 0xFF)})
	if err != nil {
		return err
	}
	if n != 2 {
		return fmt.Errorf("Invalid I2C write size %v", n)
	}
	return nil
}

func (p *MCP4725) SetVoltage(v float64) error {
	if v < 0 {
		return p.SetDac(0)
	}
	if 3.3 < v {
		return p.SetDac(0xFFF)
	}
	return p.SetDac(uint16(math.Floor((v / 3.3) * 4096)))
}

func (p *MCP4725) RunSin(lo float64, hi float64, frequency float64, nMax int64, verbose bool) error {
	//period := 1 / frequency
	t0 := float64(time.Now().UnixNano()/(1000*1000)) / float64(1000.0)
	for {
		t := float64(time.Now().UnixNano()/(1000*1000)) / float64(1000.0) //In sec
		cycles := int64((t - t0) * frequency)
		if (0 < nMax) && (nMax <= cycles) {
			return nil
		}
		voltage := (math.Sin(2*math.Pi*frequency*t)+1)*(hi-lo)/2.0 + lo
		if verbose {
			fmt.Printf("v=%v\n", voltage)
		}
		if err := p.SetVoltage(voltage); err != nil {
			return err
		}
	}
}

func (p *MCP4725) RunSaw(lo float64, hi float64, frequency float64, nMax int64, verbose bool) error {
	//period := 1 / frequency
	t0 := float64(time.Now().UnixNano()/(1000*1000)) / float64(1000.0) //In sec
	for {
		t := float64(time.Now().UnixNano()/(1000*1000)) / float64(1000.0) //In sec
		cycles := int64((t - t0) * frequency)
		if (0 < nMax) && (nMax <= cycles) {
			return nil
		}
		voltage := math.Mod(t, 1/frequency)*(hi-lo)*frequency + lo
		if verbose {
			fmt.Printf("v=%v\n", voltage)
		}
		if err := p.SetVoltage(voltage); err != nil {
			return err
		}
	}
}

func (p *MCP4725) RunSteps(points []float64, fs float64, nMax int64, verbose bool) error {
	period := 1 / fs
	counter := int64(0)
	for {
		for _, v := range points {
			t := time.Now()
			if verbose {
				fmt.Printf("v=%v\n", v)
			}
			if err := p.SetVoltage(v); err != nil {
				return err
			}
			if len(points) == 1 {
				return nil //Only one point,  leave
			}
			sleeptime := time.Duration(period)*time.Second - time.Since(t)
			if 0 < sleeptime {
				time.Sleep(sleeptime)
			}
		}
		counter++
		if (0 < nMax) && (nMax <= counter) {
			return nil
		}
	}
}

func main() {
	functionPtr := flag.String("fun", "sin", "generator function [ste,sin,saw]")
	loPtr := flag.Float64("lo", 0, "Lo voltage (dac V)")
	hiPtr := flag.Float64("hi", 2.5, "Hi voltage (dac V)")
	freqPtr := flag.Float64("freq", 1, "How frequently sequence repeats (Hz)")
	addrPtr := flag.Bool("a", false, "Add a parameter for setting address bit.  a missing=0x60 with a=0x61")
	devicePtr := flag.String("dev", "/dev/i2c-1", "I2C device file") //On raspberry PI, this is default
	nPtr := flag.Int64("n", 0, "How many periods are produced")
	verbosePtr := flag.Bool("v", false, "Print voltages on terminal")

	flag.Parse()

	f, errI2CHardware := os.OpenFile(*devicePtr, os.O_RDWR, 0600)
	defer f.Close()

	if errI2CHardware != nil {
		fmt.Printf("I2C error on device file %v  error=%v\n", *devicePtr, errI2CHardware.Error())
		return
	}

	i2cAdrr := byte(0x60)
	if *addrPtr {
		i2cAdrr = 0x61
	}
	dac := MCP4725{I2CHandle: f, Address: i2cAdrr}

	switch *functionPtr {
	case "ste": //Just set to constant value
		arr := flag.Args()
		if len(arr) == 0 {
			fmt.Printf("Please provide voltage or voltages")
			return
		}
		//Step tru values
		points := make([]float64, len(arr))
		for i, vs := range arr {
			v, err := strconv.ParseFloat(vs, 64)
			if err != nil {
				fmt.Printf("Invalid value %v\n", vs)
				break
			}
			points[i] = v
		}
		if dacErr := dac.RunSteps(points, *freqPtr, *nPtr, *verbosePtr); dacErr != nil {
			fmt.Printf("DAC error %v\n", dacErr)
		}
	case "sin":
		if dacErr := dac.RunSin(*loPtr, *hiPtr, *freqPtr, *nPtr, *verbosePtr); dacErr != nil {
			fmt.Printf("DAC error %v\n", dacErr)
		}
	case "saw":
		if dacErr := dac.RunSaw(*loPtr, *hiPtr, *freqPtr, *nPtr, *verbosePtr); dacErr != nil {
			fmt.Printf("DAC error %v\n", dacErr)
		}
	default:
		fmt.Printf("ERROR: function %v not supported", *functionPtr)
	}
}
