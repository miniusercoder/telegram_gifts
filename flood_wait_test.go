package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	gogram "github.com/amarnathcjd/gogram"
	tg "github.com/amarnathcjd/gogram/telegram"
)

func TestPreSendFloodWaitCodes(t *testing.T) {
	// Removing typed error handling loses the duration hidden by RpcError.Error().
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"rpc description", &tg.RpcError{Code: 420, Message: "FLOOD_WAIT_X", Description: "Please wait 420 seconds before repeating the action."}, -1420},
		{"wrapped rpc", fmt.Errorf("resolving username: %w", &tg.RpcError{Code: 420, Message: "FLOOD_WAIT_X", Description: "Please wait 1800 seconds before repeating the action."}), -2800},
		{"native additional info", &gogram.ErrResponseCode{Code: 420, Message: "FLOOD_WAIT_X", AdditionalInfo: 57}, -1057},
		{"wrapped native", fmt.Errorf("resolving username: %w", &gogram.ErrResponseCode{Code: 420, Message: "FLOOD_WAIT_X", AdditionalInfo: 8}), -1008},
		{"raw numeric", errors.New("resolving username: FLOOD_WAIT_21 (420)"), -1021},
		{"premium", &tg.RpcError{Code: 420, Message: "FLOOD_PREMIUM_WAIT_12"}, -1012},
		{"invalid username", &tg.RpcError{Code: 400, Message: "USERNAME_INVALID"}, -3},
		{"missing username", &tg.RpcError{Code: 400, Message: "USERNAME_NOT_OCCUPIED"}, -3},
		{"unrelated wait prose", errors.New("Please wait 420 seconds before repeating the action."), -3},
		{"transport error", errors.New("unexpected EOF"), -3},
		{"zero wait", errors.New("FLOOD_WAIT_0"), -3},
		{"int32 cap", &gogram.ErrResponseCode{Code: 420, Message: "FLOOD_WAIT_X", AdditionalInfo: int(1 << 40)}, -2147483648},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountFloodUntil = time.Time{}
			t.Cleanup(func() { accountFloodUntil = time.Time{} })
			if got := preSendErrorCode(tt.err, ErrResolveUsername); got != tt.want {
				t.Fatalf("pre-send code = %d, want %d", got, tt.want)
			}
			if tt.want == -3 && !accountFloodUntil.IsZero() {
				t.Fatal("non-flood error started an account cooldown")
			}
		})
	}
}

func TestPreSendPaymentFormFloodWait(t *testing.T) {
	accountFloodUntil = time.Time{}
	t.Cleanup(func() { accountFloodUntil = time.Time{} })
	err := &tg.RpcError{Code: 420, Message: "FLOOD_WAIT_60"}
	if got := preSendErrorCode(err, ErrGetPaymentForm); got != -1060 {
		t.Fatalf("payment-form flood code = %d, want -1060", got)
	}
	if got := preSendErrorCode(errors.New("INVOICE_INVALID"), ErrGetPaymentForm); got != -4 {
		t.Fatalf("payment-form fallback = %d, want -4", got)
	}
}

func TestCooldownBlocksValidateAndCachedSendWithoutNetwork(t *testing.T) {
	// A zero SDK client panics if a network method is reached.
	previousClient := client
	previousRecipients := resolvedRecipients
	client = &tg.Client{}
	resolvedRecipients = map[string]resolvedRecipient{"cached": {UserID: 1, AccessHash: 2}}
	accountFloodUntil = time.Now().Add(100 * time.Second)
	t.Cleanup(func() {
		client = previousClient
		resolvedRecipients = previousRecipients
		accountFloodUntil = time.Time{}
	})
	for _, call := range []struct {
		name string
		run  func() int
	}{
		{"validate another username", func() int { return ValidateRecipientGo("another") }},
		{"send cached username", func() int { return SendGiftGo("cached", 1, 0, "") }},
	} {
		t.Run(call.name, func(t *testing.T) {
			if got := call.run(); got != -1100 {
				t.Fatalf("cooldown code = %d, want -1100", got)
			}
		})
	}
}

func TestCooldownRoundsUpExpiresAndDoesNotShorten(t *testing.T) {
	t.Cleanup(func() { accountFloodUntil = time.Time{} })
	accountFloodUntil = time.Now().Add(1500 * time.Millisecond)
	if got := activeFloodCode(); got != -1002 {
		t.Fatalf("partial-second cooldown = %d, want -1002", got)
	}
	accountFloodUntil = time.Now().Add(-time.Second)
	if got := activeFloodCode(); got != 0 {
		t.Fatalf("expired cooldown = %d, want 0", got)
	}
	accountFloodUntil = time.Now().Add(time.Hour)
	deadline := accountFloodUntil
	preSendErrorCode(errors.New("FLOOD_WAIT_1"), ErrResolveUsername)
	if !accountFloodUntil.Equal(deadline) {
		t.Fatal("shorter wait shortened an existing account cooldown")
	}
}
