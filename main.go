package main

import "C"
import (
	tg "github.com/amarnathcjd/gogram/telegram"
)

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

//export Init
func Init(appId C.int, appHash *C.char, sessionFile *C.char) C.int {
	var err error

	appHashGo := C.GoString(appHash)
	sessionFileGo := C.GoString(sessionFile)

	client, err = tg.NewClient(tg.ClientConfig{
		AppID:        int32(appId),
		AppHash:      appHashGo,
		Session:      sessionFileGo,
		LogLevel:     tg.LogInfo,
		DisableCache: true,
		NoUpdates:    true,
	})
	if err != nil {
		return ErrCreateClient
	}

	err = client.Start()
	if err != nil {
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	return Success
}

//export GetStarsBalance
func GetStarsBalance() C.longlong {
	if client == nil {
		return ErrClientNotInitialized
	}
	err := client.Start()
	if err != nil {
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	starsStatus, err := client.PaymentsGetStarsStatus(false, &tg.InputPeerSelf{})
	if err != nil {
		return ErrGetPaymentForm
	}

	starsStatusAmount := starsStatus.Balance.(*tg.StarsAmountObj).Amount

	return C.longlong(starsStatusAmount)
}

//export GetMyUsername
func GetMyUsername() *C.char {
	if client == nil {
		return C.CString("")
	}
	err := client.Start()
	if err != nil {
		return C.CString("")
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	users, err := client.UsersGetUsers([]tg.InputUser{&tg.InputUserSelf{}})
	if err != nil {
		return C.CString("")
	}

	me := users[0].(*tg.UserObj).Username

	return C.CString(me)
}

//export ValidateRecipient
func ValidateRecipient(username *C.char) C.int {
	if client == nil {
		return ErrClientNotInitialized
	}
	err := client.Start()
	if err != nil {
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	goUsername := C.GoString(username)

	receiver, err := client.ResolveUsername(goUsername)
	if err != nil {
		return ErrResolveUsername
	}

	_, ok := receiver.(*tg.UserObj)
	if !ok {
		return ErrResolveUsername
	}

	return Success
}

//export SendGift
func SendGift(username *C.char, giftID C.longlong, hideName C.int) C.int {
	if client == nil {
		return ErrClientNotInitialized
	}
	err := client.Start()
	if err != nil {
		return ErrStartClient
	}
	defer func(client *tg.Client) {
		_ = client.Stop()
	}(client)

	goUsername := C.GoString(username)

	receiver, err := client.ResolveUsername(goUsername)
	if err != nil {
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
		GiftID:  int64(giftID),
		Message: nil,
	}

	paymentForm, err := client.PaymentsGetPaymentForm(starsInvoice, nil)
	if err != nil {
		return ErrGetPaymentForm
	}

	starGiftForm := paymentForm.(*tg.PaymentsPaymentFormStarGift)

	_, err = client.PaymentsSendStarsForm(starGiftForm.FormID, starsInvoice)
	if err != nil {
		return ErrSendStars
	}

	return Success
}

func main() {}
