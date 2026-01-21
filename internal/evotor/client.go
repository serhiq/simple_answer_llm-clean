package evotor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"simple_answer_llm/internal/config"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const (
	defaultBaseURL = "https://api.evotor.ru"
	apiMediaType   = "application/vnd.evotor.v2+json"
)

var (
	ErrMissingStoreID = errors.New("evotor store id is required")
	ErrMissingToken   = errors.New("evotor token is required")
	ErrUnauthorized   = errors.New("evotor unauthorized")
	ErrRateLimited    = errors.New("evotor rate limited")
	ErrEmptyQuery     = errors.New("search query is empty")
)

type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("evotor api error: %s", e.Status)
	}
	return fmt.Sprintf("evotor api error: %s: %s", e.Status, e.Body)
}

type Client struct {
	http           *resty.Client
	defaultStoreID string
	logger         *zap.Logger
}

func NewClient(cfg config.Config, logger *zap.Logger) *Client {
	httpClient := resty.New().
		SetBaseURL(defaultBaseURL).
		SetHeader("Accept", apiMediaType).
		SetHeader("Content-Type", apiMediaType).
		SetTimeout(cfg.Timeout).
		SetRetryCount(1).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(2 * time.Second).
		AddRetryCondition(func(resp *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			return resp != nil && resp.StatusCode() == http.StatusTooManyRequests
		})

	if cfg.EvotorToken != "" {
		httpClient.SetAuthScheme("Bearer")
		httpClient.SetAuthToken(cfg.EvotorToken)
	}

	return &Client{
		http:           httpClient,
		defaultStoreID: strings.TrimSpace(cfg.EvotorStoreID),
		logger:         logger.Named("evotor"),
	}
}

func (c *Client) ListStores(ctx context.Context) ([]Store, error) {
	if !c.hasToken() {
		return nil, ErrMissingToken
	}

	var stores []Store
	cursor := ""

	for {
		var resp listResponse[Store]
		query := map[string]string{}
		if cursor != "" {
			query["cursor"] = cursor
		}
		if err := c.doGet(ctx, "/stores", query, &resp); err != nil {
			return nil, err
		}
		stores = append(stores, resp.Items...)
		if resp.Paging.NextCursor == "" {
			break
		}
		cursor = resp.Paging.NextCursor
	}

	return stores, nil
}

func (c *Client) SearchItems(ctx context.Context, query string, limit int, storeID *string) ([]Item, error) {
	if !c.hasToken() {
		return nil, ErrMissingToken
	}
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}
	resolvedStoreID, err := c.resolveStoreID(storeID)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	fields := "id,name,price,code,barcodes,article_number,measure_name"
	var matches []Item

	cursor := ""
	for {
		var resp listResponse[Item]
		queryParams := map[string]string{
			"fields": fields,
		}
		if cursor != "" {
			queryParams["cursor"] = cursor
		}
		path := fmt.Sprintf("/stores/%s/products", resolvedStoreID)
		if err := c.doGet(ctx, path, queryParams, &resp); err != nil {
			return nil, err
		}

		for _, item := range resp.Items {
			if strings.Contains(strings.ToLower(item.Name), needle) {
				matches = append(matches, item)
				if limit > 0 && len(matches) >= limit {
					return matches[:limit], nil
				}
			}
		}

		if resp.Paging.NextCursor == "" {
			break
		}
		cursor = resp.Paging.NextCursor
	}

	return matches, nil
}

func (c *Client) SearchDocuments(ctx context.Context, from, to time.Time, storeID *string, limit, offset int) ([]DocumentShort, error) {
	if !c.hasToken() {
		return nil, ErrMissingToken
	}
	resolvedStoreID, err := c.resolveStoreID(storeID)
	if err != nil {
		return nil, err
	}

	cursor := ""
	var documents []DocumentShort
	seen := 0
	firstPage := true

	for {
		var resp listResponse[DocumentShort]
		queryParams := map[string]string{}
		if firstPage {
			if !from.IsZero() {
				queryParams["since"] = fmt.Sprintf("%d", from.UnixMilli())
			}
			if !to.IsZero() {
				queryParams["until"] = fmt.Sprintf("%d", to.UnixMilli())
			}
			firstPage = false
		}
		if cursor != "" {
			queryParams["cursor"] = cursor
		}

		path := fmt.Sprintf("/stores/%s/documents", resolvedStoreID)
		if err := c.doGet(ctx, path, queryParams, &resp); err != nil {
			return nil, err
		}

		for _, doc := range resp.Items {
			doc.Total = pickDocumentTotal(doc.Body)
			if seen < offset {
				seen++
				continue
			}
			documents = append(documents, doc)
			seen++
			if limit > 0 && len(documents) >= limit {
				return documents[:limit], nil
			}
		}

		if resp.Paging.NextCursor == "" {
			break
		}
		cursor = resp.Paging.NextCursor
	}

	return documents, nil
}

func (c *Client) GetDocument(ctx context.Context, docID string, storeID *string) (DocumentFull, error) {
	if !c.hasToken() {
		return DocumentFull{}, ErrMissingToken
	}
	if strings.TrimSpace(docID) == "" {
		return DocumentFull{}, errors.New("document id is required")
	}
	resolvedStoreID, err := c.resolveStoreID(storeID)
	if err != nil {
		return DocumentFull{}, err
	}

	var resp DocumentFull
	path := fmt.Sprintf("/stores/%s/documents/%s", resolvedStoreID, docID)
	if err := c.doGet(ctx, path, nil, &resp); err != nil {
		return DocumentFull{}, err
	}
	resp.Total = pickDocumentTotal(resp.Body)
	return resp, nil
}

func (c *Client) GetSalesMetrics(ctx context.Context, from, to time.Time, storeID *string, documentType *string) (SalesMetrics, error) {
	if !c.hasToken() {
		return SalesMetrics{}, ErrMissingToken
	}
	resolvedStoreID, err := c.resolveStoreID(storeID)
	if err != nil {
		return SalesMetrics{}, err
	}

	var allDocuments []DocumentShort
	cursor := ""
	firstPage := true

	for {
		var resp listResponse[DocumentShort]
		queryParams := map[string]string{}
		if firstPage {
			if !from.IsZero() {
				queryParams["since"] = fmt.Sprintf("%d", from.UnixMilli())
			}
			if !to.IsZero() {
				queryParams["until"] = fmt.Sprintf("%d", to.UnixMilli())
			}
			firstPage = false
		}
		if cursor != "" {
			queryParams["cursor"] = cursor
		}

		path := fmt.Sprintf("/stores/%s/documents", resolvedStoreID)
		if err := c.doGet(ctx, path, queryParams, &resp); err != nil {
			return SalesMetrics{}, err
		}

		allDocuments = append(allDocuments, resp.Items...)

		if resp.Paging.NextCursor == "" {
			break
		}
		cursor = resp.Paging.NextCursor
	}

	count := 0
	var totalSum float64
	docTypes := map[string]int{}

	for _, doc := range allDocuments {
		total := pickDocumentTotal(doc.Body)
		doc.Type = strings.TrimSpace(doc.Type)
		if doc.Type == "" {
			doc.Type = "UNKNOWN"
		}

		if documentType != nil {
			docTypeUpper := strings.ToUpper(*documentType)
			if docTypeUpper != "ALL" {
				if strings.ToUpper(doc.Type) != docTypeUpper {
					continue
				}
			}
		}

		count++
		totalSum += total
		docTypes[doc.Type]++
	}

	fromStr := ""
	toStr := ""
	if !from.IsZero() {
		fromStr = from.Format(time.RFC3339)
	}
	if !to.IsZero() {
		toStr = to.Format(time.RFC3339)
	}

	return SalesMetrics{
		Count:         count,
		TotalSum:      totalSum,
		StoreID:       resolvedStoreID,
		From:          fromStr,
		To:            toStr,
		DocumentTypes: docTypes,
	}, nil
}

func (c *Client) doGet(ctx context.Context, path string, query map[string]string, result any) error {
	req := c.http.R().SetContext(ctx).SetResult(result)
	if len(query) > 0 {
		req.SetQueryParams(query)
	}

	resp, err := req.Get(path)
	if err != nil {
		return fmt.Errorf("evotor request: %w", err)
	}
	if resp.IsError() {
		return apiErrorFromResponse(resp)
	}
	return nil
}

func (c *Client) resolveStoreID(storeID *string) (string, error) {
	if storeID != nil {
		if resolved := strings.TrimSpace(*storeID); resolved != "" {
			return resolved, nil
		}
	}
	if c.defaultStoreID == "" {
		return "", ErrMissingStoreID
	}
	return c.defaultStoreID, nil
}

func (c *Client) hasToken() bool {
	return strings.TrimSpace(c.http.Token) != ""
}

func apiErrorFromResponse(resp *resty.Response) error {
	body := strings.TrimSpace(resp.String())
	apiErr := &APIError{
		StatusCode: resp.StatusCode(),
		Status:     resp.Status(),
		Body:       body,
	}

	switch resp.StatusCode() {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrUnauthorized, apiErr.Error())
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Error())
	default:
		return apiErr
	}
}

func pickDocumentTotal(body DocumentBody) float64 {
	if body.Total != 0 {
		return body.Total
	}
	return body.Sum
}
