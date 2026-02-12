package logger

import "go.uber.org/zap"

var Lg *zap.Logger

func InitLogger() {
	logger, _ := zap.NewProduction()
	Lg = logger
}
