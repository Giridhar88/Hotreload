package main

import (
	"time"
)

type Debouncer struct {
	delay  time.Duration
	timer  *time.Timer
	input  chan struct{} // used to signal the debouncer that some events have come
	output chan struct{} // used to signal the runner that the files havent changed for the duration of the delay
}

func newDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay:  delay,
		input:  make(chan struct{}, 1),
		output: make(chan struct{}, 1),
		timer:  time.NewTimer(10 * time.Millisecond),
	}
}
func (d *Debouncer) run() {
	for {
		select {
		case <-d.input:
			d.timer.Reset(d.delay) //file changed reset the countdown
		case <-d.timer.C:
			d.output <- struct{}{} //file expired send signal to the runnner
		}
	}
}
func (d *Debouncer) signal() {
	d.input <- struct{}{}
}
