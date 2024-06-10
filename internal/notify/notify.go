package notify

import (
	"strings"

	gonotify "github.com/martinlindhe/notify"
)

func Send(title string, lines ...string) {
	gonotify.Notify("BlockPI RPC Aggregator", title, strings.Join(lines, "\n"), "")
}

func SendNotice(lines ...string) {
	Send("Notice: Aggregator Notice", lines...)
}

func SendError(lines ...string) {
	Send("Alert: An error occurred", lines...)
}
