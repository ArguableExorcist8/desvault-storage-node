package logger

import (
	"log"
	"os"
)

var Logger *log.Logger

func InitLogger() {
	file, err := os.OpenFile("p2p.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}

	Logger = log.New(file, "P2P ", log.Ldate|log.Ltime|log.Lshortfile)
	Logger.Println("Logger initialized.")
}
