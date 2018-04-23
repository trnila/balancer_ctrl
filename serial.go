package main

import (
	"github.com/dgryski/go-cobs"
	"log"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/jacobsa/go-serial/serial"
	"bufio"
	"time"
)

type Cmd struct {
	ID byte
}

func SimpleCmd(id byte) (Cmd)  {
	return Cmd{
		ID: id,
	}
}

type Event struct {
	name string
	data interface{}
}

func producer(measurements chan <- Measurement, events chan <- Event, commands chan interface{}) {
	options := serial.OpenOptions {
		PortName:        cfg.SerialPath,
		BaudRate:        cfg.BaudRate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	port, err := serial.Open(options)
	if err != nil {
		log.Fatalf("serial.Open: %v", err)
	}
	defer port.Close()

	reader := bufio.NewReader(port)

	timer := time.NewTicker(2 * time.Second)
	go func() {
		for {
			<- timer.C
			commands <- SimpleCmd(CMD_GETPOS)
		}
	}()

	go func() {
		for {
			cmd := <- commands

			var buffer bytes.Buffer
			err := binary.Write(&buffer, binary.LittleEndian, cmd)
			if err != nil {
				fmt.Println(err)
				continue
			}

			encoded := cobs.Encode(buffer.Bytes())
			encoded = append(encoded, '\x00')
			port.Write(encoded)
		}
	}()

	for {
		frame, err := reader.ReadBytes('\x00')
		if err != nil {
			panic(err)
		}

		decoded, err := cobs.Decode(frame)
		if err != nil {
			log.Println(err)
			continue
		}

		cmd := decoded[0]
		rr := bytes.NewReader(decoded[1:])

		if cmd == CMD_MEASUREMENT | CMD_RESPONSE {
			t := Measurement{}
			err = binary.Read(rr, binary.LittleEndian, &t)
			measurements <- t
		} else if cmd == CMD_GETPOS | CMD_RESPONSE {
			t := TargetPositionResponse{}
			err = binary.Read(rr, binary.LittleEndian, &t)
			if err != nil {
				continue
			}
			events <- Event{
				name: "target_position",
				data: t,
			}
		} else if cmd == CMD_GETDIM | CMD_RESPONSE {
			err = binary.Read(rr, binary.LittleEndian, &dim)
			if err != nil {
				fmt.Println(err)
				continue
			}

			fmt.Println("Dimension:", dim)
		} else if cmd == CMD_ERROR_RESPONSE {
			var size byte
			err = binary.Read(rr, binary.LittleEndian, &size)
			if err != nil {
				fmt.Println(err)
				continue
			}

			errorMsg := make([]byte, size)

			err = binary.Read(rr, binary.LittleEndian, &errorMsg)
			if err != nil {
				fmt.Println(err)
				continue
			}

			fmt.Printf("Error: %s\n", string(errorMsg))
		} else {
			fmt.Println("Unknown cmd %s", cmd)
		}
	}
}