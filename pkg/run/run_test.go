package run

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/trace"
	"gopkg.in/yaml.v3"
)

func TestRun(t *testing.T) {
	old := cfg.DefaultReader()
	defer cfg.SetDefaultReader(old)

	traceConfig := trace.Config{
		Otel: trace.Otel{
			Enabled: true,
		},
	}

	cfg.SetDefaultReader(func(fileName string) ([]byte, error) {
		switch fileName {
		case "trace.yaml":
			return yaml.Marshal(traceConfig)
		default:
			return nil, os.ErrNotExist
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	ch := make(chan struct{})
	go func() {
		t.Log("starting service")
		err := Run(ctx, "test-service", OptHTTPAddr(addr), OptAddRunner("ready-signal", async.Func(func(ctx context.Context) error {
			async.Sleep(ctx, time.Millisecond)
			close(ch)
			<-ctx.Done()
			return nil
		})))
		if err != nil {
			close(ch)
			t.Error(err)
		}
	}()
	<-ch
	t.Log("fetching healthcheck")
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+addr+"/healthz/live", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok got %s", resp.Status)
	}
}
