package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(environment, service, version, level string) (*zap.Logger, error) {
	var parsed zapcore.Level
	if err := parsed.Set(level); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	config := zap.NewProductionConfig()
	if environment == "local" || environment == "test" {
		config = zap.NewDevelopmentConfig()
	}
	config.Level = zap.NewAtomicLevelAt(parsed)
	log, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return log.With(zap.String("service", service), zap.String("version", version), zap.String("environment", environment)), nil
}
