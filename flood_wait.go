package main

import (
	"errors"
	"strings"
	"time"

	gogram "github.com/amarnathcjd/gogram"
	tg "github.com/amarnathcjd/gogram/telegram"
)

// C.int ABI: -1000-seconds, reserving the existing -1..-8 results.
const maxFloodWaitSeconds = int64(1<<31) - 1000

// Protected by clientMu, shared by all recipients on this client account.
var accountFloodUntil time.Time

func preSendErrorCode(err error, fallback int) int {
	if err == nil {
		return fallback
	}
	text := err.Error()
	var rpc *tg.RpcError
	if errors.As(err, &rpc) {
		if rpc.Code != 420 {
			return fallback
		}
		// RpcError.Error omits Description, which holds normalized wait seconds.
		text = rpc.Message + " " + rpc.Description
	}
	var native *gogram.ErrResponseCode
	if errors.As(err, &native) {
		if native.Code != 420 {
			return fallback
		}
		text = native.Message + " " + native.Description
	}
	if !strings.Contains(text, "FLOOD_WAIT_") && !strings.Contains(text, "FLOOD_PREMIUM_WAIT_") {
		return fallback
	}
	seconds := int64(tg.GetFloodWait(errors.New(text)))
	if native != nil {
		if wait, ok := native.AdditionalInfo.(int); ok && wait > 0 {
			seconds = int64(wait)
		}
	}
	if seconds <= 0 {
		return fallback
	}
	if seconds > maxFloodWaitSeconds {
		seconds = maxFloodWaitSeconds
	}
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	if deadline.After(accountFloodUntil) {
		accountFloodUntil = deadline
	}
	return activeFloodCode()
}

func activeFloodCode() int {
	remaining := time.Until(accountFloodUntil)
	if remaining <= 0 {
		return Success
	}
	seconds := int64((remaining + time.Second - 1) / time.Second)
	if seconds > maxFloodWaitSeconds {
		seconds = maxFloodWaitSeconds
	}
	return -1000 - int(seconds)
}
