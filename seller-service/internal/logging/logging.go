package logging

import (
	"log"
	"os"
)

func Setup(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	log.SetOutput(file)
	return file, nil
}
