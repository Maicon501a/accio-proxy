// chatprobe envia um chat assinado via SG (com spoof se ACCIO_SG_SPOOF_SEED
// estiver definido) e imprime a resposta. Reproduz o fluxo do gateway
// (chat -> 555? warm -> retry) sem depender da UI Wails.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"accio-proxy/internal/account"
	"accio-proxy/internal/config"
	"accio-proxy/internal/phoenix"
	"accio-proxy/internal/sgbridge"

	_ "modernc.org/sqlite"
)

func main() {
	log.Printf("chatprobe: seed=%q utdid spoof env=%v", os.Getenv("ACCIO_SG_SPOOF_SEED"), os.Getenv("SGTRACE_SPOOF_SEED"))
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := account.Open(cfg.AccountsDB())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	email, token, err := st.Pick(context.Background())
	if err != nil {
		log.Fatalf("pick: %v", err)
	}
	log.Printf("chatprobe: account=%s", email)

	utdid := cfg.Utdid
	if v := os.Getenv("ACCIO_PROBE_UTDID"); v != "" {
		utdid = v
		log.Printf("chatprobe: utdid override=%q", v)
	}
	signer := sgbridge.New(cfg.DataDir)
	defer signer.Close()
	client := phoenix.New(signer, utdid, "0.33.0", sgbridge.DesktopAppKey)

	req := map[string]any{
		"model":    "auto",
		"messages": []any{map[string]any{"role": "user", "content": "Reply with exactly: OK"}},
	}

	try := func(ctx context.Context) (string, error) {
		var text string
		emit := func(t, _ string, call map[string]any, _ bool) {
			text += t
			if call != nil {
				fmt.Printf("[toolcall] %v\n", call)
			}
		}
		err := client.StreamChat(ctx, req, token, emit)
		return text, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	text, err := try(ctx)
	if phoenix.IsRateLimited(err) {
		log.Printf("chatprobe: 555 — warm device + retry")
		warmCtx, c2 := context.WithTimeout(context.Background(), 50*time.Second)
		client.WarmDevice(warmCtx)
		c2()
		text, err = try(ctx)
	}
	if err != nil {
		if ge, ok := err.(*phoenix.GatewayError); ok {
			fmt.Printf("GATEWAY_ERROR status=%d\nbody=%.600s\n", ge.Status, ge.Raw)
			os.Exit(2)
		}
		log.Fatalf("chatprobe: %v", err)
	}
	fmt.Printf("REPLY: %s\n", text)
}
