package main

import (
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeMascot(writer io.Writer, path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	mascots := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") {
			file := filepath.Join(path, name)
			mascots = append(mascots, file)
		}
	}

	if len(mascots) == 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())

	mascotsLength := len(mascots)
	mascotIndex := rand.Intn(mascotsLength)
	data, err := os.ReadFile(mascots[mascotIndex])
	if err != nil {
		return err
	}
	data = append(data, '\n')
	writer.Write(data)

	return nil
}
