package digest

import (
	"fmt"
	"html"
	"strings"
)

// RenderDigestEmail builds a simple HTML body for the weekly digest. It
// deliberately shows only domain + counts by day part, never a timeline or
// exact time of access, and reuses each category's Reason as a starting
// point for a conversation instead of a new, invented message.
func RenderDigestEmail(houseName string, report []CategoryDigest) string {
	var body strings.Builder
	body.WriteString(fmt.Sprintf("<h1>Resumo semanal — %s</h1>", html.EscapeString(houseName)))

	if len(report) == 0 {
		body.WriteString("<p>Nenhum domínio observado nesta semana.</p>")
		return body.String()
	}

	body.WriteString("<p>Estes são os domínios observados (não bloqueados) esta semana, agrupados por período do dia. Não é uma lista completa de navegação — só o que foi marcado como \"Observar\" no painel.</p>")

	for _, category := range report {
		title := category.GroupName
		if title == "" {
			title = category.Category
		}
		body.WriteString(fmt.Sprintf("<h2>%s</h2>", html.EscapeString(title)))
		if category.Reason != "" {
			body.WriteString(fmt.Sprintf("<p><em>Proposta de conversa:</em> %s</p>", html.EscapeString(category.Reason)))
		}
		body.WriteString("<ul>")
		for _, domain := range category.Domains {
			body.WriteString(fmt.Sprintf(
				"<li>%s — manhã: %d, tarde: %d, noite: %d</li>",
				html.EscapeString(domain.Domain),
				domain.Counts[PeriodMorning],
				domain.Counts[PeriodAfternoon],
				domain.Counts[PeriodEvening],
			))
		}
		body.WriteString("</ul>")
	}
	return body.String()
}
