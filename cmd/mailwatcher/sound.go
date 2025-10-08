package main

import (
	"bytes"
	_ "embed"
	"io"
	"log"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

//go:embed assets/notification.mp3
var notificationSoundData []byte

func audioData() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(notificationSoundData))
}

func streamSeakCloser() (beep.StreamSeekCloser, beep.Format, error) {
	// Decode the MP3 file
	streamer, format, err := mp3.Decode(io.NopCloser(audioData()))
	if err != nil {
		log.Fatalf("Failed to decode MP3: %v", err)
	}
	return streamer, format, err
}


func init() {
	// Decode the MP3 file
	streamer, format, err := streamSeakCloser()
	if err != nil {
		log.Fatalf("Failed to decode MP3: %v", err)
	}
	defer streamer.Close()

	// Initialize the speaker with the sample rate from the MP3 file
	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Fatalf("Failed to initialize speaker: %v", err)
	}
}

func playNotification() {
	// Decode the MP3 file
	streamer, _, err := streamSeakCloser()
	if err != nil {
		log.Fatalf("Failed to decode MP3: %v", err)
	}
	defer streamer.Close()

	// Play the sound
	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	// Wait until the sound is finished playing
	<-done
}
