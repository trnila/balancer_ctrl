package main

import "fmt"
import "log"
import (
	"net/http"
	"encoding/json"
	"os"
	"github.com/trnila/go-sse"
	"github.com/caarlos0/env"
)

type config struct {
	Listen string `env:"CTRL_BIND" envDefault:":3000"`
	SerialPath string `env:"CTRL_SERIAL,required"`
	BaudRate uint `env:"CTRL_SERIAL_BAUD" envDefault:"460800"`
}

var cfg config
var dim DimensionResponse

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


func apiSetTarget(w http.ResponseWriter, r *http.Request) {
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

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/set_target", apiSetTarget)

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
	go startBroadcasting(s, measurements, events)

	fmt.Printf("Listening on %s\n", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, nil))
}

func startBroadcasting(s *sse.Server, measurements chan Measurement, events chan Event) {
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

		case event := <-events:
			b, err := json.Marshal(event.data)
			if err != nil {
				fmt.Print(err)
				continue
			}

			msg := sse.NewMessage("", string(b), event.name)
			s.SendMessage("/events/measurements", msg)
		}
	}
}
