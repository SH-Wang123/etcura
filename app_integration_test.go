//go:build integration

package main

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"etcura/internal/etcd"
	"etcura/internal/model"
)

func TestIntegrationWriteSession(t *testing.T) {
	first, second := os.Getenv("ETCD_ENDPOINTS"), os.Getenv("ETCD_SECOND_ENDPOINT")
	if first == "" || second == "" || first == second {
		t.Fatal("two distinct disposable etcd endpoints are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	app := &App{ctx: ctx, mgr: etcd.NewManager()}
	defer app.shutdown(ctx)

	unused, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := unused.Addr().String()
	_ = unused.Close()

	connect := func(id string, endpoints []string, readOnly bool) model.StatusInfo {
		t.Helper()
		dial, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		profile := &model.ConnectionConfig{ID: id, Name: id, Endpoints: endpoints, ReadOnly: readOnly}
		status, err := app.mgr.Connect(dial, profile, "")
		if err != nil {
			t.Fatal(err)
		}
		return status
	}
	expectError := func(err error, code string) {
		t.Helper()
		var appErr *model.AppError
		if !errors.As(err, &appErr) || appErr.Code != code {
			t.Fatalf("got %v, want %s", err, code)
		}
	}

	a := connect("a", []string{dead, first}, false)
	client, _, err := app.mgr.Client(a.Session, false)
	if err != nil || client.Ctx().Err() != nil {
		t.Fatalf("client lifetime is tied to the probe: %v", err)
	}
	key := "/etcura-session-test/same-key"
	ad, err := app.CreateKey(a.Session, key, "YQ==", "")
	if err != nil {
		t.Fatal(err)
	}
	b := connect("b", []string{second}, false)
	bd, err := app.CreateKey(b.Session, key, "Yg==", "")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := app.SaveKey(a.Session, key, "YWJj", ad.ModRevision)
	if err != nil || updated.ValueBase64 != "YWJj" {
		t.Fatalf("independent first tab: %+v, %v", updated, err)
	}
	if len(app.ListSessions()) != 2 {
		t.Fatal("both sessions should still be open")
	}
	app.Disconnect(a.Session)
	_, err = app.SaveKey(a.Session, key, "YQ==", bd.ModRevision)
	expectError(err, model.CodeConnectionChanged)
	expectError(app.DeleteKey(a.Session, key, bd.ModRevision), model.CodeConnectionChanged)
	_, err = app.CreateKey(a.Session, key+"/new", "", "")
	expectError(err, model.CodeConnectionChanged)
	_, err = app.GetKey(a.Session, key)
	expectError(err, model.CodeConnectionChanged)
	d, err := app.GetKey(b.Session, key)
	if err != nil || d.ValueBase64 != bd.ValueBase64 || d.ModRevision != bd.ModRevision {
		t.Fatalf("unexpected record in second cluster: %+v, %v", d, err)
	}

	app.Disconnect(b.Session)
	locked := connect("b", []string{second}, true)
	_, err = app.GetKey(locked.Session, key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.CreateKey(locked.Session, key+"/new", "", "")
	expectError(err, model.CodeReadOnly)
	_, err = app.SaveKey(locked.Session, key, "YQ==", bd.ModRevision)
	expectError(err, model.CodeReadOnly)
	expectError(app.DeleteKey(locked.Session, key, bd.ModRevision), model.CodeReadOnly)
	_, err = app.GetKey(b.Session, key)
	expectError(err, model.CodeConnectionChanged)

	a = connect("a", []string{first}, false)
	d, err = app.GetKey(a.Session, key)
	if err != nil || d.ValueBase64 != updated.ValueBase64 {
		t.Fatalf("unexpected record in first cluster: %+v, %v", d, err)
	}
}
