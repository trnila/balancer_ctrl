package main

import "fmt"
import "log"
import (
	"github.com/jacobsa/go-serial/serial"
	"bufio"
	"github.com/dgryski/go-cobs"
	"encoding/binary"
	"bytes"
	"net/http"
	"encoding/json"
	"time"
	"os"
	"github.com/trnila/go-sse"
	"github.com/caarlos0/env"
)

type config struct {
	Listen string `env:"CTRL_BIND" envDefault:":3000"`
	SerialPath string `env:"CTRL_SERIAL,required"`
	BaudRate uint `env:"CTRL_SERIAL_BAUD" envDefault:"460800"`
}

const CMD_RESPONSE = 128;
const CMD_GETTER = 64;

const CMD_RESET = 0;
const CMD_POS = 1;
const CMD_PID = 2;

const CMD_GETPOS = CMD_GETTER | CMD_POS;
const CMD_GETPID = CMD_GETTER | CMD_PID;
const CMD_GETDIM = CMD_GETTER | (CMD_PID + 1);

const CMD_MEASUREMENT = 0 | CMD_RESPONSE;
const CMD_ERROR_RESPONSE = 255;

var cfg config
var dim DimensionResponse


type Measurement struct {
	CX, CY float32
	VX, VY float32
	POSX, POSY float32
	RVX, RVY float32
	RAX, RAY float32
	NX, NY float32
	RX, RY float32
	USX, USY float32
	RAWX, RAWY float32
}

type ReqSetPos struct {
	X, Y int32
}

type TargetPositionResponse struct {
	X, Y int32
}

type SetPositionCommand struct {
	ID byte
	X, Y int32
}

type DimensionResponse struct {
	Width, Height int32
	X1, Y1 int32
	X2, Y2 int32
}

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


func apiHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var targetPos ReqSetPos
	err := decoder.Decode(&targetPos)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	commands <- SetPositionCommand{
		ID: 1,
		X: targetPos.X,
		Y: targetPos.Y,
	}

	commands <- SimpleCmd(CMD_GETPOS)
	w.WriteHeader(http.StatusOK)
}

var commands = make(chan interface{}, 10)


func main() {
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%+v\n", cfg)

	measurements := make(chan Measurement, 10)
	events := make(chan Event, 10)
	go producer(measurements, events, commands)

	commands <- Cmd{ID:CMD_GETDIM}

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/set_target", apiHandler)

	options := sse.Options{
		ClientConnected: func(client *sse.Client) {
			b, err := json.Marshal(dim)
			if err != nil {
				fmt.Print(err)
				return
			}

			client.SendMessage(sse.NewMessage("", string(b), "dimension"))
		},
	}
	s := sse.NewServer(&options)

	http.Handle("/events/", s)
	go func () {
		for {
			select {
				case measurement := <-measurements:
					b, err := json.Marshal(measurement)
					if err != nil {
						fmt.Print(err)
						continue
					}

					msg := sse.NewMessage("", string(b), "measurement")
					s.SendMessage("/events/measurements", msg)

				case event := <- events:
					b, err := json.Marshal(event.data)
					if err != nil {
						fmt.Print(err)
						continue
					}

					msg := sse.NewMessage("", string(b), event.name)
					s.SendMessage("/events/measurements", msg)
			}
		}
	}()

	fmt.Printf("Listening on %s\n", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, nil))
}
