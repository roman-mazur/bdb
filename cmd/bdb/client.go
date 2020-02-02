package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/google/gousb"
	"github.com/google/gousb/usbid"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var (
	debug = flag.Int("debug", 0, "libusb debug level (0..3)")
)

func main() {
	flag.Parse()

	command := flag.Arg(0)

	ctx := gousb.NewContext()
	defer ctx.Close()

	ctx.Debug(*debug)

	devices := openDevices(ctx)
	defer func() {
		for _, device := range devices {
			device.Close()
		}
	}()

	switch command {
	case "devices":
		if len(devices) > 0 {
			fmt.Println("Connected balena devices:")
			fmt.Println()
			for _, d := range devices {
				in, out, done := unwrap(d)
				defer done()
				_, err := out.Write([]byte{1})
				if err != nil {
					panic(err)
				}
				buf := make([]byte, 250)
				_, err = in.Read(buf)
				if err != nil {
					panic(err)
				}
				fmt.Println(string(buf[1:buf[0]]))
			}
			fmt.Println()
		}

	case "shell":
		if len(devices) > 0 {
			fmt.Println("Staring shell over USB...")
			in, out, done := unwrap(devices[0])
			defer done()

			go func() {
				_, err := out.Write([]byte{2})
				if err != nil {
					panic(err)
				}
				go func() {
					scan := bufio.NewScanner(os.Stdin)
					for scan.Scan() {
						if _, err := out.Write([]byte(scan.Text() + "\n")); err != nil {
							panic(err)
						}
					}
				}()
				_, err = io.Copy(os.Stdout, in)
				if err != nil {
					log.Fatal(err)
				}
			}()
			<-time.After(60 * time.Second)
		}

	default:
		log.Fatalf("Command is not defined")
	}

}

func unwrap(d *gousb.Device) (*gousb.InEndpoint, *gousb.OutEndpoint, func()) {
	intf, done, err := d.DefaultInterface()
	if err != nil {
		panic(err)
	}
	in, err := intf.InEndpoint(1)
	if err != nil {
		panic(err)
	}
	out, err := intf.OutEndpoint(1)
	if err != nil {
		panic(err)
	}
	return in, out, done
}

func openDevices(ctx *gousb.Context) []*gousb.Device {
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return strings.Contains(usbid.Describe(desc), "FunctionFS")
	})
	if err != nil {
		panic(err)
	}
	return devices
}
