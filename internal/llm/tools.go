package llm

import openrouter "github.com/revrost/go-openrouter"

func ToolSchemas() []openrouter.Tool {
	return []openrouter.Tool{
		getSalesMetricsTool(),
		listStoresTool(),
		searchItemsTool(),
		searchDocumentsTool(),
		getDocumentTool(),
	}
}

func getSalesMetricsTool() openrouter.Tool {
	return openrouter.Tool{
		Type: openrouter.ToolTypeFunction,
		Function: &openrouter.FunctionDefinition{
			Name:        "GetSalesMetrics",
			Description: "Get sales count and total sum for a period. Returns count, total_sum, store_id, period (from/to), and document_types with counts. Use this for 'how many receipts' or 'sum for period' queries. Much faster than SearchDocuments + aggregation. Default: counts only SELL documents (sales).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from": map[string]any{
						"type":        "string",
						"format":      "date-time",
						"description": "Start date in RFC3339 format (e.g., 2025-01-01T00:00:00Z). If not specified, use 7 days ago.",
					},
					"to": map[string]any{
						"type":        "string",
						"format":      "date-time",
						"description": "End date in RFC3339 format (e.g., 2025-01-31T23:59:59Z). If not specified, use now.",
					},
					"document_type": map[string]any{
						"type":        "string",
						"description": "Document type to count. Use 'SELL' for sales only (default). Use 'ALL' to include all types (SELL, RETURN, REFUND).",
					},
					"store_id": map[string]any{
						"type":        "string",
						"description": "Optional store ID. Use when the user selected a specific store; otherwise omit to use the default store.",
					},
				},
				"required":             []string{"from", "to"},
				"additionalProperties": false,
			},
		},
	}
}

func listStoresTool() openrouter.Tool {
	return openrouter.Tool{
		Type: openrouter.ToolTypeFunction,
		Function: &openrouter.FunctionDefinition{
			Name:        "ListStores",
			Description: "List all available stores for the current token. Returns stores with id and name. Use this to help user select which store to query if not specified.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
	}
}

func searchItemsTool() openrouter.Tool {
	return openrouter.Tool{
		Type: openrouter.ToolTypeFunction,
		Function: &openrouter.FunctionDefinition{
			Name:        "SearchItems",
			Description: "Find items by free-text query. Returns items with id, name, price, code, barcodes, article_number, measure_name. Search is case-insensitive and matches substrings in item names. Default limit: 10.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Text to search for in item names (case-insensitive substring match).",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of items to return (default: 10, max: 50).",
					},
					"store_id": map[string]any{
						"type":        "string",
						"description": "Optional store ID. Use when the user selected a specific store; otherwise omit to use the default store.",
					},
				},
				"required":             []string{"query"},
				"additionalProperties": false,
			},
		},
	}
}

func searchDocumentsTool() openrouter.Tool {
	return openrouter.Tool{
		Type: openrouter.ToolTypeFunction,
		Function: &openrouter.FunctionDefinition{
			Name:        "SearchDocuments",
			Description: "List documents for a period and store. Returns documents with id, timestamp, total, type, store_id, device_id. Types include SELL (sale), RETURN (return), REFUND (refund). Use item_query to filter documents that contain a specific item name in positions (this will fetch full documents and check positions locally). Default limit: 50.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from": map[string]any{
						"type":        "string",
						"format":      "date-time",
						"description": "Start date in RFC3339 format (e.g., 2025-01-01T00:00:00Z). If not specified, use 7 days ago.",
					},
					"to": map[string]any{
						"type":        "string",
						"format":      "date-time",
						"description": "End date in RFC3339 format (e.g., 2025-01-31T23:59:59Z). If not specified, use now.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of documents to return (default: 50, max: 200).",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of documents to skip (for pagination).",
					},
					"item_query": map[string]any{
						"type":        "string",
						"description": "Optional text to search for in document positions. If provided, fetches full documents and filters locally to find items matching this query (case-insensitive).",
					},
					"store_id": map[string]any{
						"type":        "string",
						"description": "Optional store ID. Use when the user selected a specific store; otherwise omit to use the default store.",
					},
				},
				"required":             []string{"from", "to"},
				"additionalProperties": false,
			},
		},
	}
}

func getDocumentTool() openrouter.Tool {
	return openrouter.Tool{
		Type: openrouter.ToolTypeFunction,
		Function: &openrouter.FunctionDefinition{
			Name:        "GetDocument",
			Description: "Fetch a single document with all positions. Returns document id, type (SELL/RETURN/REFUND), close_date, total, store_id, device_id, and positions with product_id, name, quantity, price, sum. Use for detailed inspection of specific documents.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"doc_id": map[string]any{
						"type":        "string",
						"description": "Document ID to fetch.",
					},
					"store_id": map[string]any{
						"type":        "string",
						"description": "Optional store ID. Use when the user selected a specific store; otherwise omit to use the default store.",
					},
				},
				"required":             []string{"doc_id"},
				"additionalProperties": false,
			},
		},
	}
}
