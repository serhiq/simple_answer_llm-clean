package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

func hasFilters(filters appliedFilters) bool {
	return strings.TrimSpace(filters.DateFrom) != "" || strings.TrimSpace(filters.DateTo) != "" || strings.TrimSpace(filters.StoreID) != ""
}

func writeFilters(filters appliedFilters) {
	if strings.TrimSpace(filters.DateFrom) != "" || strings.TrimSpace(filters.DateTo) != "" {
		fmt.Fprintf(os.Stdout, "- период: %s — %s\n", formatDate(filters.DateFrom), formatDate(filters.DateTo))
	}
	if strings.TrimSpace(filters.StoreID) != "" {
		fmt.Fprintf(os.Stdout, "- store_id: %s\n", filters.StoreID)
	}
}

func writeResults(results any) {
	switch v := results.(type) {
	case []resultItem:
		if len(v) == 0 {
			fmt.Fprintln(os.Stdout, "- (нет результатов)")
			return
		}
		for i, item := range v {
			fmt.Fprintf(os.Stdout, "%d) %s (id=%s", i+1, item.Name, item.ID)
			if item.Price != 0 {
				fmt.Fprintf(os.Stdout, ", цена=%.2f", item.Price)
			}
			if item.ArticleNumber != "" {
				fmt.Fprintf(os.Stdout, ", артикул=%s", item.ArticleNumber)
			}
			if len(item.Barcodes) > 0 {
				fmt.Fprintf(os.Stdout, ", штрихкод=%s", item.Barcodes[0])
			}
			fmt.Fprintln(os.Stdout, ")")
		}
	case []resultDocument:
		if len(v) == 0 {
			fmt.Fprintln(os.Stdout, "- (нет результатов)")
			return
		}
		for i, doc := range v {
			fmt.Fprintf(os.Stdout, "%d) doc_id=%s, дата=%s, сумма=%.2f", i+1, doc.ID, doc.Timestamp, doc.Total)
			if doc.StoreID != "" {
				fmt.Fprintf(os.Stdout, ", store=%s", doc.StoreID)
			}
			if doc.DeviceID != "" {
				fmt.Fprintf(os.Stdout, ", device=%s", doc.DeviceID)
			}
			fmt.Fprintln(os.Stdout)
		}
	default:
		fmt.Fprintln(os.Stdout, "- (формат результатов не поддержан)")
	}
}

func logResponse(logger *zap.Logger, resp response) {
	if logger == nil {
		return
	}
	logger.Info("response",
		zap.String("query", strings.TrimSpace(resp.Query)),
		zap.String("answer", strings.TrimSpace(resp.AnswerText)),
		zap.Int("results_count", countResults(resp.Results)),
		zap.String("next_step", strings.TrimSpace(resp.NextStep)),
		zap.Any("filters", resp.AppliedFilters),
	)
}

func countResults(results any) int {
	switch v := results.(type) {
	case []resultItem:
		return len(v)
	case []resultDocument:
		return len(v)
	default:
		return 0
	}
}

func formatDate(value string) string {
	if value == "" {
		return "-"
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Format("2006-01-02")
	}
	return value
}
