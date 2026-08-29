package gotel

import "os"

type Config struct {
	serviceName    string `env:"SERVICE_NAME"      envDefault:"gotel"`
	serviceVersion string `env:"SERVICE_VERSION"   envDefault:"0.0.1"`
	enabled        bool   `env:"TELEMETRY_ENABLED" envDefault:"true"`
}

func NewConfigFromEnv() (Config) {
	telem := Config{
		serviceName: os.Getenv("SERVICE_NAME"),
		serviceVersion: os.Getenv("SERVICE_VERSION"),
		enabled: os.Getenv("TELEMETRY_ENABLED") == "true",
	}
 
	return telem
}
