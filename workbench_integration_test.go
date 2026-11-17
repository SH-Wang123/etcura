//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"etcura/internal/etcd"
	"etcura/internal/model"
	"etcura/internal/workbench"
)

func TestIntegrationWorkbench(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	manager := etcd.NewManager()
	app := &App{ctx: ctx, mgr: manager, workbench: workbench.New(manager)}
	defer app.shutdown(ctx)

	left, source := connectFixture(t, ctx, manager, "development", os.Getenv("ETCD_ENDPOINTS"), false)
	right, target := connectFixture(t, ctx, manager, "staging", os.Getenv("ETCD_SECOND_ENDPOINT"), false)
	sourcePrefix, targetPrefix := "/milestone/dev/", "/milestone/stage/"
	binarySuffix, binaryValue := "\x00\xff\x01", []byte{0xff, 0x00, 0x01}
	put := func(client *clientv3.Client, key, value string, opts ...clientv3.OpOption) {
		t.Helper()
		if _, err := client.Put(ctx, key, value, opts...); err != nil {
			t.Fatal(err)
		}
	}

	lease, err := target.Grant(ctx, 300)
	if err != nil {
		t.Fatal(err)
	}
	sourceLease, err := source.Grant(ctx, 300)
	if err != nil {
		t.Fatal(err)
	}

	put(source, sourcePrefix+binarySuffix, string(binaryValue))
	put(source, sourcePrefix+"change", "new")
	put(source, sourcePrefix+"conflict", "new")
	put(source, sourcePrefix+"lease", "temporary", clientv3.WithLease(sourceLease.ID))
	put(target, targetPrefix+"change", "old", clientv3.WithLease(lease.ID))
	put(target, targetPrefix+"conflict", "old")
	put(target, targetPrefix+"retained", "target-only")

	var workload = 170
	if input := os.Getenv("ETCURA_SCALE"); input != "" {
		workload, err = strconv.Atoi(input)
		if err != nil || workload < 1 {
			t.Fatalf("invalid ETCURA_SCALE: %q", input)
		}
	}
	for start := 0; start < workload; start += 100 {
		leftOps, rightOps := make([]clientv3.Op, 0, 100), make([]clientv3.Op, 0, 100)
		for i := start; i < min(start+100, workload); i++ {
			suffix := fmt.Sprintf("config/%06d", i)
			leftOps = append(leftOps, clientv3.OpPut(sourcePrefix+suffix, "equal"))
			rightOps = append(rightOps, clientv3.OpPut(targetPrefix+suffix, "equal"))
		}
		if _, err := source.Txn(ctx).Then(leftOps...).Commit(); err != nil {
			t.Fatal(err)
		}
		if _, err := target.Txn(ctx).Then(rightOps...).Commit(); err != nil {
			t.Fatal(err)
		}
	}

	watch, err := app.StartWatch(model.WatchStart{Session: right.Session, Key: targetPrefix, Prefix: true})
	if err != nil {
		t.Fatal(err)
	}
	defer app.StopWatch(watch)
	req := model.CompareRequest{SourceSession: left.Session, TargetSession: right.Session, SourcePrefix: sourcePrefix, TargetPrefix: targetPrefix}
	info, err := app.StartCompare(req)
	if err != nil {
		t.Fatal(err)
	}

	// A source write after the snapshot must not alter the viewed value or plan.
	put(source, sourcePrefix+"change", "newer than snapshot")
	diff := make(map[string]model.CompareRow)
	count := 0
	from := time.Now()
	for cursor := ""; ; {
		page, err := app.ComparePage(info.ID, cursor, "different")
		if err != nil {
			t.Fatal(err)
		}
		count += page.Scanned
		for _, row := range page.Rows {
			if _, exists := diff[row.Key]; exists {
				t.Fatalf("duplicate row: %q", row.Label)
			}
			diff[row.Key] = row
		}
		if page.NextCursor == "" {
			break
		}
		if cursor == page.NextCursor {
			t.Fatal("pagination did not advance")
		}
		cursor = page.NextCursor
	}
	t.Logf("compared %d suffixes across two clusters in %s", count, time.Since(from))
	if count != workload+5 || len(diff) != 5 {
		t.Fatalf("compare count=%d, diff=%+v", count, diff)
	}
	encode := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	binaryKey := encode(binarySuffix)
	view, err := app.CompareDetail(info.ID, binaryKey)
	if err != nil || view.Source == nil || view.Source.Base64 != encode(string(binaryValue)) || view.Target != nil {
		t.Fatalf("binary preview: %+v, %v", view, err)
	}
	plan, err := app.PreviewSync(info.ID, []string{binaryKey, encode("change"), encode("conflict"), encode("lease")})
	if err != nil {
		t.Fatal(err)
	}
	put(target, targetPrefix+"conflict", "other writer")
	results, err := app.ApplySync(plan.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"success", "success", "conflict", "skipped"}
	for i, result := range results {
		if result.State != want[i] {
			t.Fatalf("result %d = %+v, want %s", i, result, want[i])
		}
	}
	if _, err := app.ApplySync(plan.ID, nil); err == nil {
		t.Fatal("reused confirmation token")
	}

	got, err := target.Get(ctx, targetPrefix+binarySuffix)
	if err != nil || len(got.Kvs) != 1 || !bytes.Equal(got.Kvs[0].Value, binaryValue) {
		t.Fatalf("binary after sync: %v, %v", got, err)
	}
	change, err := target.Get(ctx, targetPrefix+"change")
	if err != nil || len(change.Kvs) != 1 || string(change.Kvs[0].Value) != "new" || change.Kvs[0].Lease != int64(lease.ID) {
		t.Fatalf("changed value or lease: %v, %v", change, err)
	}
	for key, want := range map[string]string{"conflict": "other writer", "retained": "target-only", "lease": ""} {
		got, err := target.Get(ctx, targetPrefix+key)
		if err != nil || want == "" && len(got.Kvs) != 0 || want != "" && (len(got.Kvs) != 1 || string(got.Kvs[0].Value) != want) {
			t.Fatalf("%s: %v, %v", key, got, err)
		}
	}
	waitWatch(t, app, watch, func(event model.WatchEvent) bool {
		return event.Key == targetPrefix+"change" && event.Before != nil && event.Before.Base64 == encode("old") &&
			event.After != nil && event.After.Base64 == encode("new")
	})

	// Repeat comparison after applying; binary now matches and new source writes are visible.
	info, err = app.StartCompare(req)
	if err != nil {
		t.Fatal(err)
	}
	view, err = app.CompareDetail(info.ID, binaryKey)
	if err != nil || view.Row.State != "same" {
		t.Fatalf("recompare binary: %+v, %v", view, err)
	}
	app.CancelCompare(info.ID)
	if _, err = app.ComparePage(info.ID, "", "all"); err == nil {
		t.Fatal("cancelled comparison still readable")
	}

	readOnly, _ := connectFixture(t, ctx, manager, "readonly", os.Getenv("ETCD_SECOND_ENDPOINT"), true)
	req.TargetSession = readOnly.Session
	info, err = app.StartCompare(req)
	if err != nil || !info.TargetReadOnly {
		t.Fatalf("read-only comparison: %+v, %v", info, err)
	}
	if _, err := app.PreviewSync(info.ID, []string{encode("change")}); err == nil {
		t.Fatal("read-only target offered sync")
	}
}

func connectFixture(t *testing.T, ctx context.Context, manager *etcd.Manager, name, endpoint string, readOnly bool) (model.StatusInfo, *clientv3.Client) {
	t.Helper()
	if endpoint == "" {
		t.Fatal("disposable etcd endpoint missing")
	}
	conn, err := manager.Connect(ctx, &model.ConnectionConfig{ID: name, Name: name, Endpoints: []string{endpoint}, ReadOnly: readOnly}, "")
	if err != nil {
		t.Fatal(err)
	}
	client, _, err := manager.Client(conn.Session, false)
	if err != nil {
		t.Fatal(err)
	}
	return conn, client
}

func waitWatch(t *testing.T, app *App, id string, match func(model.WatchEvent) bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		batch, err := app.PollWatch(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range batch.Events {
			if match(event) {
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("expected change not observed")
}

func TestIntegrationWatchRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	manager := etcd.NewManager()
	app := &App{ctx: ctx, mgr: manager, workbench: workbench.New(manager)}
	defer app.shutdown(ctx)
	endpoint := os.Getenv("ETCD_SECOND_ENDPOINT")
	_, writer := connectFixture(t, ctx, manager, "direct", endpoint, false)
	upstream, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	proxy := startTestProxy(t, upstream.Host)
	conn, _ := connectFixture(t, ctx, manager, "watch proxy", proxy.listener.Addr().String(), false)
	key := "/milestone/recovery/key"
	id, err := app.StartWatch(model.WatchStart{Session: conn.Session, Key: key})
	if err != nil {
		t.Fatal(err)
	}
	defer app.StopWatch(id)
	write := func(value string) *clientv3.PutResponse {
		t.Helper()
		resp, err := writer.Put(ctx, key, value)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	after := func(value string) func(model.WatchEvent) bool {
		return func(event model.WatchEvent) bool {
			return event.Key == key && event.After != nil && event.After.Base64 == base64.StdEncoding.EncodeToString([]byte(value))
		}
	}
	write("initial")
	waitWatch(t, app, id, after("initial"))

	proxy.enable(false)
	write("while disconnected")
	proxy.enable(true)
	waitWatch(t, app, id, after("while disconnected"))

	proxy.enable(false)
	write("lost")
	compaction := write("compacted").Header.Revision
	if _, err := writer.Compact(ctx, compaction); err != nil {
		t.Fatal(err)
	}
	proxy.enable(true)
	waitWatch(t, app, id, func(event model.WatchEvent) bool { return event.Type == "gap" })
	write("resumed")
	waitWatch(t, app, id, after("resumed"))
}

type testProxy struct {
	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]net.Conn
	enabled  bool
}

func startTestProxy(t *testing.T, upstream string) *testProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &testProxy{listener: listener, conns: map[net.Conn]net.Conn{}, enabled: true}
	t.Cleanup(func() { _ = listener.Close(); p.enable(false) })
	go func() {
		for {
			from, err := listener.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			if !p.enabled {
				p.mu.Unlock()
				_ = from.Close()
				continue
			}
			to, err := net.DialTimeout("tcp", upstream, time.Second)
			if err != nil {
				p.mu.Unlock()
				_ = from.Close()
				continue
			}
			p.conns[from] = to
			p.mu.Unlock()
			go func() {
				go func() { _, _ = io.Copy(to, from); _ = to.Close(); _ = from.Close() }()
				_, _ = io.Copy(from, to)
				_ = from.Close()
				_ = to.Close()
				p.mu.Lock()
				delete(p.conns, from)
				p.mu.Unlock()
			}()
		}
	}()
	return p
}

func (p *testProxy) enable(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = enabled
	if !enabled {
		for from, to := range p.conns {
			_ = from.Close()
			_ = to.Close()
		}
	}
}
