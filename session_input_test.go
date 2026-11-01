package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func syntheticSession() string {
	s := tg.Session{Key: bytes.Repeat([]byte{7}, 256), Hash: bytes.Repeat([]byte{8}, 8), Hostname: "149.154.167.50", AppID: 2040}
	return s.Encode()
}

func TestSessionInputStringFilePreserved(t *testing.T) {
	token := syntheticSession()
	data := []byte("\n" + token + "\r\n")
	if len(data)%16 == 0 {
		data = append(data, '\n')
	}
	file := filepath.Join(t.TempDir(), "fixture.session")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	gotFile, gotString, err := sessionInput(file, "")
	if err != nil {
		t.Fatalf("valid string session of %d bytes rejected: %v", len(data), err)
	}
	if gotFile != "" || gotString != token {
		t.Fatal("string session must bypass binary storage")
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(after, data) {
		t.Fatal("session input rewrote the source file")
	}
}

func TestSessionInputFormats(t *testing.T) {
	shortKey := tg.Session{Key: []byte{7}, Hash: bytes.Repeat([]byte{8}, 8)}
	shortHash := tg.Session{Key: bytes.Repeat([]byte{7}, 256), Hash: []byte{8}}
	legacy := "1BvX" + base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("K", 256)+":_:"+strings.Repeat("H", 8)+":_:149.154.167.50:_:4:_:2040"))
	tests := []struct {
		name     string
		data     []byte
		explicit string
		wantText string
		wantErr  bool
	}{
		{"binary", bytes.Repeat([]byte{0x80}, 32), "", "", false},
		{"empty", nil, "", "", false},
		{"truncated binary", bytes.Repeat([]byte{0x80}, 31), "", "", true},
		{"truncated string", []byte(syntheticSession()[:30]), "", "", true},
		{"invalid prefix", []byte("1BvZ" + strings.Repeat("A", 28)), "", "", true},
		{"invalid encoding", []byte("1BvE!invalid"), "", "", true},
		{"short key", []byte(shortKey.Encode()), "", "", true},
		{"short hash", []byte(shortHash.Encode()), "", "", true},
		{"legacy", []byte(legacy), "", legacy, false},
		{"explicit", []byte("ignored malformed file"), "\n" + syntheticSession() + "\n", syntheticSession(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "fixture.session")
			if err := os.WriteFile(file, tt.data, 0600); err != nil {
				t.Fatal(err)
			}
			gotFile, gotString, err := sessionInput(file, tt.explicit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error present = %v, want %v", err != nil, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotString != tt.wantText || (tt.wantText != "" && gotFile != "") || (tt.wantText == "" && gotFile != file) {
				t.Fatal("wrong binary/string session selection")
			}
		})
	}
}

func TestSessionInputMissingFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "missing.session")
	gotFile, gotString, err := sessionInput(file, "")
	if err != nil || gotFile != file || gotString != "" {
		t.Fatal("missing file must retain SDK initialization semantics")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("session input created the missing file")
	}
}
