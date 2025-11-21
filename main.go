package main

import "C"
import (
	"log"
	"os"

	tg "github.com/amarnathcjd/gogram/telegram"
)

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

func InitGo(appId int32, appHash string, sessionFile string, sessionString string) int {
	LogInit(sessionFile)

	var err error

	device := tg.DeviceConfig{
		DeviceModel:   "OTHСB52B-Е-EXTREME",
		SystemVersion: "Windows 10",
		AppVersion:    "6.1.1 x64",
		LangCode:      "ru",
	}

	client, err = tg.NewClient(tg.ClientConfig{
		AppID:         appId,
		AppHash:       appHash,
		Session:       sessionFile,
		StringSession: sessionString,
		LogLevel:      tg.LogInfo,
		DeviceConfig:  device,
		DisableCache:  true,
		NoUpdates:     true,
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
	if client == nil {
		return ErrClientNotInitialized
	}
	err := client.Start()
	if err != nil {
		log.Println("Error starting client:", err)
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	starsStatus, err := client.PaymentsGetStarsStatus(false, &tg.InputPeerSelf{})
	if err != nil {
		log.Println("Error getting stars status:", err)
		return ErrGetPaymentForm
	}

	starsStatusAmount := starsStatus.Balance.(*tg.StarsAmountObj).Amount

	return starsStatusAmount
}

func GetMyUsernameGo() string {
	if client == nil {
		return ""
	}
	err := client.Start()
	if err != nil {
		log.Println("Error starting client:", err)
		return ""
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	users, err := client.UsersGetUsers([]tg.InputUser{&tg.InputUserSelf{}})
	if err != nil {
		log.Println("Error getting user:", err)
		return ""
	}

	me := users[0].(*tg.UserObj).Username

	return me
}

func ValidateRecipientGo(username string) int {
	if client == nil {
		return ErrClientNotInitialized
	}
	err := client.Start()
	if err != nil {
		log.Println("Error starting client:", err)
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	receiver, err := client.ResolveUsername(username)
	if err != nil {
		log.Println("Error resolving username:", err)
		return ErrResolveUsername
	}

	_, ok := receiver.(*tg.UserObj)
	if !ok {
		return ErrResolveUsername
	}

	return Success
}

func SendGiftGo(username string, giftID int64, hideName int) int {
	if client == nil {
		return ErrClientNotInitialized
	}
	log.Println("Starting client...")
	err := client.Start()
	if err != nil {
		log.Println("Error starting client:", err)
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	log.Println("Resolving username:", username)
	receiver, err := client.ResolveUsername(username)
	if err != nil {
		log.Println("Error resolving username:", err)
		return ErrResolveUsername
	}

	receiverUser, ok := receiver.(*tg.UserObj)
	if !ok {
		return ErrResolveUsername
	}

	starsInvoice := &tg.InputInvoiceStarGift{
		HideName:       hideName != 0,
		IncludeUpgrade: false,
		Peer: &tg.InputPeerUser{
			UserID:     receiverUser.ID,
			AccessHash: receiverUser.AccessHash,
		},
		GiftID:  giftID,
		Message: nil,
	}

	log.Println("Getting payment form for giftID:", giftID)
	paymentForm, err := client.PaymentsGetPaymentForm(starsInvoice, nil)
	if err != nil {
		log.Println("Error getting payment form:", err)
		return ErrGetPaymentForm
	}

	starGiftForm := paymentForm.(*tg.PaymentsPaymentFormStarGift)

	log.Println("Sending stars gift to:", username)
	_, err = client.PaymentsSendStarsForm(starGiftForm.FormID, starsInvoice)
	if err != nil {
		log.Println("Error sending stars:", err)
		return ErrSendStars
	}

	return Success
}

//export Init
func Init(appId C.int, appHash *C.char, sessionFile *C.char) C.int {
	return C.int(InitGo(int32(appId), C.GoString(appHash), C.GoString(sessionFile), ""))
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
func SendGift(username *C.char, giftID C.longlong, hideName C.int) C.int {
	return C.int(SendGiftGo(C.GoString(username), int64(giftID), int(hideName)))
}

func main() {}
