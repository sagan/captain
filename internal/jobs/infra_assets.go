package jobs

import (
	"context"
	"fmt"
	"github.com/zeptop-dev/captain/internal/notify"
	"github.com/zeptop-dev/captain/internal/store"
	"time"
)

// Infrastructure reminders are operator notices, independent of customer
// balances/subscriptions. Daily durable receipts avoid a restart message storm.
func (r *Runner) infraReminders(ctx context.Context, at time.Time) {
	if at.Sub(r.lastInfraReminders) < time.Hour {
		return
	}
	r.lastInfraReminders = at
	if e := r.Store.PruneInfraReminders(ctx, at); e != nil {
		r.Log.Warn("prune asset reminders", "err", e)
	}
	if r.Bot == nil {
		return
	}
	var cfg struct {
		BotToken    string `json:"bot_token"`
		AdminChatID int64  `json:"admin_chat_id"`
	}
	if e := r.Store.GetSetting(ctx, store.SettingTelegram, &cfg); e != nil || cfg.BotToken == "" || cfg.AdminChatID == 0 {
		return
	}
	ctx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	assets, e := r.Store.InfraAssets(ctx)
	if e != nil {
		r.Log.Warn("asset reminders", "err", e)
		return
	}
	for _, a := range assets {
		if ctx.Err() != nil {
			break
		}
		claimed, e := r.Store.ClaimInfraReminder(ctx, a, at)
		if e != nil {
			r.Log.Warn("claim asset reminder", "err", e)
			continue
		}
		if !claimed {
			continue
		}
		text := fmt.Sprintf("Infrastructure renewal: <b>%s</b> (%s)\nDue: %s · %s %d minor units\nReview and record the supplier payment in Captain.", notify.Escape(a.Name), notify.Escape(a.SupplierName), a.NextDue, a.Currency, a.AmountMinor)
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		e = r.Bot.NotifyAdmin(sendCtx, text)
		cancel()
		if err := r.Store.FinishInfraReminder(ctx, a, at, e == nil); err != nil {
			r.Log.Warn("finish asset reminder", "err", err)
		}
		if e != nil {
			r.Log.Warn("send asset reminder", "err", e)
		}
	}
}
