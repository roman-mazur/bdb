package main

import (
	"flag"
	"fmt"
	"github.com/creack/pty"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"rmazur.io/bdb/usb"
	"syscall"
)

type workStarter = func(stopSignal <-chan struct{})

func controlLoop(control *os.File, starter workStarter) {
	var state struct {
		bound   bool
		enabled bool
		working bool
	}

	workerStop := make(chan struct{})

readLoop:
	for {
		event, err := usb.ReadFfsEvent(control)
		if err != nil {
			log.Fatalf("Error reading from ep0: %s", err)
		}
		log.Printf("USB event: %s", event.Type)

		switch event.Type {
		case usb.FunctionfsBind:
			if state.bound {
				log.Println("Got bind while already bound")
				break readLoop
			}
			if state.enabled {
				log.Println("Got bind while already enabled")
				break readLoop
			}
			state.bound = true

		case usb.FunctionfsEnable:
			if !state.bound {
				log.Println("Got enable while not bound")
				break readLoop
			}
			if state.enabled {
				log.Println("Got enable while already enabled")
				break readLoop
			}
			state.enabled = true
			go starter(workerStop)

		case usb.FunctionfsDisable:
			if !state.bound {
				log.Println("Got disable while not bound")
			}
			if !state.enabled {
				log.Println("Got disable while not enabled")
			}
			state.enabled = false
			break readLoop

		case usb.FunctionfsUnbind:
			if !state.bound {
				log.Println("Got unbind while not bound")
			}
			if !state.enabled {
				log.Println("Got unbind while not enabled")
			}
			state.bound = false
			break readLoop

		case usb.FunctionfsSetup:
			log.Printf("Got setup control request: %v", event)
			if (event.SetupPayload.RequestType & usb.DirectionIn) != 0 {
				log.Println("Acking device-to-host control transfer")
				if n, err := control.WriteString(""); err != nil || n != 0 {
					log.Println("Failed to write empty packet to host")
				}
			} else {
				data, err := ioutil.ReadAll(io.LimitReader(control, int64(event.SetupPayload.Length)))
				if err != nil {
					log.Printf("Failed to read data from host: %s", err)
				} else {
					if len(data) != int(event.SetupPayload.Length) {
						log.Printf("Expected %d, but got %d from host", event.SetupPayload.Length, len(data))
					}
					log.Printf("control request data: [%s]", string(data))
				}
			}
		}
	}
	log.Println("Control loop finished")
	if state.working {
		log.Println("Stopping worker")
		workerStop <- struct{}{}
	}
}

func waitForInput(input io.Reader, command chan<- byte) {
	var buf [1]byte
	n, err := input.Read(buf[:])
	if err != nil {
		log.Printf("Problem reading from input: %s", err)
		command <- 0
	} else if n != 0 {
		command <- buf[0]
	}
}

func worker(input io.Reader, output io.Writer, stopSignal <-chan struct{}) {
	commandChannel := make(chan byte)
	running := true
	for running {
		go waitForInput(input, commandChannel)

		select {
		case command := <-commandChannel:
			log.Printf("Command %d", command)
			err := handleCommand(command, input, output)
			if err != nil {
				log.Printf("Error handling the command: %s", err)
				running = false
			}
		case <-stopSignal:
			running = false
		}
	}
	log.Println("Worker stopped")
}

func handleCommand(command byte, input io.Reader, output io.Writer) error {
	switch command {
	case 0:
		return fmt.Errorf("host communication problem")
	case 1:
		description := fmt.Sprintf("%s\t%s %s", os.Getenv("BALENA_DEVICE_UUID")[0:7], os.Getenv("BALENA_DEVICE_TYPE"),
			os.Getenv("BALENA_HOST_OS_VERSION"))
		data := make([]byte, len(description)+1)
		data[0] = byte(len(data) - 1)
		copy(data[1:], description)
		output.Write(data)
		return nil
	case 2:
		log.Println("Starting shell...")
		cmd := exec.Command("/bin/bash")
		terminal, err := pty.Start(cmd)
		if err != nil {
			return err
		}

		go func() {
			_, _ = io.Copy(output, terminal)
		}()

		finish := make(chan struct{})
		go func() {
			log.Printf("Start piping to the shell process %d", cmd.Process.Pid)
			buf := make([]byte, 512)
			running := true
			go func() {
				<-finish
				running = false
			}()

			// TODO: Fix this blocking.
			for running {
				n, err := input.Read(buf)
				if !running {
					break
				}
				if err == nil && n > 0 {
					_, err = terminal.Write(buf[0:n])
				}
				if err != nil {
					log.Printf("Error sending data to shell: %s", err)
					running = false
				}
			}
			log.Printf("Finished piping to the shell process %d", cmd.Process.Pid)
		}()

		_ = cmd.Wait()
		finish <- struct{}{}
		return nil
	default:
		return fmt.Errorf("unknown command")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("USB FFS directory is not defined in the arguments")
	}
	ffsDir := args[0]

	ep0, err := os.Create(path.Join(ffsDir, "ep0"))
	if err != nil {
		log.Fatalf("Cannot open ep0: %s", err)
	}
	defer ep0.Close()

	if _, err := ep0.Write(usb.BuildUsbDescriptors()); err != nil {
		log.Fatalf("Cannot write descriptoors: %s", err)
	}
	if _, err := ep0.Write(usb.BuildUsbStrings()); err != nil {
		log.Fatalf("Cannot write strings: %s", err)
	}

	bdbOut, err := os.Create(path.Join(ffsDir, "ep1"))
	if err != nil {
		log.Fatalf("Cannot open ep1: %s", err)
	}
	defer bdbOut.Close()
	bdbIn, err := os.Open(path.Join(ffsDir, "ep2"))
	if err != nil {
		log.Fatalf("Cannot open ep2: %s", err)
	}
	defer bdbIn.Close()

	go controlLoop(ep0, func(stopSignal <-chan struct{}) {
		worker(bdbIn, bdbOut, stopSignal)
	})

	waitForSignal()
}

func waitForSignal() {
	stopSig := make(chan os.Signal)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)
	<-stopSig
}
