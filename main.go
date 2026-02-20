package main

import (
	"github.com/fenilmodi00/ipo-backend/config"
	"github.com/fenilmodi00/ipo-backend/internal/app"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.LoadConfig()

	if err := app.Run(cfg); err != nil {
		logrus.WithError(err).Fatal("Application exited with error")
	}
}
