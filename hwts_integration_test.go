package cclient

import (
	"io"
	"sync"
	"testing"
	"time"

	tls "github.com/refraction-networking/utls"
)

func TestHTTP2HardwareTimestampsArePerRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires network access")
	}

	client, err := NewClient(
		tls.HelloChrome_Auto,
		nil,   // no proxy
		true,  // allow redirects
		false, // skip TLS verification
		false, // do not force HTTP/1.1
		15*time.Second,
		"",   // no key log file
		true, // enable hardware/software RX timestamps
		nil,  // no connection callback
	)
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	type result struct {
		first time.Time
		last  time.Time
		err   error
	}

	const requests = 4
	results := make([]result, requests)

	var wg sync.WaitGroup
	wg.Add(requests)
	for i := 0; i < requests; i++ {
		go func(idx int) {
			defer wg.Done()
			resp, err := client.Get("https://api-manager.upbit.com/api/v1")
			if err != nil {
				results[idx].err = err
				return
			}
			defer resp.Body.Close()
			_, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				results[idx].err = readErr
				return
			}
			results[idx].first = resp.FirstHardwarePacketRX
			results[idx].last = resp.LastHardwarePacketRX
		}(i)
		if i < requests-1 {
			time.Sleep(3 * time.Millisecond)
		}
	}
	wg.Wait()

	seenFirst := make(map[time.Time]struct{})
	seenLast := make(map[time.Time]struct{})
	for idx, res := range results {
		if res.err != nil {
			t.Fatalf("request %d failed: %v", idx, res.err)
		}
		if res.first.IsZero() || res.last.IsZero() {
			t.Fatalf("request %d missing timestamps: first=%v last=%v", idx, res.first, res.last)
		}
		seenFirst[res.first] = struct{}{}
		seenLast[res.last] = struct{}{}
	}
	if len(seenFirst) != requests {
		t.Fatalf("expected %d unique first timestamps, got %d", requests, len(seenFirst))
	}
	if len(seenLast) != requests {
		t.Fatalf("expected %d unique last timestamps, got %d", requests, len(seenLast))
	}
}
