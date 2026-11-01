package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

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
			levelColor = "\033[36m"
		case tg.LogWarn:
			levelColor = "\033[33m"
		case tg.LogError:
			levelColor = "\033[31m"
		case tg.LogDebug:
			levelColor = "\033[37m"
		case tg.LogTrace:
			levelColor = "\033[35m"
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

	return fmt.Sprintf("%s %s%s%s %s %s\n", timestamp, levelColor, levelStr, resetColor, fullPrefix, entry.Message)
}

func LogInit(prefix string) {
	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[" + prefix + "] ")
}

var client *tg.Client
var clientMu sync.Mutex

var sessionLockFile *os.File
var sessionFilePath string
var initMu sync.Mutex

const HardProxyURL = ""

// Коды ошибок
const (
	Success                 = 0
	ErrCreateClient         = -1
	ErrStartClient          = -2
	ErrResolveUsername      = -3
	ErrGetPaymentForm       = -4
	ErrSendStars            = -5
	ErrClientNotInitialized = -6
	ErrSessionLocked        = -7
	ErrBadSessionFile       = -8
)

// === КЭШ резолва username -> peer, чтобы не делать второй ResolveUsername ===
type resolvedRecipient struct {
	UserID     int64
	AccessHash int64
}

var resolvedRecipients = map[string]resolvedRecipient{}
var resolvedRecipientsMu sync.RWMutex

func normalizeUsername(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimPrefix(u, "@")
	u = strings.TrimSpace(u)
	u = strings.ToLower(u)
	return u
}

func lockSessionFile(sessionFile string) error {
	// Advisory lock held for lifetime of the client.
	// Prevents concurrent processes from truncating/writing the same session file.
	lockPath := sessionFile + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return err
	}
	sessionLockFile = f
	sessionFilePath = sessionFile
	return nil
}

func unlockSessionFile() {
	if sessionLockFile == nil {
		return
	}
	_ = syscall.Flock(int(sessionLockFile.Fd()), syscall.LOCK_UN)
	_ = sessionLockFile.Close()
	sessionLockFile = nil
	sessionFilePath = ""
}

func InitGo(appId int32, appHash string, sessionFile string, sessionString string, proxyStr string) int {
	LogInit(sessionFile)

	initMu.Lock()
	defer initMu.Unlock()

	// Already initialized
	if client != nil {
		if sessionFilePath == sessionFile {
			return Success
		}
		return ErrCreateClient
	}

	if err := lockSessionFile(sessionFile); err != nil {
		log.Println("Session is locked by another process:", err)
		unlockSessionFile()
		return ErrSessionLocked
	}

	sdkSessionFile, sdkSessionString, err := sessionInput(sessionFile, sessionString)
	if err != nil {
		log.Println("Invalid session input")
			unlockSessionFile()
			return ErrBadSessionFile
	}

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
	logger = logger.SetPrefix("[" + sessionFile + "] ")
	logger = logger.SetFormatter(&UserLogFormatter{
		Prefix: "[" + sessionFile + "] ",
		Color:  true,
	})

	// Prefer proxy from caller; fallback to HardProxyURL.
	proxyURL := strings.TrimSpace(proxyStr)
	if proxyURL == "" {
		proxyURL = HardProxyURL
	}
	if proxyURL != "" {
		fmt.Println("Using proxy:", proxyURL)
		proxy, err = tg.ProxyFromURL(proxyURL)
	}

	if err != nil {
		log.Println("Error creating proxy:", err)
		unlockSessionFile()
		return ErrCreateClient
	}

	client, err = tg.NewClient(tg.ClientConfig{
		AppID:         appId,
		AppHash:       appHash,
		Session:       sdkSessionFile,
		StringSession: sdkSessionString,
		MemorySession: sdkSessionString != "",
		LogLevel:      tg.LogInfo,
		Logger:        logger,
		DeviceConfig:  device,
		DisableCache:  true,
		NoUpdates:     true,
		Proxy:         proxy,
		Timeout:       20,
		ReqTimeout:    20,
		// Surface FLOOD_WAIT immediately; the caller owns retries and cooldown.
		FloodHandler: func(err error) bool { return false },
	})
	if err != nil {
		log.Println("Error creating client:", err)
		client = nil
		unlockSessionFile()
		return ErrCreateClient
	}

	err = client.Start() // интерактивно: телефон/код/2FA
	if err != nil {
		log.Println("Error starting client:", err)
		_ = client.Stop()
		client = nil
		unlockSessionFile()
		return ErrStartClient
	}

	return Success
}

func GetStarsBalanceGo() int64 {
	if client == nil {
		return ErrClientNotInitialized
	}

	clientMu.Lock()
	defer clientMu.Unlock()

	starsStatus, err := client.PaymentsGetStarsStatus(false, &tg.InputPeerSelf{})
	if err != nil {
		log.Println("Error getting stars status:", err)
		return ErrGetPaymentForm
	}

	return starsStatus.Balance.(*tg.StarsAmountObj).Amount
}

func GetMyUsernameGo() string {
	if client == nil {
		return ""
	}

	clientMu.Lock()
	defer clientMu.Unlock()

	users, err := client.UsersGetUsers([]tg.InputUser{&tg.InputUserSelf{}})
	if err != nil {
		log.Println("Error getting user:", err)
		return ""
	}

	return users[0].(*tg.UserObj).Username
}

func ValidateRecipientGo(username string) int {
	if client == nil {
		return ErrClientNotInitialized
	}

	clientMu.Lock()
	defer clientMu.Unlock()
	if code := activeFloodCode(); code != Success {
		return code
	}

	u := normalizeUsername(username)
	receiver, err := client.ResolveUsername(u)
	if err != nil {
		log.Println("Error resolving username:", err)
		return preSendErrorCode(err, ErrResolveUsername)
	}

	receiverUser, ok := receiver.(*tg.UserObj)
	if !ok {
		return ErrResolveUsername
	}

	resolvedRecipientsMu.Lock()
	resolvedRecipients[u] = resolvedRecipient{
		UserID:     receiverUser.ID,
		AccessHash: receiverUser.AccessHash,
	}
	resolvedRecipientsMu.Unlock()

	return Success
}

func SendGiftGo(username string, giftID int64, hideName int, comment string) int {
	if client == nil {
		return ErrClientNotInitialized
	}

	clientMu.Lock()
	defer clientMu.Unlock()
	if code := activeFloodCode(); code != Success {
		return code
	}

	u := normalizeUsername(username)

	resolvedRecipientsMu.RLock()
	rr, ok := resolvedRecipients[u]
	resolvedRecipientsMu.RUnlock()

	if !ok || rr.UserID == 0 || rr.AccessHash == 0 {
		log.Println("Resolving username (SendGift):", u)
		receiver, err := client.ResolveUsername(u)
		if err != nil {
			log.Println("Error resolving username:", err)
			return preSendErrorCode(err, ErrResolveUsername)
		}
		receiverUser, ok2 := receiver.(*tg.UserObj)
		if !ok2 {
			return ErrResolveUsername
		}
		rr = resolvedRecipient{UserID: receiverUser.ID, AccessHash: receiverUser.AccessHash}
		resolvedRecipientsMu.Lock()
		resolvedRecipients[u] = rr
		resolvedRecipientsMu.Unlock()
	}

	starsInvoice := &tg.InputInvoiceStarGift{
		HideName:       hideName != 0,
		IncludeUpgrade: false,
		Peer: &tg.InputPeerUser{
			UserID:     rr.UserID,
			AccessHash: rr.AccessHash,
		},
		GiftID: giftID,
		Message: &tg.TextWithEntities{
			Text:     comment,
			Entities: nil,
		},
	}

	log.Println("Getting payment form for giftID:", giftID)
	paymentForm, err := client.PaymentsGetPaymentForm(starsInvoice, nil)
	if err != nil {
		log.Println("Error getting payment form:", err)
		return preSendErrorCode(err, ErrGetPaymentForm)
	}

	starGiftForm := paymentForm.(*tg.PaymentsPaymentFormStarGift)

	log.Println("Sending stars gift to:", u)
	_, err = client.PaymentsSendStarsForm(starGiftForm.FormID, starsInvoice)
	if err != nil {
		log.Println("Error sending stars:", err)
		return ErrSendStars
	}

	return Success
}

func SendMessageGo(username string, message string) int {
	if client == nil {
		return ErrClientNotInitialized
	}

	clientMu.Lock()
	defer clientMu.Unlock()

	u := normalizeUsername(username)
	receiver, err := client.ResolveUsername(u)
	if err != nil {
		log.Println("Error resolving username:", err)
		return ErrResolveUsername
	}

	receiverPeer, err := client.ResolvePeer(receiver)
	if err != nil {
		log.Println("Error resolving peer:", err)
		return ErrResolveUsername
	}

	_, err = client.MessagesSendMessage(&tg.MessagesSendMessageParams{
		Peer:     receiverPeer,
		Message:  message,
		RandomID: tg.GenerateRandomLong(),
	})
	if err != nil {
		log.Println("Error sending message:", err)
		return ErrSendStars
	}

	return Success
}

func CloseGo() int {
	initMu.Lock()
	defer initMu.Unlock()

	if client == nil {
		unlockSessionFile()
		return Success
	}

	clientMu.Lock()
	defer clientMu.Unlock()

	_ = client.Stop()
	client = nil
	resolvedRecipientsMu.Lock()
	resolvedRecipients = map[string]resolvedRecipient{}
	resolvedRecipientsMu.Unlock()
	unlockSessionFile()
	return Success
}

//export Init
func Init(appId C.int, appHash *C.char, sessionFile *C.char, proxy *C.char) C.int {
	// proxy аргумент игнорируется, если HardProxyURL задан.
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

//export FreeCString
func FreeCString(ptr *C.char) {
	C.free(unsafe.Pointer(ptr))
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

//export Close
func Close() C.int {
	return C.int(CloseGo())
}

func main() {}
