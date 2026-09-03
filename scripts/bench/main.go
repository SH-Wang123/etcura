// bench walks the etcd key space through the exact same pagination code path
// the app uses (etcd.ListPage, cursor based, keys-only) and reports Go-side
// heap usage so continuous paging can be checked for unbounded growth.
//
// Usage (after running scripts/seed):
//
//	go run ./scripts/bench -endpoints 127.0.0.1:12379 -pages 100
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"etcd-eye/internal/etcd"
	"etcd-eye/internal/model"
)

func main() {
	endpoints := flag.String("endpoints", "127.0.0.1:12379", "comma-separated etcd endpoints")
	prefix := flag.String("prefix", "/benchmark/", "prefix to walk")
	pages := flag.Int("pages", 100, "number of pages to walk")
	pageSize := flag.Int("page-size", 200, "keys per page (backend clamps to 1000)")
	rounds := flag.Int("rounds", 3, "walk repetitions")
	flag.Parse()

	cfg := model.ConnectionConfig{
		Name:             "bench",
		Endpoints:        []string{*endpoints},
		DialTimeoutMs:    5000,
		RequestTimeoutMs: 10000,
	}
	client, err := etcd.NewClient(context.Background(), &cfg, "")
	if err != nil {
		fatal("connect: %v", err)
	}
	defer client.Close()

	ctx := etcd.WithRequestTimeout(context.Background(), model.DefaultRequestTimeout)

	// Warm up dial.
	if _, err := etcd.ListPage(ctx, client, *prefix, "", 1); err != nil {
		fatal("probe: %v", err)
	}

	for round := 1; round <= *rounds; round++ {
		heapBefore := heapAlloc()
		var (
			totalKeys   int
			nextCursor  string
			start       = time.Now()
			seenCursors = map[string]bool{}
		)

		for p := 0; p < *pages; p++ {
			page, err := etcd.ListPage(ctx, client, *prefix, nextCursor, *pageSize)
			if err != nil {
				fatal("page %d: %v", p, err)
			}
			totalKeys += len(page.Items)
			// "" is a legitimate cursor (first page); any repeat is a loop.
			if seenCursors[page.Cursor] {
				fatal("cursor loop detected at page %d (cursor %q)", p, page.Cursor)
			}
			seenCursors[page.Cursor] = true
			if !page.HasMore {
				break
			}
			nextCursor = page.NextCursor
		}

		elapsed := time.Since(start)
		heapAfter := heapAlloc()

		fmt.Printf("round %d: walked %d pages, %d keys in %s (%.0f keys/s)\n",
			round, len(seenCursors), totalKeys, elapsed.Round(time.Millisecond),
			float64(totalKeys)/elapsed.Seconds())
		fmt.Printf("         heap before %s -> after %s (delta %+s), sys %s\n",
			fmtBytes(heapBefore), fmtBytes(heapAfter), fmtBytes(heapAfter-heapBefore), fmtBytes(heapSys()))

		if round < *rounds {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("memory is stable when the after-values stay in the same range across rounds")
}

func heapAlloc() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func heapSys() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
}

func fmtBytes(b uint64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.2f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.2f MB", float64(b)/(1024*1024))
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "bench: "+format+"\n", args...)
	os.Exit(1)
}
