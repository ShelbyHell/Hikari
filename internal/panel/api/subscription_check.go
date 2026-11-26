package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/panel/subs"
)

type subscriptionCheckInput struct {
	ID     int64  `path:"id" minimum:"1"`
	App    string `query:"app" required:"true" enum:"happ,clash"`
	Mihomo string `query:"mihomo_version" maxLength:"15" pattern:"^v?[0-9]{1,4}[.][0-9]{1,4}[.][0-9]{1,4}$" doc:"Для Clash: версия ядра, например 1.19.34. Без неё — приложение не сообщило версию"`
}

type subscriptionCheckOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         subs.SubscriptionCheck
}

func (h *handlers) registerSubscriptionCheck(tags []string) {
	huma.Register(h.api, huma.Operation{
		OperationID: "check-user-subscription",
		Method:      http.MethodGet,
		Path:        "/api/v1/users/{id}/subscription-check",
		Summary:     "Проверить состав подписки для Happ или Clash, без выдачи ключей и привязки устройства",
		Tags:        tags,
	}, h.checkUserSubscription)
}

func (h *handlers) checkUserSubscription(ctx context.Context, in *subscriptionCheckInput) (*subscriptionCheckOutput, error) {
	if h.d.CheckSubscription == nil {
		return nil, huma.Error503ServiceUnavailable("preview_unavailable")
	}
	u, err := h.d.Users.Get(ctx, in.ID)
	if err != nil {
		return nil, mapDomainErr(err)
	}
	app := subs.DetectApp("Happ")
	if in.App == "clash" {
		app = subs.DetectApp("mihomo/" + in.Mihomo)
	}
	report, err := h.d.CheckSubscription(ctx, u, app)
	if err != nil {
		return nil, err
	}
	return &subscriptionCheckOutput{CacheControl: "no-store", Body: report}, nil
}
