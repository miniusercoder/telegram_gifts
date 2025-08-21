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
