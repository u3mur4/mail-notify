package main

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
)

var notificationSound = "/tmp/notification.mp3"

func init() {
	if _, err := os.Stat(notificationSound); !errors.Is(err, os.ErrNotExist) {
		return
	}

	// decode and load the default notification sound
	data, err := Assets.Open("notification.mp3")
	if err != nil {
		log.WithError(err).Fatal("cannot open notification sound")
		return
	}

	b, err := ioutil.ReadAll(data)
	if err != nil {
		log.WithError(err).Fatal("cannot read notification sound")
		return
	}

	ioutil.WriteFile(notificationSound, b, 0777)
}

func playNotificationSound() error {
	cmd := exec.Command("mpv", notificationSound)
	return cmd.Run()

}
