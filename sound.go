package main

import (
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

//go:generate go run -tags=dev assets/assets_generate.go

// notificationSound plays when a new email received
var notificationSound beep.StreamSeekCloser

func init() {
	// decode and load the default notification sound
	data, err := Assets.Open("notification.mp3")
	if err != nil {
		log.WithError(err).Fatal("cannot open notification sound")
		return
	}

	streamer, format, err := mp3.Decode(data)
	if err != nil {
		log.Fatal(err)
	}
	notificationSound = streamer

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.WithError(err).Fatal("cannot initialize speaker")
		return
	}
}
