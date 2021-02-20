package stdout

import (
	"context"
	"fmt"

	"github.com/kentik/ktranslate/pkg/eggs/logger"
	go_metrics "github.com/kentik/go-metrics"
	"github.com/kentik/ktranslate/pkg/formats"
	"github.com/kentik/ktranslate/pkg/kt"
)

type StdoutSink struct {
	logger.ContextL
}

func NewSink(log logger.Underlying, registry go_metrics.Registry) (*StdoutSink, error) {
	return &StdoutSink{
		ContextL: logger.NewContextLFromUnderlying(logger.SContext{S: "stdoutSink"}, log),
	}, nil
}

func (s *StdoutSink) Init(ctx context.Context, format formats.Format, compression kt.Compression) error {
	return nil
}

func (s *StdoutSink) Send(ctx context.Context, payload []byte) {
	fmt.Printf("%s\n", string(payload))
}

func (s *StdoutSink) Close() {}

func (s *StdoutSink) HttpInfo() map[string]float64 {
	return map[string]float64{}
}