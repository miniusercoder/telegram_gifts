package main

import "C"
import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	tg "github.com/amarnathcjd/gogram/telegram"
)

type UserLogFormatter struct {
	Prefix string
	Color  bool
}

func (f *UserLogFormatter) Format(entry *tg.LogEntry) string {
	var levelColor string
	var resetColor = "\033[0m"

	if f.Color {
		switch entry.Level {
		case tg.LogInfo:
			levelColor = "\033[36m" // Cyan for INFO to match typical logs
		case tg.LogWarn:
			levelColor = "\033[33m" // Yellow
		case tg.LogError:
			levelColor = "\033[31m" // Red
		case tg.LogDebug:
			levelColor = "\033[37m" // White
		case tg.LogTrace:
			levelColor = "\033[35m" // Magenta
		default:
			levelColor = "\033[37m"
		}
	} else {
		resetColor = ""
	}

	var timestamp string
	if t, ok := entry.Time.(time.Time); ok {
		timestamp = t.Format("2006/01/02 15:04:05")
	} else {
		timestamp = fmt.Sprint(entry.Time)
	}

	fullPrefix := f.Prefix
	if entry.Prefix != "" {
		// If entry has its own prefix, we might want to combine them or just ignore entry.Prefix if it's internal noise
		// The user wants a specific prefix. Let's prepend ours.
		// If entry.Prefix is just "mtproto", we want "[session] [mtproto]"
		fullPrefix = f.Prefix + entry.Prefix
	}

	levelStr := "INFO "
	switch entry.Level {
	case tg.LogInfo:
		levelStr = "INFO "
	case tg.LogWarn:
		levelStr = "WARN "
	case tg.LogError:
		levelStr = "ERROR"
	case tg.LogDebug:
		levelStr = "DEBUG"
	case tg.LogTrace:
		levelStr = "TRACE"
	}

	// Format: 2026/01/20 22:04:50 INFO  [stars.dat] [mtproto] connecting...
	return fmt.Sprintf("%s %s%s%s %s %s\n", timestamp, levelColor, levelStr, resetColor, fullPrefix, entry.Message)
}

func LogInit(prefix string) {
	// Время + стандартный формат
	log.SetFlags(log.LstdFlags)
	// Вывод в stderr
	log.SetOutput(os.Stderr)
	log.SetPrefix("[" + prefix + "] ")
}

var client *tg.Client

// Коды ошибок
const (
	Success                 = 0
	ErrCreateClient         = -1
	ErrStartClient          = -2
	ErrResolveUsername      = -3
	ErrGetPaymentForm       = -4
	ErrSendStars            = -5
	ErrClientNotInitialized = -6
)

func InitGo(appId int32, appHash string, sessionFile string, sessionString string, proxyStr string) int {
	LogInit(sessionFile)

	var err error
	var proxy tg.Proxy

	device := tg.DeviceConfig{
		DeviceModel:   "ZJJDH12S4-B-ULTRA",
		SystemVersion: "Windows 10",
		AppVersion:    "6.4.2",
		LangCode:      "ru",
	}

	logger := tg.NewLogger(tg.LogInfo, tg.LoggerConfig{
		Level:  tg.LogInfo,
		Prefix: "[" + sessionFile + "] ",
		Output: os.Stderr,
		Color:  true,
	})

	// Force the prefix again, as NewLogger might not be applying it to the top level correctly
	// or gogram overriden it internally.
	// But since we can't be sure about implementation, we try SetPrefix method if available.
	// Based on my interface discovery, `SetPrefix` exists.
	logger = logger.SetPrefix("[" + sessionFile + "] ")
	logger = logger.SetFormatter(&UserLogFormatter{
		Prefix: "[" + sessionFile + "] ",
		Color:  true,
	})

	if proxyStr != "" {
		proxy, err = tg.ProxyFromURL(proxyStr)
	}

	if err != nil {
		log.Println("Error creating proxy:", err)
		return ErrCreateClient
	}

	client, err = tg.NewClient(tg.ClientConfig{
		AppID:         appId,
		AppHash:       appHash,
		Session:       sessionFile,
		StringSession: sessionString,
		LogLevel:      tg.LogInfo,
		Logger:        logger,
		DeviceConfig:  device,
		DisableCache:  true,
		NoUpdates:     true,
		Proxy:         proxy,
	})
	if err != nil {
		log.Println("Error creating client:", err)
		return ErrCreateClient
	}

	err = client.Start()
	if err != nil {
		log.Println("Error starting client:", err)
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	return Success
}

func GetStarsBalanceGo() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resultCh := make(chan int64, 1)

	go func() {
		if client == nil {
			resultCh <- ErrClientNotInitialized
			return
		}

		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				_ = client.Stop()
			}
		}()

		err := client.Start()
		if err != nil {
			if ctx.Err() == nil {
				log.Println("Error starting client:", err)
				resultCh <- ErrStartClient
			}
			return
		}
		defer func(client *tg.Client) {
			_ = client.Stop()
		}(client)

		if ctx.Err() != nil {
			return
		}

		starsStatus, err := client.PaymentsGetStarsStatus(false, &tg.InputPeerSelf{})
		if err != nil {
			log.Println("Error getting stars status:", err)
			if ctx.Err() == nil {
				resultCh <- ErrGetPaymentForm
			}
			return
		}

		starsStatusAmount := starsStatus.Balance.(*tg.StarsAmountObj).Amount
		select {
		case resultCh <- starsStatusAmount:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-resultCh:
		return res
	case <-ctx.Done():
		log.Println("Timeout reached inside Go wrapper")
		return ErrGetPaymentForm
	}
}

func GetMyUsernameGo() string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resultCh := make(chan string, 1)

	go func() {
		if client == nil {
			resultCh <- ""
			return
		}

		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				_ = client.Stop()
			}
		}()

		err := client.Start()
		if err != nil {
			if ctx.Err() == nil {
				log.Println("Error starting client:", err)
				resultCh <- ""
			}
			return
		}
		defer func(client *tg.Client) {
			_ = client.Stop()
		}(client)

		if ctx.Err() != nil {
			return
		}

		users, err := client.UsersGetUsers([]tg.InputUser{&tg.InputUserSelf{}})
		if err != nil {
			log.Println("Error getting user:", err)
			if ctx.Err() == nil {
				resultCh <- ""
			}
			return
		}

		me := users[0].(*tg.UserObj).Username
		select {
		case resultCh <- me:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-resultCh:
		return res
	case <-ctx.Done():
		log.Println("Timeout reached inside Go wrapper")
		return ""
	}
}

func ValidateRecipientGo(username string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resultCh := make(chan int, 1)

	go func() {
		if client == nil {
			resultCh <- ErrClientNotInitialized
			return
		}

		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				_ = client.Stop()
			}
		}()

		err := client.Start()
		if err != nil {
			if ctx.Err() == nil {
				log.Println("Error starting client:", err)
				resultCh <- ErrStartClient
			}
			return
		}
		defer func(client *tg.Client) {
			_ = client.Stop()
		}(client)

		if ctx.Err() != nil {
			return
		}

		receiver, err := client.ResolveUsername(username)
		if err != nil {
			log.Println("Error resolving username:", err)
			if ctx.Err() == nil {
				resultCh <- ErrResolveUsername
			}
			return
		}

		_, ok := receiver.(*tg.UserObj)
		if !ok {
			if ctx.Err() == nil {
				resultCh <- ErrResolveUsername
			}
			return
		}

		select {
		case resultCh <- Success:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-resultCh:
		return res
	case <-ctx.Done():
		log.Println("Timeout reached inside Go wrapper")
		return ErrResolveUsername
	}
}

func SendGiftGo(username string, giftID int64, hideName int, comment string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resultCh := make(chan int, 1)

	go func() {
		if client == nil {
			resultCh <- ErrClientNotInitialized
			return
		}

		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				_ = client.Stop()
			}
		}()

		log.Println("Starting client...")
		err := client.Start()
		if err != nil {
			if ctx.Err() == nil {
				log.Println("Error starting client:", err)
				resultCh <- ErrStartClient
			}
			return
		}
		defer func() { _ = client.Stop() }()

		if ctx.Err() != nil {
			return
		}

		log.Println("Resolving username:", username)
		receiver, err := client.ResolveUsername(username)
		if err != nil {
			log.Println("Error resolving username:", err)
			if ctx.Err() == nil {
				resultCh <- ErrResolveUsername
			}
			return
		}

		receiverUser, ok := receiver.(*tg.UserObj)
		if !ok {
			if ctx.Err() == nil {
				resultCh <- ErrResolveUsername
			}
			return
		}

		starsInvoice := &tg.InputInvoiceStarGift{
			HideName:       hideName != 0,
			IncludeUpgrade: false,
			Peer: &tg.InputPeerUser{
				UserID:     receiverUser.ID,
				AccessHash: receiverUser.AccessHash,
			},
			GiftID: giftID,
			Message: &tg.TextWithEntities{
				Text:     comment,
				Entities: nil,
			},
		}

		if ctx.Err() != nil {
			return
		}

		log.Println("Getting payment form for giftID:", giftID)
		paymentForm, err := client.PaymentsGetPaymentForm(starsInvoice, nil)
		if err != nil {
			log.Println("Error getting payment form:", err)
			if ctx.Err() == nil {
				resultCh <- ErrGetPaymentForm
			}
			return
		}

		starGiftForm := paymentForm.(*tg.PaymentsPaymentFormStarGift)

		if ctx.Err() != nil {
			return
		}

		log.Println("Sending stars gift to:", username)
		_, err = client.PaymentsSendStarsForm(starGiftForm.FormID, starsInvoice)
		if err != nil {
			log.Println("Error sending stars:", err)
			if ctx.Err() == nil {
				resultCh <- ErrSendStars
			}
			return
		}

		select {
		case resultCh <- Success:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-resultCh:
		return res
	case <-ctx.Done():
		log.Println("Timeout reached inside Go wrapper")
		return ErrSendStars
	}
}

func SendMessageGo(username string, message string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resultCh := make(chan int, 1)

	go func() {
		if client == nil {
			resultCh <- ErrClientNotInitialized
			return
		}

		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				_ = client.Stop()
			}
		}()

		err := client.Start()
		if err != nil {
			if ctx.Err() == nil {
				log.Println("Error starting client:", err)
				resultCh <- ErrStartClient
			}
			return
		}
		defer func() {
			_ = client.Stop()
		}()

		if ctx.Err() != nil {
			return
		}

		receiver, err := client.ResolveUsername(username)
		if err != nil {
			log.Println("Error resolving username:", err)
			if ctx.Err() == nil {
				resultCh <- ErrResolveUsername
			}
			return
		}

		receiverPeer, err := client.ResolvePeer(receiver)
		if err != nil {
			log.Println("Error resolving peer:", err)
			if ctx.Err() == nil {
				resultCh <- ErrResolveUsername
			}
			return
		}

		if ctx.Err() != nil {
			return
		}

		_, err = client.MessagesSendMessage(&tg.MessagesSendMessageParams{
			Peer:     receiverPeer,
			Message:  message,
			RandomID: tg.GenerateRandomLong(),
		})
		if err != nil {
			log.Println("Error sending message:", err)
			if ctx.Err() == nil {
				resultCh <- ErrSendStars
			}
			return
		}

		select {
		case resultCh <- Success:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-resultCh:
		return res
	case <-ctx.Done():
		log.Println("Timeout reached inside Go wrapper")
		return ErrSendStars
	}
}

//export Init
func Init(appId C.int, appHash *C.char, sessionFile *C.char, proxy *C.char) C.int {
	return C.int(InitGo(int32(appId), C.GoString(appHash), C.GoString(sessionFile), "", C.GoString(proxy)))
}

//export GetStarsBalance
func GetStarsBalance() C.longlong {
	return C.longlong(GetStarsBalanceGo())
}

//export GetMyUsername
func GetMyUsername() *C.char {
	return C.CString(GetMyUsernameGo())
}

//export ValidateRecipient
func ValidateRecipient(username *C.char) C.int {
	return C.int(ValidateRecipientGo(C.GoString(username)))
}

//export SendGift
func SendGift(username *C.char, giftID C.longlong, hideName C.int, comment *C.char) C.int {
	return C.int(SendGiftGo(C.GoString(username), int64(giftID), int(hideName), C.GoString(comment)))
}

//export SendMessage
func SendMessage(username *C.char, message *C.char) C.int {
	return C.int(SendMessageGo(C.GoString(username), C.GoString(message)))
}

func main() {}
