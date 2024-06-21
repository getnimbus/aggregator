package alert

import (
	"context"
	"os"
	"time"

	"github.com/getnimbus/ultrago/u_logger"
	"github.com/gtuk/discordwebhook"
	"github.com/yudppp/throttle"
)

var throttler = throttle.New(time.Second)

func AlertDiscord(ctx context.Context, message string) {
	ctx, logger := u_logger.GetLogger(ctx)

	discordEnv := os.Getenv("DISCORD_WEBHOOK")
	if discordEnv == "" {
		logger.Warnf("discord webhook is not set")
		return
	}

	throttler.Do(func() {
		if err := discordwebhook.SendMessage(discordEnv, discordwebhook.Message{
			Content: &message,
		}); err != nil {
			logger.Warnf("failed to send message to discord: %v", err)
			return
		}
	})
}
