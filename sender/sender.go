package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
)

type Sender struct {
	endpoint string
	in       <-chan metrics.Envelope
	client   *http.Client
	batch    []metrics.Envelope
	maxBatch int
	flush    time.Duration
}

func New(endpoint string, in <-chan metrics.Envelope) *Sender {
	return &Sender{
		endpoint: endpoint,
		in:       in,
		client:   &http.Client{Timeout: 10 * time.Second},
		batch:    make([]metrics.Envelope, 0, 50),
		maxBatch: 50,
		flush:    5 * time.Second,
	}
}

func (s *Sender) Run(ctx context.Context) {
	ticker := time.NewTicker(s.flush)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.sendBatch()
			return
		case m := <-s.in:
			s.batch = append(s.batch, m)
			if len(s.batch) >= s.maxBatch {
				s.sendBatch()
			}
		case <-ticker.C:
			if len(s.batch) > 0 {
				s.sendBatch()
			}
		}
	}
}

func (s *Sender) sendBatch() {
	if len(s.batch) == 0 {
		return
	}

	defer func() {
		s.batch = s.batch[:0]
	}()

	data, err := json.Marshal(s.batch)
	if err != nil {
		log.Printf("error marshalling json: %v", err)
		return
	}

	resp, err := s.client.Post(s.endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("error posting: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("server returned non-success status code %d for batch of %d metrics", resp.StatusCode, len(s.batch))
		return
	}
}
