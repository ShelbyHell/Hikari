package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/panel/subs"
	"mikan/internal/panel/subs/ready"
)

// ReadySubTemplate is a template of the ready catalog (internal/panel/subs/ready), to load into the
// editor: loading saves nothing.
type ReadySubTemplate struct {
	ID      string `json:"id"`
	Core    string `json:"core" enum:"mihomo,singbox,xray"`
	Name    string `json:"name"`
	Summary string `json:"summary" doc:"Описание по-русски; панель переводит его по id"`
	Author  string `json:"author"`
	License string `json:"license" doc:"Лицензия текста шаблона, SPDX"`
	Source  string `json:"source" doc:"Откуда шаблон и списки, на которые он ссылается"`
	Builtin bool   `json:"builtin" doc:"Свой шаблон mikan: панель собирает его из своих настроек, когда его загружают"`
}

type readySubTemplatesOutput struct {
	Body struct {
		Templates []ReadySubTemplate `json:"templates" doc:"По ядрам, в порядке каталога"`
	}
}

type readySubTemplateInput struct {
	ID string `path:"id" maxLength:"64"`
}

type readySubTemplateOutput struct {
	Body struct {
		ReadySubTemplate
		Text string `json:"body" doc:"Текст шаблона: для своего шаблона собран из текущих настроек панели"`
	}
}

func (h *handlers) registerReadySubTemplates() {
	tags := []string{"sub-templates"}
	huma.Register(h.api, huma.Operation{
		OperationID: "list-ready-sub-templates",
		Method:      http.MethodGet,
		Path:        "/api/v1/sub-templates/ready",
		Summary:     "Готовые шаблоны подписки",
		Description: "Каталог, из которого редактор загружает шаблон. В базу панель их сама не кладёт.",
		Tags:        tags,
	}, h.listReadySubTemplates)

	huma.Register(h.api, huma.Operation{
		OperationID: "get-ready-sub-template",
		Method:      http.MethodGet,
		Path:        "/api/v1/sub-templates/ready/{id}",
		Summary:     "Готовый шаблон подписки с текстом",
		Description: "Ничего не сохраняет: текст для редактора.",
		Tags:        tags,
	}, h.getReadySubTemplate)
}

func readyView(r ready.Template) ReadySubTemplate {
	return ReadySubTemplate{
		ID:      r.ID,
		Core:    r.Core,
		Name:    r.Name,
		Summary: r.Summary,
		Author:  r.Author,
		License: r.License,
		Source:  r.Source,
		Builtin: r.Starter,
	}
}

func (h *handlers) listReadySubTemplates(context.Context, *struct{}) (*readySubTemplatesOutput, error) {
	out := &readySubTemplatesOutput{}
	out.Body.Templates = make([]ReadySubTemplate, 0, len(ready.All()))
	for _, r := range ready.All() {
		out.Body.Templates = append(out.Body.Templates, readyView(r))
	}
	return out, nil
}

func (h *handlers) getReadySubTemplate(ctx context.Context, in *readySubTemplateInput) (*readySubTemplateOutput, error) {
	r, ok := ready.Get(in.ID)
	if !ok {
		return nil, huma.Error404NotFound("not_found")
	}
	out := &readySubTemplateOutput{}
	out.Body.ReadySubTemplate, out.Body.Text = readyView(r), r.Body
	if !r.Starter {
		return out, nil
	}
	if h.d.RoutesPreview == nil {
		return nil, huma.Error503ServiceUnavailable("preview_unavailable")
	}
	raw, err := h.d.RoutesPreview(ctx, subs.PreviewRequest{Starter: true})
	if err != nil {
		return nil, err
	}
	out.Body.Text = string(raw)
	return out, nil
}
