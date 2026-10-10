// Package factory orchestrates on-demand account creation.
package factory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"accio-proxy/internal/account"
	"accio-proxy/internal/emulator"
)

// TempMail is the pluggable temp-email provider (RE of the real site comes
// later; the interface is already the contract the factory consumes).
type TempMail interface {
	// NewInbox returns a fresh email address.
	NewInbox(ctx context.Context) (email string, err error)
	// WaitForCode polls the inbox for the Accio verification code.
	WaitForCode(ctx context.Context, email string, timeout time.Duration) (code string, err error)
}

// TokenHarvester watches MITM captures and extracts session artifacts
// (accessToken from the phoenix WS handshake URL).
type TokenHarvester interface {
	// SnapshotNow marks every token currently in the capture feed as seen.
	// Call before driving a login so the harvest only accepts tokens minted
	// by that login.
	SnapshotNow(ctx context.Context) error
	// WaitForAccessToken blocks until a fresh WS handshake with a new
	// accessToken appears in the capture feed (or timeout).
	WaitForAccessToken(ctx context.Context, since time.Time, timeout time.Duration) (token, connectionToken string, err error)
}

// Factory creates accounts until the requested count is reached.
type Factory struct {
	Store    *account.Store
	Driver   *emulator.Driver
	Mail     TempMail
	Harvest  TokenHarvester
	Log      func(format string, args ...any)
}

// CreateN creates up to n accounts, logging progress. Returns the number
// created. The loop is resilient: a failed attempt logs and continues.
func (f *Factory) CreateN(ctx context.Context, n int) (int, error) {
	created := 0
	for i := 0; i < n; i++ {
		if ctx.Err() != nil {
			return created, ctx.Err()
		}
		if err := f.createOne(ctx); err != nil {
			f.log("account %d/%d failed: %v", i+1, n, err)
			continue
		}
		created++
		f.log("account %d/%d created", i+1, n)
	}
	return created, nil
}

func (f *Factory) createOne(ctx context.Context) error {
	email, err := f.Mail.NewInbox(ctx)
	if err != nil {
		return fmt.Errorf("tempmail: %w", err)
	}
	f.log("[%s] starting signup flow", email)
	if err := f.Harvest.SnapshotNow(ctx); err != nil {
		f.log("[%s] snapshot: %v (continuing)", email, err)
	}
	if err := f.Driver.LoginEmailFlow(ctx, email); err != nil {
		return fmt.Errorf("ui flow: %w", err)
	}
	code, err := f.Mail.WaitForCode(ctx, email, 3*time.Minute)
	if err != nil {
		return fmt.Errorf("code: %w", err)
	}
	f.log("[%s] code received: %s", email, code)
	if err := f.Driver.EnterCodeFlow(ctx, code); err != nil {
		return fmt.Errorf("enter code: %w", err)
	}
	// Wait for the post-login WS handshake and harvest the token.
	since := time.Now()
	token, _, err := f.Harvest.WaitForAccessToken(ctx, since, 60*time.Second)
	if err != nil {
		return fmt.Errorf("harvest: %w", err)
	}
	f.log("[%s] token harvested", email)
	_, err = f.Store.Insert(account.Account{
		Email: email, AccessToken: token, Status: "active",
		Note: "factory",
	})
	return err
}

func (f *Factory) log(format string, args ...any) {
	if f.Log != nil {
		f.Log(format, args...)
	}
}

// ManualLogin drives the manual login flow from the UI (user pastes email,
// receives code, types it in the frontend).
type ManualLogin struct {
	Driver  *emulator.Driver
	Harvest TokenHarvester
	Store   *account.Store
}

// StartEmail begins the manual login: types the email and taps send-code.
func (m *ManualLogin) StartEmail(ctx context.Context, email string) error {
	return m.Driver.LoginEmailFlow(ctx, email)
}

// FinishCode enters the code, waits for the WS handshake, harvests the
// token and persists the account.
func (m *ManualLogin) FinishCode(ctx context.Context, email, code string) error {
	if err := m.Harvest.SnapshotNow(ctx); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	since := time.Now()
	if err := m.Driver.EnterCodeFlow(ctx, code); err != nil {
		return err
	}
	token, _, err := m.Harvest.WaitForAccessToken(ctx, since, 60*time.Second)
	if err != nil {
		return fmt.Errorf("harvest: %w", err)
	}
	_, err = m.Store.Insert(account.Account{Email: email, AccessToken: token, Status: "active", Note: "manual"})
	return err
}

var _ = strings.TrimSpace
