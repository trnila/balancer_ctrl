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
	SerialPath string `env:"CTRL_SERIAL"`
	BaudRate uint `env:"CTRL_SERIAL_BAUD" envDefault:"460800"`
	MulticastIPv6 string `env:"CTRL_MULTICAST"`
}

var cfg config
var dim DimensionResponse
var lastTarget TargetPositionResponse

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


func sendJSON(client *sse.Client, event string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	client.SendMessage(sse.NewMessage("", string(b), event))

	return nil
}

func broadcastJSON(server *sse.Server, event string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	server.SendMessage("/events/measurements", sse.NewMessage("", string(b), event))

	return nil
}


func main() {
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%+v\n", cfg)

	broker := NewBroker()
	go broker.Start()

	if len(cfg.MulticastIPv6) > 0 {
		go startMulticast(cfg.MulticastIPv6, broker.Subscribe())
	}

	measurements := broker.Subscribe()
	events := make(chan Event, 10)


	if cfg.SerialPath == "" {
		go producer_random(broker, events, commands)
	} else {
		go producer(broker, events, commands)
	}

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/set_target", apiSetTarget)

	options := sse.Options{
		ClientConnected: func(client *sse.Client) {
			err := sendJSON(client, "dimension", dim)
			if err != nil {
				fmt.Print(err)
			}


			err = sendJSON(client, "target_position", lastTarget)
			if err != nil {
				fmt.Print(err)
			}
		},
	}
	s := sse.NewServer(&options)

	http.Handle("/events/", s)
	go startBroadcasting(s, measurements, events)

	fmt.Printf("Listening on %s\n", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, nil))
}

func startBroadcasting(s *sse.Server, measurements <- chan interface{}, events chan Event) {
	for {
		select {
		case measurement := <-measurements:
			err := broadcastJSON(s, "measurement", measurement)
			if err != nil {
				fmt.Print(err)
			}

		case event := <-events:
			if event.name == "target_position" {
				target, ok := event.data.(TargetPositionResponse)
				if ok {
					lastTarget = target
				}
			}

			err := broadcastJSON(s, event.name, event.data)
			if err != nil {
				fmt.Print(err)
			}
		}
	}
}
