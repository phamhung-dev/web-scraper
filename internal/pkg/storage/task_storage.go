package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
	"web-scraper/internal/pkg/common"
	"web-scraper/internal/pkg/models"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type taskStorage struct {
	elastic *elasticSearch
	timeout time.Duration
}

func NewTaskStorage(elastic *elasticSearch) *taskStorage {
	return &taskStorage{
		elastic: elastic,
		timeout: time.Second * 10,
	}
}

func (t *taskStorage) Insert(ctx context.Context, task *models.Task) error {
	body, err := json.Marshal(*task)
	if err != nil {
		return fmt.Errorf("insert: marshall: %w", err)
	}

	req := esapi.CreateRequest{
		Index:      t.elastic.alias,
		DocumentID: task.ID.String(),
		Body:       bytes.NewReader(body),
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	res, err := req.Do(ctx, t.elastic.client)
	if err != nil {
		return fmt.Errorf("insert: request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 409 {
		return fmt.Errorf("insert: request: %w", common.ErrConflict)
	}
	if res.IsError() {
		return fmt.Errorf("insert: response: %s", res.String())
	}

	return nil
}

func (t *taskStorage) BulkInsert(ctx context.Context, histories []models.Task) error {
	body, err := json.Marshal(histories)
	if err != nil {
		return fmt.Errorf("bulk insert: marshall: %w", err)
	}

	req := esapi.BulkRequest{
		Index: t.elastic.alias,
		Body:  bytes.NewReader(body),
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	res, err := req.Do(ctx, t.elastic.client)
	if err != nil {
		return fmt.Errorf("bulk insert: request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 409 {
		return fmt.Errorf("bulk insert: request: %w", common.ErrConflict)
	}
	if res.IsError() {
		return fmt.Errorf("bulk insert: response: %s", res.String())
	}

	return nil
}
