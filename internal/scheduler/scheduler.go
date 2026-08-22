// Package scheduler implements the Cosca `scheduler.cron` capability
// (KERNEL.md STABLE): cron-based scheduling of deterministic commands.
//
// Schedules are expressed in natural language (pt-BR/EN) and normalized into
// three kinds:
//
//	interval  "every 30m", "a cada 2h", "every 15s", "every 5m30s"
//	daily     "daily at 9am", "diariamente às 9:00", "every day at 08:30"
//	cron      "0 9 * * *" (5-field cron subset)
//
// The parser is stdlib-only (time/strings/regexp) — no robfig/cron dependency.
// Cron evaluation is a minute-by-minute matcher over the 5 standard fields
// (minute hour day-of-month month day-of-week) with a 2-year lookahead cap.
// See CronLimitations() for the documented subset.
package scheduler

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Kinds of schedules.
const (
	KindInterval = "interval" // "every 30m" — intervalo fixo
	KindDaily    = "daily"    // "daily at 9am" — uma hora fixa por dia
	KindCron     = "cron"     // "0 9 * * *" — expressão cron de 5 campos
)

// MaxCronLookaheadMinutes limita a busca de próxima ocorrência de um cron a
// 2 anos (em minutos). Acima disso NextAfter devolve erro — nenhum job fica
// esperando indefinidamente.
const MaxCronLookaheadMinutes = 2 * 365 * 24 * 60

// Schedule é uma agenda normalizada a partir da linguagem natural.
// Kind define qual campo é relevante:
//
//	interval → Interval
//	daily    → Hour + Minute
//	cron     → Cron (string crua de 5 campos)
type Schedule struct {
	Kind     string        `json:"kind"`     // "interval" | "daily" | "cron"
	Interval time.Duration `json:"interval"` // para kind=interval
	Cron     string        `json:"cron"`     // "0 9 * * *" para kind=cron
	Hour     int           `json:"hour"`     // para kind=daily
	Minute   int           `json:"minute"`   // para kind=daily
	Raw      string        `json:"raw"`      // texto original ex.: "every 30m"
}

// CronLimitations documenta o subconjunto de cron implementado. É o contrato
// público do parser: o que funciona e o que NÃO funciona (limitação conhecida).
func CronLimitations() string {
	return `Cron suportado (5 campos): minuto hora dia-do-mês mês dia-da-semana.

Campos aceitam: valor único (0-59 / 0-23 / 1-31 / 1-12 / 0-7), "*", passo
"*/N", intervalo "a-b", intervalo com passo "a-b/N" e listas separadas por
vírgula (ex.: "0,15,30,45 * * * *").

Dia da semana: 0 e 7 = domingo. Quando dia-do-mês E dia-da-semana estão ambos
restritos, vale a semântica padrão do cron (OR — casa se qualquer um casar).

NÃO suportado (limitação conhecida):
  - campo de segundos (cron de 6 campos);
  - nomes de mês/dia (jan, mon);
  - "?", "L", "W", "#";
  - atalhos "@" (reboot, daily, hourly, ...);
  - a busca da próxima ocorrência é limitada a 2 anos (erro além disso).
`
}

// ---------------------------------------------------------------------------
// ParseNL — parser de linguagem natural
// ---------------------------------------------------------------------------

// errEmptySchedule é o erro para agenda vazia.
var errEmptySchedule = errors.New("scheduler: agenda vazia — informe algo como \"every 30m\", \"daily at 9am\" ou \"0 9 * * *\"")

// ParseNL converte texto em linguagem natural (pt-BR/EN) em um Schedule.
// Exemplos:
//
//	"every 30m"        → interval 30m
//	"every 2h"         → interval 2h
//	"a cada 30 minutos" → interval 30m
//	"daily at 9am"     → daily 09:00
//	"diariamente às 9:00" → daily 09:00
//	"0 9 * * *"        → cron "0 9 * * *"
//
// Entrada inválida devolve erro claro em pt-BR.
func ParseNL(input string) (*Schedule, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil, errEmptySchedule
	}
	lower := normalizeText(raw)

	switch {
	case isCronString(lower):
		return parseCron(lower, raw)
	case hasDailyPrefix(lower):
		return parseDaily(lower, raw)
	case hasIntervalPrefix(lower) || isDurationLike(lower):
		return parseInterval(lower, raw)
	}
	return nil, fmt.Errorf(
		"scheduler: agenda %q não reconhecida — use \"every <duração>\" (ex.: every 30m), \"daily at <hora>\" / \"diariamente às <hora>\", ou um cron de 5 campos \"0 9 * * *\"",
		raw)
}

// normalizeText colapsa espaços e deixa minúsculo para casamento de prefixos.
func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// isDurationLike devolve true se o texto parece uma duração: uma duração Go
// simples ("30m", "2h", "5m30s") ou unidades por extenso ("2 hours", "1 day",
// "2 horas"). Permite agenda de intervalo sem o prefixo "every".
func isDurationLike(s string) bool {
	if d, err := time.ParseDuration(s); err == nil {
		return d > 0
	}
	return len(durRe.FindAllStringSubmatch(s, -1)) > 0
}

// hasIntervalPrefix verifica os prefixos EN/pt-BR de intervalo.
func hasIntervalPrefix(lower string) bool {
	for _, p := range []string{"every ", "a cada "} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// parseInterval parseia "every 30m", "a cada 2h", "every 5m30s" etc.
func parseInterval(lower, raw string) (*Schedule, error) {
	rest := lower
	for _, p := range []string{"every ", "a cada "} {
		if strings.HasPrefix(lower, p) {
			rest = strings.TrimSpace(strings.TrimPrefix(lower, p))
			break
		}
	}
	d, err := parseDurationHuman(rest)
	if err != nil {
		return nil, err
	}
	return &Schedule{Kind: KindInterval, Interval: d, Raw: raw}, nil
}

// parseDurationHuman interpreta uma duração: tenta time.ParseDuration e, se
// falhar, parseia unidades por extenso (EN/pt-BR: "30 minutes", "2 horas").
func parseDurationHuman(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		if d <= 0 {
			return 0, fmt.Errorf("scheduler: duração deve ser > 0 (ex.: 30m, 2h)")
		}
		return d, nil
	}

	var total time.Duration
	matched := false
	for _, m := range durRe.FindAllStringSubmatch(s, -1) {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		mult, ok := durUnitMult(m[2])
		if !ok {
			continue
		}
		total += time.Duration(v * float64(mult))
		matched = true
	}
	if !matched {
		return 0, fmt.Errorf("scheduler: duração %q não reconhecida (ex.: 30m, 2h, 5m30s, 1 day, 2 horas)", s)
	}
	if total <= 0 {
		return 0, fmt.Errorf("scheduler: duração deve ser > 0")
	}
	return total, nil
}

// durRe casa "5m30s", "30 minutes", "2 horas" etc.
var durRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*(ms|millisecond|milliseconds|milissegundo|milissegundos|sec|secs|second|seconds|segundo|segundos|s|min|mins|minute|minutes|minuto|minutos|m|hr|hour|hours|hora|horas|h|day|days|dia|dias|d)`)

// durUnitMult converte a unidade por extenso em nanosegundos.
func durUnitMult(u string) (time.Duration, bool) {
	switch u {
	case "ms", "millisecond", "milliseconds", "milissegundo", "milissegundos":
		return time.Millisecond, true
	case "s", "sec", "secs", "second", "seconds", "segundo", "segundos":
		return time.Second, true
	case "m", "min", "mins", "minute", "minutes", "minuto", "minutos":
		return time.Minute, true
	case "h", "hr", "hour", "hours", "hora", "horas":
		return time.Hour, true
	case "d", "day", "days", "dia", "dias":
		return 24 * time.Hour, true
	}
	return 0, false
}

// dailyPrefixes são os prefixos EN/pt-BR de agenda diária.
var dailyPrefixes = []string{
	"todos os dias", "todo dia", "todos dias",
	"every day", "everyday",
	"diariamente", "daily",
}

// hasDailyPrefix verifica se o texto começa com um prefixo diário.
func hasDailyPrefix(lower string) bool {
	for _, p := range dailyPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// dailyConnectors são as palavras que ligam o prefixo à hora ("at", "às").
var dailyConnectors = []string{"a las ", "às ", "as ", "at ", "@ "}

// parseDaily parseia "daily at 9am", "diariamente às 9:00", "every day at 08:30".
func parseDaily(lower, raw string) (*Schedule, error) {
	rest := lower
	for _, p := range dailyPrefixes {
		if strings.HasPrefix(lower, p) {
			rest = strings.TrimSpace(strings.TrimPrefix(lower, p))
			break
		}
	}
	for _, c := range dailyConnectors {
		if strings.HasPrefix(rest, c) {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, c))
			break
		}
	}
	h, m, ok := parseClock(rest)
	if !ok {
		return nil, fmt.Errorf(
			"scheduler: hora %q inválida na agenda diária %q (use ex.: 9am, 09:00, 17:45, 9h30)",
			rest, raw)
	}
	return &Schedule{Kind: KindDaily, Hour: h, Minute: m, Raw: raw}, nil
}

var (
	ampmRe  = regexp.MustCompile(`^(\d{1,2})(?::(\d{1,2}))?\s*(am|pm)$`)
	clockRe = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})$`)
	hourRe  = regexp.MustCompile(`^(\d{1,2})h(\d{2})?$`)
)

// parseClock interpreta uma hora em formato humano: "9am", "9pm", "08:30",
// "9:30", "17:45", "9h", "9h30". Devolve (hora, minuto, ok).
func parseClock(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	if m := ampmRe.FindStringSubmatch(s); m != nil {
		h, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, 0, false
		}
		min := 0
		if m[2] != "" {
			min, err = strconv.Atoi(m[2])
			if err != nil {
				return 0, 0, false
			}
		}
		if h < 0 || h > 12 || min < 0 || min > 59 {
			return 0, 0, false
		}
		if m[3] == "pm" && h != 12 {
			h += 12
		}
		if m[3] == "am" && h == 12 {
			h = 0
		}
		return h, min, true
	}
	if m := clockRe.FindStringSubmatch(s); m != nil {
		h, err1 := strconv.Atoi(m[1])
		min, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil || h < 0 || h > 23 || min < 0 || min > 59 {
			return 0, 0, false
		}
		return h, min, true
	}
	if m := hourRe.FindStringSubmatch(s); m != nil {
		h, err := strconv.Atoi(m[1])
		if err != nil || h < 0 || h > 23 {
			return 0, 0, false
		}
		min := 0
		if m[2] != "" {
			min, err = strconv.Atoi(m[2])
			if err != nil || min > 59 {
				return 0, 0, false
			}
		}
		return h, min, true
	}
	return 0, 0, false
}

// cronFieldChars limita o charset de um campo de cron (isCronString).
var cronFieldChars = regexp.MustCompile(`^[\d*/,\-]+$`)

// isCronString devolve true se o texto tem exatamente 5 campos e cada um usa
// apenas o charset cron (dígitos, *, /, vírgula, hífen).
func isCronString(s string) bool {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return false
	}
	for _, f := range fields {
		if !cronFieldChars.MatchString(f) {
			return false
		}
	}
	return true
}

// parseCron valida e normaliza a expressão cron de 5 campos.
func parseCron(lower, raw string) (*Schedule, error) {
	if _, err := parseCronExpr(strings.Join(strings.Fields(lower), " ")); err != nil {
		return nil, err
	}
	return &Schedule{Kind: KindCron, Cron: strings.Join(strings.Fields(lower), " "), Raw: raw}, nil
}

// ---------------------------------------------------------------------------
// NextAfter — próxima ocorrência
// ---------------------------------------------------------------------------

// NextAfter devolve a próxima ocorrência estritamente depois de t:
//
//	interval: t + Interval
//	daily:    a próxima hora:minuto (atravessa a meia-noite)
//	cron:     avalia os 5 campos minuto a minuto até 2 anos (MaxCronLookaheadMinutes)
func (s *Schedule) NextAfter(t time.Time) (time.Time, error) {
	switch s.Kind {
	case KindInterval:
		if s.Interval <= 0 {
			return time.Time{}, fmt.Errorf("scheduler: intervalo inválido %s", s.Interval)
		}
		return t.Add(s.Interval), nil
	case KindDaily:
		return nextDaily(s, t), nil
	case KindCron:
		return nextCron(s.Cron, t)
	default:
		return time.Time{}, fmt.Errorf("scheduler: kind %q inválido", s.Kind)
	}
}

// nextDaily devolve a próxima ocorrência da hora:minuto estritamente após t.
func nextDaily(s *Schedule, t time.Time) time.Time {
	candidate := time.Date(t.Year(), t.Month(), t.Day(), s.Hour, s.Minute, 0, 0, t.Location())
	if !candidate.After(t) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

// nextCron devolve a próxima ocorrência do cron estritamente após t, ou erro
// se não houver nenhuma dentro de MaxCronLookaheadMinutes (2 anos).
func nextCron(raw string, t time.Time) (time.Time, error) {
	expr, err := parseCronExpr(raw)
	if err != nil {
		return time.Time{}, err
	}
	cur := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location()).Add(time.Minute)
	for i := 0; i < MaxCronLookaheadMinutes; i++ {
		if expr.matches(cur) {
			return cur, nil
		}
		cur = cur.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf(
		"scheduler: cron %q não tem ocorrência nos próximos 2 anos (limite MaxCronLookaheadMinutes)", raw)
}

// ---------------------------------------------------------------------------
// Cron — matcher de 5 campos
// ---------------------------------------------------------------------------

// fieldItem é um componente compilado de um campo de cron.
type fieldItem struct {
	star   bool // "*" ou "*/N"
	step   int  // passo (1 para valores simples)
	single bool // valor único
	value  int  // para single
	from   int  // início do intervalo
	to     int  // fim do intervalo
}

// cronExpr é uma expressão cron compilada.
type cronExpr struct {
	minute, hour, dom, month, dow []fieldItem
}

// parseCronExpr compila os 5 campos validando os limites.
func parseCronExpr(raw string) (*cronExpr, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) != 5 {
		return nil, fmt.Errorf(
			"scheduler: cron %q precisa de 5 campos (minuto hora dia-do-mês mês dia-da-semana), ex.: \"0 9 * * *\"", raw)
	}
	minute, err := compileField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("scheduler: cron %q: minuto inválido: %w", raw, err)
	}
	hour, err := compileField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("scheduler: cron %q: hora inválida: %w", raw, err)
	}
	dom, err := compileField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("scheduler: cron %q: dia-do-mês inválido: %w", raw, err)
	}
	month, err := compileField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("scheduler: cron %q: mês inválido: %w", raw, err)
	}
	dow, err := compileField(parts[4], 0, 7)
	if err != nil {
		return nil, fmt.Errorf("scheduler: cron %q: dia-da-semana inválido: %w", raw, err)
	}
	return &cronExpr{minute: minute, hour: hour, dom: dom, month: month, dow: dow}, nil
}

// compileField compila um campo de cron (valor, *, */N, a-b, a-b/N, listas).
func compileField(s string, min, max int) ([]fieldItem, error) {
	if strings.TrimSpace(s) == "" {
		return nil, errors.New("campo vazio")
	}
	var items []fieldItem
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("campo vazio na lista")
		}
		step := 1
		base := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			base = part[:idx]
			n, err := strconv.Atoi(part[idx+1:])
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("passo inválido %q", part)
			}
			step = n
		}
		if base == "*" {
			items = append(items, fieldItem{star: true, step: step})
			continue
		}
		if idx := strings.Index(base, "-"); idx >= 0 {
			a, err1 := strconv.Atoi(base[:idx])
			b, err2 := strconv.Atoi(base[idx+1:])
			if err1 != nil || err2 != nil || a < min || b < min || a > max || b > max || a > b {
				return nil, fmt.Errorf("intervalo %q fora de %d-%d", base, min, max)
			}
			items = append(items, fieldItem{from: a, to: b, step: step})
			continue
		}
		v, err := strconv.Atoi(base)
		if err != nil || v < min || v > max {
			return nil, fmt.Errorf("valor %q fora de %d-%d", base, min, max)
		}
		items = append(items, fieldItem{single: true, value: v, step: step})
	}
	return items, nil
}

// matches avalia se um campo compilado casa o valor v.
func matchesField(field []fieldItem, v int) bool {
	for _, it := range field {
		switch {
		case it.star && it.step == 1:
			return true
		case it.star:
			if v%it.step == 0 {
				return true
			}
		case it.single:
			if v == it.value {
				return true
			}
		default: // intervalo a-b (com passo opcional)
			if v < it.from || v > it.to {
				continue
			}
			if it.step == 1 || (v-it.from)%it.step == 0 {
				return true
			}
		}
	}
	return false
}

// fieldIsStar devolve true quando o campo é apenas "*" (restrição vazia).
func fieldIsStar(field []fieldItem) bool {
	for _, it := range field {
		if !it.star || it.step != 1 {
			return false
		}
	}
	return true
}

// matches avalia se o cron casa o instante t (minuto exato).
func (e *cronExpr) matches(t time.Time) bool {
	if !matchesField(e.minute, t.Minute()) {
		return false
	}
	if !matchesField(e.hour, t.Hour()) {
		return false
	}
	domOK := matchesField(e.dom, t.Day())
	if !matchesField(e.month, int(t.Month())) {
		return false
	}
	// Dow: 0 (domingo) e 7 (domingo) são equivalentes.
	dowOK := matchesField(e.dow, int(t.Weekday())) || matchesField(e.dow, 7)
	domRestricted := !fieldIsStar(e.dom)
	dowRestricted := !fieldIsStar(e.dow)
	if domRestricted && dowRestricted {
		return domOK || dowOK
	}
	return domOK && dowOK
}

// ---------------------------------------------------------------------------
// Format
// ---------------------------------------------------------------------------

// Human devolve uma forma legível da agenda para tabelas. Prefere o texto
// original (Raw); sem ele, reconstrói a partir do Kind.
func (s *Schedule) Human() string {
	if strings.TrimSpace(s.Raw) != "" {
		return s.Raw
	}
	switch s.Kind {
	case KindInterval:
		return "every " + s.Interval.String()
	case KindDaily:
		return fmt.Sprintf("daily at %02d:%02d", s.Hour, s.Minute)
	case KindCron:
		return s.Cron
	}
	return s.Kind
}

// Validate valida a consistência do Schedule.
func (s *Schedule) Validate() error {
	switch s.Kind {
	case KindInterval:
		if s.Interval <= 0 {
			return fmt.Errorf("scheduler: intervalo deve ser > 0 (recebido %s)", s.Interval)
		}
		return nil
	case KindDaily:
		if s.Hour < 0 || s.Hour > 23 || s.Minute < 0 || s.Minute > 59 {
			return fmt.Errorf("scheduler: hora diária inválida %02d:%02d", s.Hour, s.Minute)
		}
		return nil
	case KindCron:
		if _, err := parseCronExpr(s.Cron); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("scheduler: kind %q inválido (esperava interval, daily ou cron)", s.Kind)
	}
}
