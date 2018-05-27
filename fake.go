package main

import (
	"time"
	"math"
)

func createTargetEvt(X, Y int32) Event {
	return Event{
		name: "target_position",
		data: TargetPositionResponse{X: X, Y:Y},
	}
}

func producer_random(broker *Broker, events chan <- Event, commands chan interface{}) {
	dim.Width = 180
	dim.Height = 230

	targetX := dim.Width / 2
	targetY := dim.Height / 2
	events <-  createTargetEvt(targetX, targetY)

	go func() {
		for {
			cmd := <- commands
			pos, ok := cmd.(SetPositionCommand)
			if ok {
				targetX = pos.X
				targetY = pos.Y

				events <-  createTargetEvt(targetX, targetY)
			}
		}
	}()

	go func() {
		timer := time.NewTicker(time.Second)

		for {
			<- timer.C
			events <-  createTargetEvt(targetX, targetY)
		}
	}()


	meas := Measurement{}
	meas.POSX = float32(targetX)
	meas.POSY = float32(targetY)

	var R int32 = 20
	sign := 1
	for {
		for i := -R; i < R; i++ {
			var x = int32(i)
			if sign != 1 {
				x *= -1
			}

			y := math.Sqrt(float64(R*R - x*x));

			meas.POSX = float32(targetX + x)
			meas.POSY = float32(float64(targetY) + y * float64(sign))

			if meas.POSX >= 0 && meas.POSX < float32(dim.Width) && meas.POSY >= 0 && meas.POSY < float32(dim.Height) {
				broker.Broadcast(meas)
			}

			time.Sleep(20 * time.Millisecond)
		}
		sign *= -1

	}
}
