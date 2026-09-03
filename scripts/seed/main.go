// seed writes benchmark data into a local etcd so pagination and memory
// behaviour can be verified with realistic volume.
//
// Usage:
//
//	go run ./scripts/seed -endpoints 127.0.0.1:12379 -count 100000
//
// Keys look like /benchmark/00000001 ... /benchmark/10000000 and values are
// random with a mixed size distribution: mostly tens of bytes, many hundreds,
// a few kilobytes.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

func main() {
	endpoints := flag.String("endpoints", "127.0.0.1:12379", "comma-separated etcd endpoints")
	prefix := flag.String("prefix", "/benchmark/", "key prefix")
	count := flag.Int("count", 100000, "number of keys to write")
	concurrency := flag.Int("concurrency", 16, "parallel writers")
	batchSize := flag.Int("batch", 120, "keys per txn batch (etcd default max txn ops is 128)")
	clean := flag.Bool("clean", false, "delete the prefix before seeding")
	flag.Parse()

	if !strings.HasSuffix(*prefix, "/") {
		*prefix += "/"
	}

	cfg := clientv3.Config{
		Endpoints:   strings.Split(*endpoints, ","),
		DialTimeout: 5 * time.Second,
		Logger:      zap.NewNop(),
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		fatal("cannot create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Wait for the server.
	for {
		sctx, scancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := client.Status(sctx, client.Endpoints()[0])
		scancel()
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			fatal("etcd unreachable: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("connected to %s\n", *endpoints)

	if *clean {
		dctx, dcancel := context.WithTimeout(ctx, 2*time.Minute)
		_, err := client.Delete(dctx, *prefix, clientv3.WithPrefix())
		dcancel()
		if err != nil {
			fatal("clean: %v", err)
		}
		fmt.Printf("cleaned %s*\n", *prefix)
	}

	start := time.Now()
	var written atomic.Int64
	var next atomic.Int64

	workers := *concurrency
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(w)))
			for {
				first := next.Add(int64(*batchSize)) - int64(*batchSize)
				if first >= int64(*count) {
					return
				}
				ops := make([]clientv3.Op, 0, *batchSize)
				for i := first; i < first+int64(*batchSize) && i < int64(*count); i++ {
					ops = append(ops, clientv3.OpPut(
						fmt.Sprintf("%s%08d", *prefix, i+1),
						randomValue(rng),
					))
				}
				tctx, tcancel := context.WithTimeout(ctx, 30*time.Second)
				_, err := client.Txn(tctx).Then(ops...).Commit()
				tcancel()
				if err != nil {
					fatal("batch write failed: %v", err)
				}
				n := written.Add(int64(len(ops)))
				if n%20000 < int64(*batchSize) {
					elapsed := time.Since(start).Seconds()
					fmt.Fprintf(os.Stderr, "\r%d keys (%.0f keys/s)", n, float64(n)/elapsed)
				}
			}
		}(w)
	}
	wg.Wait()

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Printf("done: %d keys under %s in %s\n", *count, *prefix, time.Since(start).Round(time.Millisecond))
}

// randomValue: 60% tiny (30-120 B), 35% small (200-900 B), 5% larger (2-8 KB).
func randomValue(rng *rand.Rand) string {
	var size int
	switch r := rng.Intn(100); {
	case r < 60:
		size = 30 + rng.Intn(90)
	case r < 95:
		size = 200 + rng.Intn(700)
	default:
		size = 2048 + rng.Intn(6144)
	}
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "seed: "+format+"\n", args...)
	os.Exit(1)
}
