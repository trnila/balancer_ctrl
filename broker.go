package main

type Broker struct {
	broadcast chan interface {}
	subscribe chan chan interface {}
	unsubscribe chan chan interface {}
}

func NewBroker() *Broker {
	return &Broker{
		broadcast: make(chan interface{}),
		subscribe: make(chan chan interface{}),
		unsubscribe: make(chan chan interface{}),
	}
}

func (b *Broker) Start() {
	channels := map[chan interface{}]int{}
	lastId := 0

	for {
		select {
			case c := <- b.subscribe:
				channels[c] = lastId
				lastId += 1

			case c := <- b.unsubscribe:
				delete(channels, c)

			case data := <- b.broadcast:
				for channel := range channels {
					channel <- data
				}
		}
	}
}

func (b *Broker) Subscribe() <- chan interface{} {
	c := make(chan interface{}, 8)
	b.subscribe <- c
	return c
}

func (b *Broker) Unsubscribe(c chan interface{}) {
	b.unsubscribe <- c
}

func (b *Broker) Broadcast(data interface{}) {
	b.broadcast <- data
}

