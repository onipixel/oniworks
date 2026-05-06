// OniWorks Stress Test
//
// Tests the HTTP router, Oni Memory, PostgreSQL query builder, and WebSocket hub
// under concurrent load and reports throughput, latency percentiles, and errors.
//
// Usage:
//
//	go run ./testing/stress \
//	  --db "postgres://postgres:password@localhost:5432/oniworks_stress?sslmode=disable" \
//	  --workers 50 --duration 10s
package main

import (
	"context"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/onipixel/oniworks/framework/database"
	onihttp "github.com/onipixel/oniworks/framework/http"
	"github.com/onipixel/oniworks/framework/memory"
	"github.com/onipixel/oniworks/framework/routing"
)

// ─── flags ───────────────────────────────────────────────────────────────────

var (
	flagDSN      = flag.String("db", "postgres://postgres:password@localhost:5432/postgres?sslmode=disable", "PostgreSQL DSN")
	flagWorkers  = flag.Int("workers", 50, "Concurrent goroutines per test")
	flagDuration = flag.Duration("duration", 8*time.Second, "Test duration per component")
	flagSkipDB   = flag.Bool("skip-db", false, "Skip database tests")
	flagSkipWS   = flag.Bool("skip-ws", false, "Skip WebSocket tests")
)

// ─── result ──────────────────────────────────────────────────────────────────

type result struct {
	ops      int64
	errors   int64
	latencies []int64 // microseconds
	mu       sync.Mutex
}

func (r *result) record(lat time.Duration, err bool) {
	if err {
		atomic.AddInt64(&r.errors, 1)
		return
	}
	atomic.AddInt64(&r.ops, 1)
	r.mu.Lock()
	r.latencies = append(r.latencies, lat.Microseconds())
	r.mu.Unlock()
}

func (r *result) report(label string, duration time.Duration) {
	ops := atomic.LoadInt64(&r.ops)
	errs := atomic.LoadInt64(&r.errors)
	rps := float64(ops) / duration.Seconds()

	r.mu.Lock()
	lats := make([]int64, len(r.latencies))
	copy(lats, r.latencies)
	r.mu.Unlock()

	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

	p50, p95, p99, avg := latencyStats(lats)

	errRate := 0.0
	if ops+errs > 0 {
		errRate = float64(errs) / float64(ops+errs) * 100
	}

	color := green
	if errRate > 1 {
		color = red
	} else if errRate > 0 {
		color = yellow
	}

	fmt.Printf("  %-38s %s%8.0f ops/s%s  p50:%5.1fms  p95:%5.1fms  p99:%5.1fms  avg:%5.1fms  err:%.1f%%\n",
		label, color,
		rps, reset,
		float64(p50)/1000,
		float64(p95)/1000,
		float64(p99)/1000,
		float64(avg)/1000,
		errRate,
	)
}

func latencyStats(sorted []int64) (p50, p95, p99, avg int64) {
	if len(sorted) == 0 {
		return
	}
	p50 = sorted[int(math.Round(float64(len(sorted))*0.50))-1]
	p95 = sorted[int(math.Round(float64(len(sorted))*0.95))-1]
	p99 = sorted[int(math.Round(float64(len(sorted))*0.99))-1]
	var sum int64
	for _, v := range sorted {
		sum += v
	}
	avg = sum / int64(len(sorted))
	return
}

// ─── terminal colors ──────────────────────────────────────────────────────────

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

// ─── run helper ───────────────────────────────────────────────────────────────

// run fires `workers` goroutines, each calling `fn` in a tight loop for `dur`.
// Returns a populated result.
func run(workers int, dur time.Duration, fn func() (time.Duration, error)) *result {
	res := &result{latencies: make([]int64, 0, workers*500)}
	var wg sync.WaitGroup
	deadline := time.Now().Add(dur)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				lat, err := fn()
				res.record(lat, err != nil)
			}
		}()
	}
	wg.Wait()
	return res
}

// ─── main ────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	fmt.Printf("\n%s%s OniWorks Stress Test%s\n", bold, cyan, reset)
	fmt.Printf("%s%s workers / %s per test / Go %s%s\n\n",
		gray, fmt.Sprintf("%d", *flagWorkers), *flagDuration, runtime.Version(), reset)

	runHTTP()
	runMemory()
	if !*flagSkipDB {
		runDatabase()
	}
	if !*flagSkipWS {
		runWebSocket()
	}

	fmt.Printf("\n%s✓ Done%s\n\n", green, reset)
}

// ─── 1. HTTP Router ───────────────────────────────────────────────────────────

func runHTTP() {
	fmt.Printf("%s── HTTP Router%s\n", bold, reset)

	r := routing.New()
	r.Get("/ping", func(c *onihttp.Context) error {
		return c.JSON(200, onihttp.Map{"ok": true})
	})
	r.Get("/users/:id/posts/:postID", func(c *onihttp.Context) error {
		return c.JSON(200, onihttp.Map{
			"user": c.Param("id"),
			"post": c.Param("postID"),
		})
	})
	r.Post("/echo", func(c *onihttp.Context) error {
		var body map[string]any
		_ = c.Bind(&body)
		return c.JSON(200, body)
	})

	// In-process testing via ServeHTTP — measures the router itself with zero TCP overhead.
	// httptest.ResponseRecorder instances are pooled to avoid allocation noise.
	recPool := sync.Pool{New: func() any { return httptest.NewRecorder() }}

	inproc := func(method, path string, body []byte, ct string) func() (time.Duration, error) {
		return func() (time.Duration, error) {
			var br *bytes.Reader
			if body != nil {
				br = bytes.NewReader(body)
			}
			var req *http.Request
			if br != nil {
				req = httptest.NewRequest(method, path, br)
				req.Header.Set("Content-Type", ct)
			} else {
				req = httptest.NewRequest(method, path, nil)
			}
			w := recPool.Get().(*httptest.ResponseRecorder)
			w.Body.Reset()
			w.HeaderMap = make(http.Header)
			w.Code = 200

			t := time.Now()
			r.ServeHTTP(w, req)
			lat := time.Since(t)

			code := w.Code
			recPool.Put(w)
			if code/100 == 5 {
				return 0, fmt.Errorf("status %d", code)
			}
			return lat, nil
		}
	}

	// Simple GET
	res := run(*flagWorkers, *flagDuration, inproc("GET", "/ping", nil, ""))
	res.report("GET /ping  (no params, in-process)", *flagDuration)

	// Param extraction
	res = run(*flagWorkers, *flagDuration, inproc("GET", "/users/42/posts/99", nil, ""))
	res.report("GET /users/:id/posts/:postID (params)", *flagDuration)

	// POST with JSON body
	payload := []byte(`{"hello":"oniworks","n":42}`)
	res = run(*flagWorkers, *flagDuration, inproc("POST", "/echo", payload, "application/json"))
	res.report("POST /echo (JSON bind + response)", *flagDuration)

	// 404 miss
	res = run(*flagWorkers, *flagDuration, inproc("GET", "/no/such/route", nil, ""))
	res.report("GET /not-found (miss path)", *flagDuration)

	// Real TCP round-trip with one shared server+client — measures network overhead
	tcpSrv := httptest.NewServer(r)
	defer tcpSrv.Close()
	tcpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: *flagWorkers,
			DisableKeepAlives:   false,
			IdleConnTimeout:     120 * time.Second,
		},
		Timeout: 5 * time.Second,
	}
	// Parallel warm-up: open all worker connections at once
	var tcpWg sync.WaitGroup
	for i := 0; i < *flagWorkers; i++ {
		tcpWg.Add(1)
		go func() {
			defer tcpWg.Done()
			req, _ := http.NewRequest("GET", tcpSrv.URL+"/ping", nil)
			resp, _ := tcpClient.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}()
	}
	tcpWg.Wait()

	tcpRes := run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		req, _ := http.NewRequest("GET", tcpSrv.URL+"/ping", nil)
		t := time.Now()
		resp, err := tcpClient.Do(req)
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return 0, fmt.Errorf("status %d", resp.StatusCode)
		}
		return time.Since(t), nil
	})
	tcpRes.report("GET /ping  (TCP round-trip, keep-alive)", *flagDuration)

	fmt.Println()
}

// ─── 2. Oni Memory ───────────────────────────────────────────────────────────

func runMemory() {
	fmt.Printf("%s── Oni Memory (in-process)%s\n", bold, reset)

	mem := memory.New(memory.Options{EvictInterval: 5 * time.Second})
	defer mem.Shutdown()

	// Concurrent Set
	res := run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		key := fmt.Sprintf("key:%d", time.Now().UnixNano()%100000)
		mem.Set(key, "value", 30*time.Second)
		return time.Since(t), nil
	})
	res.report("Set (random key, 30s TTL)", *flagDuration)

	// Seed keys for Get test
	for i := 0; i < 10000; i++ {
		mem.Set(fmt.Sprintf("getkey:%d", i), i, 0)
	}

	// Concurrent Get (hot keys)
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		key := fmt.Sprintf("getkey:%d", time.Now().UnixNano()%10000)
		_, ok := mem.Get(key)
		if !ok {
			return 0, fmt.Errorf("key miss")
		}
		return time.Since(t), nil
	})
	res.report("Get (10K seeded keys)", *flagDuration)

	// Mixed read/write (80% read, 20% write)
	var mixCounter int64
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		n := atomic.AddInt64(&mixCounter, 1)
		if n%5 == 0 {
			mem.Set(fmt.Sprintf("mix:%d", n%1000), n, 10*time.Second)
		} else {
			mem.Get(fmt.Sprintf("mix:%d", n%1000))
		}
		return time.Since(t), nil
	})
	res.report("Mixed 80% Get / 20% Set", *flagDuration)

	// Atomic Incr
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		mem.Incr("stress:counter")
		return time.Since(t), nil
	})
	res.report("Incr (single atomic counter, high contention)", *flagDuration)

	// CompareAndSwap
	mem.Set("cas:lock", nil, 0)
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		mem.CompareAndSwap("cas:key", nil, "locked", time.Second)
		mem.Delete("cas:key")
		return time.Since(t), nil
	})
	res.report("CompareAndSwap + Delete (lock/unlock)", *flagDuration)

	// Pub/Sub — 20 subscribers, measure publish throughput
	const subCount = 20
	var received int64
	cancels := make([]func(), subCount)
	for i := 0; i < subCount; i++ {
		cancels[i] = mem.Subscribe("stress:channel", func(_ string, _ any) {
			atomic.AddInt64(&received, 1)
		})
	}

	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		mem.Publish("stress:channel", "payload")
		return time.Since(t), nil
	})
	for _, c := range cancels {
		c()
	}
	// Report effective message rate (published * subscribers)
	pubOps := atomic.LoadInt64(&res.ops)
	msgRate := float64(atomic.LoadInt64(&received)) / flagDuration.Seconds()
	_ = pubOps
	res.report(fmt.Sprintf("Pub/Sub (%d subscribers, %.0fK msgs/s effective)", subCount, msgRate/1000), *flagDuration)

	// TTL eviction — write short-TTL keys and verify they're gone
	start := time.Now()
	const ttlKeys = 5000
	for i := 0; i < ttlKeys; i++ {
		mem.Set(fmt.Sprintf("evict:%d", i), i, 200*time.Millisecond)
	}
	time.Sleep(600 * time.Millisecond) // wait for eviction loop
	var missing int
	for i := 0; i < ttlKeys; i++ {
		if !mem.Has(fmt.Sprintf("evict:%d", i)) {
			missing++
		}
	}
	evicted := float64(missing) / float64(ttlKeys) * 100
	evictLabel := fmt.Sprintf("TTL eviction (%.0f%% of %d keys cleaned up)", evicted, ttlKeys)
	evictColor := green
	if evicted < 95 {
		evictColor = yellow
	}
	fmt.Printf("  %-38s %s%s%s  elapsed: %s\n",
		evictLabel, evictColor, "✓", reset, time.Since(start).Round(time.Millisecond))

	fmt.Println()
}

// ─── 3. PostgreSQL / Query Builder ───────────────────────────────────────────

func runDatabase() {
	fmt.Printf("%s── PostgreSQL (via OniWorks Query Builder)%s\n", bold, reset)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, *flagDSN)
	if err != nil {
		fmt.Printf("  %s✗ connect failed: %v — run with --skip-db to skip%s\n\n", red, err, reset)
		return
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("  %s✗ ping failed: %v%s\n\n", red, err, reset)
		pool.Close()
		return
	}
	defer pool.Close()

	// Open via OniWorks DB wrapper
	db, err := database.Open(database.Config{
		Driver:      database.DriverPostgres,
		Host:        "localhost",
		Port:        5432,
		User:        "postgres",
		Password:    "password",
		Name:        "postgres",
		SSLMode:     "disable",
		MaxOpen:     *flagWorkers + 10,
		MaxIdle:     *flagWorkers,
		MaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		fmt.Printf("  %s✗ database.Open failed: %v%s\n\n", red, err, reset)
		return
	}
	defer db.Close()

	// Create stress test table
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS oni_stress (
			id    BIGSERIAL PRIMARY KEY,
			name  TEXT NOT NULL,
			score BIGINT NOT NULL DEFAULT 0,
			ts    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		fmt.Printf("  %s✗ create table: %v%s\n\n", red, err, reset)
		return
	}
	// Seed rows for SELECT tests
	_, _ = pool.Exec(ctx, `DELETE FROM oni_stress`)
	for i := 0; i < 1000; i++ {
		_, _ = pool.Exec(ctx, `INSERT INTO oni_stress (name, score) VALUES ($1, $2)`,
			fmt.Sprintf("user-%d", i), i)
	}
	fmt.Printf("  %sseeded 1,000 rows into oni_stress%s\n", gray, reset)

	// Silence query logging during stress test
	db.SetLogLevel(slog.LevelError)
	database.SetDefault(db)

	// Simple SELECT by primary key
	res := run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		id := (time.Now().UnixNano() % 1000) + 1
		var row struct {
			ID    int64  `db:"id"`
			Name  string `db:"name"`
			Score int64  `db:"score"`
		}
		err := database.Table("oni_stress").
			Where("id = ?", id).
			First(&row)
		if err != nil && err != database.ErrNotFound {
			return 0, err
		}
		return time.Since(t), nil
	})
	res.report("SELECT … WHERE id = ? (First)", *flagDuration)

	// Full table scan with LIMIT
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		var rows []struct {
			ID    int64  `db:"id"`
			Score int64  `db:"score"`
		}
		err := database.Table("oni_stress").
			Where("score > ?", time.Now().UnixNano()%500).
			OrderBy("score DESC").
			Limit(20).
			All(&rows)
		return time.Since(t), err
	})
	res.report("SELECT … WHERE score > ? ORDER BY LIMIT 20", *flagDuration)

	// COUNT
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		_, err := database.Table("oni_stress").
			Where("score > ?", 0).
			Count()
		return time.Since(t), err
	})
	res.report("COUNT(*) WHERE score > 0", *flagDuration)

	// INSERT
	var insertSeq int64
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		seq := atomic.AddInt64(&insertSeq, 1)
		_, err := pool.Exec(ctx,
			`INSERT INTO oni_stress (name, score) VALUES ($1, $2)`,
			fmt.Sprintf("stress-%d", seq), seq%10000)
		return time.Since(t), err
	})
	res.report("INSERT one row", *flagDuration)

	// UPDATE
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		id := (time.Now().UnixNano() % 1000) + 1
		err := database.Table("oni_stress").
			Where("id = ?", id).
			Update(database.Map{"score": time.Now().UnixNano() % 9999})
		return time.Since(t), err
	})
	res.report("UPDATE … WHERE id = ?", *flagDuration)

	// Transaction (read + update inside tx)
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		err := db.Transaction(func(tx *database.DB) error {
			id := (time.Now().UnixNano() % 1000) + 1
			var row struct {
				ID    int64 `db:"id"`
				Score int64 `db:"score"`
			}
			if err := tx.Table("oni_stress").Where("id = ?", id).First(&row); err != nil {
				return err
			}
			return tx.Table("oni_stress").
				Where("id = ?", id).
				Update(database.Map{"score": row.Score + 1})
		})
		return time.Since(t), err
	})
	res.report("Transaction: SELECT + UPDATE (read-modify-write)", *flagDuration)

	// Paginate
	res = run(*flagWorkers, *flagDuration, func() (time.Duration, error) {
		t := time.Now()
		var rows []struct {
			ID   int64  `db:"id"`
			Name string `db:"name"`
		}
		page := int(time.Now().UnixNano()%5) + 1
		_, err := database.Table("oni_stress").
			OrderBy("id ASC").
			Paginate(page, 25, &rows)
		return time.Since(t), err
	})
	res.report("Paginate (COUNT + SELECT, page 1-5)", *flagDuration)

	// Connection pool saturation test — more goroutines than idle connections
	satWorkers := *flagWorkers * 3
	satRes := run(satWorkers, *flagDuration/2, func() (time.Duration, error) {
		t := time.Now()
		var count int64
		err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM oni_stress`).Scan(&count)
		return time.Since(t), err
	})
	satRes.report(fmt.Sprintf("Pool saturation (%d workers > %d idle conns)", satWorkers, *flagWorkers), *flagDuration/2)

	// Cleanup
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS oni_stress`)
	fmt.Println()
}

// ─── 4. WebSocket Hub ─────────────────────────────────────────────────────────

func runWebSocket() {
	fmt.Printf("%s── WebSocket Hub (Oni Socket)%s\n", bold, reset)

	mem := memory.New(memory.Options{})
	defer mem.Shutdown()

	// Build a minimal hub using only the OniWorks router + gorilla WebSocket
	// (We test the full upgrade + read/write path manually to avoid import cycle)
	upgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	// broadcast channel: server sends messages here, all connections receive them
	broadcast := make(chan []byte, 1024)

	type conn struct {
		ws *websocket.Conn
	}

	var (
		connsMu sync.RWMutex
		conns   []*conn
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := &conn{ws: ws}
		connsMu.Lock()
		conns = append(conns, c)
		connsMu.Unlock()
		// Write loop
		for msg := range broadcast {
			_ = ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
		ws.Close()
		connsMu.Lock()
		for i, cc := range conns {
			if cc == c {
				conns = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		connsMu.Unlock()
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/ws"
	makeDial := func(srv *httptest.Server) websocket.Dialer {
		return websocket.Dialer{
			NetDial: func(_, _ string) (net.Conn, error) {
				return net.Dial("tcp", srv.Listener.Addr().String())
			},
		}
	}
	dialer := makeDial(srv)

	// ── connect N clients ─────────────────────────────────────────────
	connCount := min(*flagWorkers, 200)
	var connected int64
	var connectErr int64
	var dialWg sync.WaitGroup
	for i := 0; i < connCount; i++ {
		dialWg.Add(1)
		go func() {
			defer dialWg.Done()
			ws, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				atomic.AddInt64(&connectErr, 1)
				return
			}
			atomic.AddInt64(&connected, 1)
			_ = ws
		}()
	}
	dialWg.Wait()
	time.Sleep(200 * time.Millisecond) // let all connections settle

	connColor := green
	if connectErr > 0 {
		connColor = yellow
	}
	fmt.Printf("  %-38s %s%d/%d connected%s  errors: %d\n",
		"Concurrent connection establishment",
		connColor, atomic.LoadInt64(&connected), connCount, reset,
		atomic.LoadInt64(&connectErr),
	)

	// ── broadcast throughput (server → all clients) ───────────────────
	// Close the connection-establishment clients
	connsMu.Lock()
	for _, c := range conns {
		c.ws.Close()
	}
	conns = conns[:0]
	connsMu.Unlock()
	time.Sleep(150 * time.Millisecond)

	const bcastConns = 50
	var received int64
	bcastClientConns := make([]*websocket.Conn, 0, bcastConns)
	for i := 0; i < bcastConns; i++ {
		ws, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			continue
		}
		bcastClientConns = append(bcastClientConns, ws)
		go func(ws *websocket.Conn) {
			ws.SetReadDeadline(time.Time{}) // no deadline on reader
			for {
				_, _, err := ws.ReadMessage()
				if err != nil {
					return
				}
				atomic.AddInt64(&received, 1)
			}
		}(ws)
	}
	time.Sleep(200 * time.Millisecond)

	// Snapshot the server-side connections for broadcasting
	serverConns := func() []*conn {
		connsMu.RLock()
		defer connsMu.RUnlock()
		out := make([]*conn, len(conns))
		copy(out, conns)
		return out
	}()

	wsBcastPayload := []byte(`{"type":"broadcast","payload":"hello OniWorks"}`)
	bcastStart := time.Now()
	bcastCount := 0
	bcastDeadline := time.Now().Add(*flagDuration / 2)
	var bcastLatencies []int64
	var bcastExpected int64
	for time.Now().Before(bcastDeadline) {
		t := time.Now()
		sent := 0
		for _, c := range serverConns {
			_ = c.ws.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			if err := c.ws.WriteMessage(websocket.TextMessage, wsBcastPayload); err == nil {
				sent++
			}
		}
		atomic.AddInt64(&bcastExpected, int64(sent))
		bcastLatencies = append(bcastLatencies, time.Since(t).Microseconds())
		bcastCount++
		time.Sleep(time.Millisecond)
	}
	bcastDuration := time.Since(bcastStart)
	time.Sleep(300 * time.Millisecond) // drain in-flight messages

	sort.Slice(bcastLatencies, func(i, j int) bool { return bcastLatencies[i] < bcastLatencies[j] })
	p50, p95, p99, avg := latencyStats(bcastLatencies)
	actualReceived := atomic.LoadInt64(&received)
	expected := atomic.LoadInt64(&bcastExpected)
	deliveryRate := 0.0
	if expected > 0 {
		deliveryRate = float64(actualReceived) / float64(expected) * 100
	}

	fmt.Printf("  %-38s %s%d broadcasts/s%s  to %d conns  p50:%5.1fms  p95:%5.1fms  p99:%5.1fms  avg:%5.1fms\n",
		"Broadcast throughput (server→clients)",
		green,
		int(float64(bcastCount)/bcastDuration.Seconds()), reset,
		len(serverConns),
		float64(p50)/1000, float64(p95)/1000, float64(p99)/1000, float64(avg)/1000,
	)
	deliveryColor := green
	if deliveryRate < 95 {
		deliveryColor = yellow
	}
	fmt.Printf("  %-38s %s%.1f%% delivery rate%s  (%d/%d messages received)\n",
		"Message delivery accuracy",
		deliveryColor, deliveryRate, reset,
		actualReceived, expected,
	)

	// ── per-connection round-trip latency ────────────────────────────
	echoMux := http.NewServeMux()
	echoUpgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	echoMux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		ws, err := echoUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			mt, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if err := ws.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	})
	echoSrv := httptest.NewServer(echoMux)
	defer echoSrv.Close()
	echoDial := makeDial(echoSrv)
	echoURL := "ws" + echoSrv.URL[4:] + "/echo"
	echoMsg := []byte(`{"ping":true}`)

	// Each goroutine owns its own dedicated connection — no concurrent sharing.
	echoWorkers := min(*flagWorkers, 50)
	echoRes := &result{latencies: make([]int64, 0, echoWorkers*2000)}
	var echoWg sync.WaitGroup
	echoDeadline := time.Now().Add(*flagDuration / 2)

	for i := 0; i < echoWorkers; i++ {
		echoWg.Add(1)
		go func() {
			defer echoWg.Done()
			ws, _, err := echoDial.Dial(echoURL, nil)
			if err != nil {
				atomic.AddInt64(&echoRes.errors, 1)
				return
			}
			defer ws.Close()
			for time.Now().Before(echoDeadline) {
				t := time.Now()
				_ = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := ws.WriteMessage(websocket.TextMessage, echoMsg); err != nil {
					echoRes.record(0, true)
					return
				}
				_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, _, err := ws.ReadMessage()
				echoRes.record(time.Since(t), err != nil)
				if err != nil {
					return
				}
			}
		}()
	}
	echoWg.Wait()
	echoRes.report(fmt.Sprintf("Round-trip echo (%d dedicated conns, sequential)", echoWorkers), *flagDuration/2)

	for _, c := range bcastClientConns {
		c.Close()
	}
	fmt.Println()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Silence unused import warning for json in test builds
var _ = json.Marshal
