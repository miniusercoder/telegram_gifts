package main

import (
	"errors"
	"os"
	"strings"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func sessionInput(file, explicitString string) (string, string, error) {
	text := strings.TrimSpace(explicitString)
	if text == "" {
		data, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			return file, "", nil
		}
		if err != nil {
			return "", "", errors.New("cannot read session input")
		}
		text = strings.TrimSpace(string(data))
		if !strings.HasPrefix(text, "1Bv") {
			// Binary AES ciphertext must contain complete blocks.
			if len(data)%16 != 0 {
				return "", "", errors.New("invalid binary session length")
			}
			return file, "", nil
		}
	}
	var session tg.Session
	if err := session.Decode(text); err != nil || len(session.Key) != 256 || len(session.Hash) != 8 {
		// Never return decoder errors: they may include session contents.
		return "", "", errors.New("invalid string session")
	}
	// Leave the source file untouched; SDK persistence stays in memory.
	return "", text, nil
}
