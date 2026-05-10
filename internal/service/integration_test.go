package service

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
)

func TestE2E_LogsLandInEvents(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	srv := receiver.NewServer(receiver.Config{
		Addr:    "127.0.0.1:0",
		Logs:    svc,
		Metrics: svc,
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := srv.Addr()
	go func() { _ = srv.Serve() }()
	t.Cleanup(srv.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		addr,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "tcp", addr)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	cli := collogspb.NewLogsServiceClient(conn)
	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvStr("project.name", "demo")}},
			ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
				{TimeUnixNano: 100, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"), kvStr("session.id", "S"), kvStr("prompt.id", "P"),
				}},
				{TimeUnixNano: 200, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.api_request"), kvStr("session.id", "S"), kvStr("prompt.id", "P"),
				}},
			}}},
		}},
	}
	if _, err := cli.Export(ctx, req); err != nil {
		t.Fatalf("Export: %v", err)
	}

	rows, err := repo.DB().QueryContext(ctx, "SELECT event_name, session_id, prompt_id, attrs FROM events ORDER BY ts")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name, sess, prompt, attrs string
		if err := rows.Scan(&name, &sess, &prompt, &attrs); err != nil {
			t.Fatal(err)
		}
		if sess != "S" || prompt != "P" {
			t.Fatalf("identity wrong: %s/%s", sess, prompt)
		}
		got = append(got, name)
	}
	if len(got) != 2 || got[0] != "user_prompt" || got[1] != "api_request" {
		t.Fatalf("got %v", got)
	}

	var prompts int64
	if err := repo.DB().QueryRow(`SELECT prompts FROM sessions WHERE session_id = ?`, "S").Scan(&prompts); err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if prompts == 0 {
		t.Errorf("expected sessions.prompts > 0 after ingest")
	}
}
