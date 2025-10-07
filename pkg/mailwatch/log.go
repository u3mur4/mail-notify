package mailwatch

import (
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	log = logrus.New()

	log.SetOutput(os.Stderr)

	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true, // Enable full timestamps
		ForceColors:   true, // Force colored output
	})

	// Set log level to debug, info, warn, error, etc.
	log.SetLevel(logrus.WarnLevel)
}

func GetLogger() *logrus.Logger {
	return log
}
