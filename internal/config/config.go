package config

import (
	"flag"
	"os"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.RunAddress, "a", ":8080", "Server address")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "Database URI")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", "", "Accrual system address")
	flag.Parse()

	if envRunAddr := os.Getenv("RUN_ADDRESS"); envRunAddr != "" {
		cfg.RunAddress = envRunAddr
	}
	if envDBURI := os.Getenv("DATABASE_URI"); envDBURI != "" {
		cfg.DatabaseURI = envDBURI
	}
	if envAccrualAddr := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualAddr != "" {
		cfg.AccrualSystemAddress = envAccrualAddr
	}

	return cfg
}
