package cli

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"simple_answer_llm/internal/evotor"

	"go.uber.org/zap"
)

const (
	defaultDocLimit    = 50
	defaultOutputLimit = 10
	defaultPeriodDays  = 7
)

type response struct {
	Query          string
	AnswerText     string
	AppliedFilters appliedFilters
	Results        any
	ToolCalls      []toolCallRecord
	NextStep       string
}

type appliedFilters struct {
	DateFrom string `json:"date_from,omitempty"`
	DateTo   string `json:"date_to,omitempty"`
	StoreID  string `json:"store_id,omitempty"`
}

type toolCallRecord struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
	MS   int64          `json:"ms"`
	OK   bool           `json:"ok"`
	Err  string         `json:"err,omitempty"`
}

type resultItem struct {
	ID            string   `json:"item_id"`
	Name          string   `json:"name"`
	Price         float64  `json:"price,omitempty"`
	Code          string   `json:"code,omitempty"`
	Barcodes      []string `json:"barcodes,omitempty"`
	ArticleNumber string   `json:"article_number,omitempty"`
	MeasureName   string   `json:"measure_name,omitempty"`
}

type resultDocument struct {
	ID        string  `json:"doc_id"`
	Timestamp string  `json:"timestamp"`
	Total     float64 `json:"total"`
	StoreID   string  `json:"store_id,omitempty"`
	DeviceID  string  `json:"device_id,omitempty"`
}

type periodRange struct {
	From time.Time
	To   time.Time
}

type clarificationError struct {
	Message string
}

func (e clarificationError) Error() string {
	return e.Message
}

func trackCall[T any](logger *zap.Logger, name string, args map[string]any, fn func() (T, error)) (T, toolCallRecord, error) {
	start := time.Now()
	result, err := fn()
	elapsed := time.Since(start)
	record := toolCallRecord{
		Name: name,
		Args: args,
		MS:   elapsed.Milliseconds(),
		OK:   err == nil,
	}
	if err != nil {
		record.Err = err.Error()
	}
	logger.Info("tool call",
		zap.String("name", name),
		zap.Any("args", args),
		zap.Int64("ms", record.MS),
		zap.Bool("ok", record.OK),
		zap.String("err", record.Err),
	)
	return result, record, err
}

func friendlyEvotorError(err error) string {
	switch {
	case errors.Is(err, evotor.ErrMissingToken):
		return "Нет доступа: неверный или отсутствующий токен."
	case errors.Is(err, evotor.ErrMissingStoreID):
		return "Нужен store_id: укажите --store-id или EVOTOR_STORE_ID."
	case errors.Is(err, evotor.ErrUnauthorized):
		return "Нет доступа: неверный токен или недостаточно прав."
	case errors.Is(err, evotor.ErrRateLimited):
		return "Слишком много запросов. Попробуйте позже."
	default:
		if err == nil {
			return ""
		}
		return err.Error()
	}
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func documentHasItem(doc evotor.DocumentFull, needle string) bool {
	for _, pos := range doc.Body.Positions {
		name := strings.ToLower(strings.TrimSpace(pos.Name))
		productName := strings.ToLower(strings.TrimSpace(pos.ProductName))
		if (name != "" && strings.Contains(name, needle)) || (productName != "" && strings.Contains(productName, needle)) {
			return true
		}
	}
	return false
}

func extractItemQuery(query string) string {
	if quoted := extractQuoted(query); quoted != "" {
		return quoted
	}

	lower := strings.ToLower(query)
	for _, key := range []string{"позици", "товар"} {
		idx := strings.Index(lower, key)
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(query[idx+len(key):])
		if rest == "" {
			continue
		}
		return strings.Trim(rest, " .,:;!?")
	}

	return ""
}

func extractQuoted(query string) string {
	re := regexp.MustCompile(`[\"«""](.+?)[\"»"""]`)
	match := re.FindStringSubmatch(query)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func resolvePeriod(query string, opts *Options, interactive bool) (periodRange, string, error) {
	if opts.From != "" || opts.To != "" {
		return parsePeriodFromFlags(opts)
	}

	now := time.Now()
	lower := strings.ToLower(query)

	switch {
	case strings.Contains(lower, "вчера"):
		start := startOfDay(now.AddDate(0, 0, -1))
		end := endOfDay(now.AddDate(0, 0, -1))
		return periodRange{From: start, To: end}, "", nil
	case strings.Contains(lower, "сегодня"):
		start := startOfDay(now)
		return periodRange{From: start, To: now}, "", nil
	case strings.Contains(lower, "позавчера"):
		start := startOfDay(now.AddDate(0, 0, -2))
		end := endOfDay(now.AddDate(0, 0, -2))
		return periodRange{From: start, To: end}, "", nil
	case strings.Contains(lower, "недел"):
		to := now
		from := now.AddDate(0, 0, -defaultPeriodDays)
		return periodRange{From: from, To: to}, "", nil
	}

	month, hasMonth := detectMonth(lower)
	if hasMonth {
		year, hasYear := detectYear(lower)
		if !hasYear {
			year = now.Year()
			from := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
			to := endOfDay(from.AddDate(0, 1, -1))
			return periodRange{From: from, To: to}, fmt.Sprintf("Год не указан, использован %d.", year), nil
		}
		from := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
		to := endOfDay(from.AddDate(0, 1, -1))
		return periodRange{From: from, To: to}, "", nil
	}

	to := now
	from := now.AddDate(0, 0, -defaultPeriodDays)
	return periodRange{From: from, To: to}, "Период не указан, использованы последние 7 дней.", nil
}

func parsePeriodFromFlags(opts *Options) (periodRange, string, error) {
	var from time.Time
	var to time.Time
	var err error

	if opts.From != "" {
		from, err = parseDate(opts.From)
		if err != nil {
			return periodRange{}, "", fmt.Errorf("invalid --from date: %w", err)
		}
		from = startOfDay(from)
	}
	if opts.To != "" {
		to, err = parseDate(opts.To)
		if err != nil {
			return periodRange{}, "", fmt.Errorf("invalid --to date: %w", err)
		}
		to = endOfDay(to)
	}
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -defaultPeriodDays)
	}
	if to.IsZero() {
		to = time.Now()
	}
	if to.Before(from) {
		return periodRange{}, "", errors.New("--to must be after --from")
	}
	return periodRange{From: from, To: to}, "", nil
}

func parseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, time.Local)
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func detectMonth(lower string) (time.Month, bool) {
	months := map[string]time.Month{
		"январ":   time.January,
		"феврал":  time.February,
		"март":    time.March,
		"апрел":   time.April,
		"мая":     time.May,
		"май":     time.May,
		"июн":     time.June,
		"июл":     time.July,
		"август":  time.August,
		"сентябр": time.September,
		"октябр":  time.October,
		"ноябр":   time.November,
		"декабр":  time.December,
	}

	for key, month := range months {
		if strings.Contains(lower, key) {
			return month, true
		}
	}
	return 0, false
}

func detectYear(lower string) (int, bool) {
	re := regexp.MustCompile(`\b(20\d{2})\b`)
	match := re.FindStringSubmatch(lower)
	if len(match) < 2 {
		return 0, false
	}
	year, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return year, true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatTypeCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}
