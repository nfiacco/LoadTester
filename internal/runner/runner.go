package runner

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type LoadTestArgs struct {
	Duration   time.Duration
	Qps        uint64
	Workers    uint64 // Use multiple workers to support high QPS in the event of slow responses
	MaxWorkers uint64
	AutoScale  bool
	Timeout    uint64
	Method     string
	OutputFile string
}

type Runner struct {
	target   string
	args     LoadTestArgs
	stopch   chan struct{}
	stopOnce sync.Once
	client   http.Client
}

type Result struct {
	Success   bool
	Latency   time.Duration
	Timestamp time.Time
	Seq       uint64
	Error     string
	Code      uint16
}

type loadTest struct {
	began time.Time
	seqmu sync.Mutex
	seq   uint64
}

func NewRunner(target string, args LoadTestArgs) *Runner {
	return &Runner{
		target:   target,
		args:     args,
		stopch:   make(chan struct{}),
		stopOnce: sync.Once{},
		client: http.Client{
			Timeout: time.Duration(args.Timeout) * time.Second,
		},
	}
}

func (r *Runner) Run() error {
	results := r.StartTest()
	resultList := []*Result{}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	w, err := createWriter(r.args.OutputFile)
	if err != nil {
		return fmt.Errorf("error opening %s: %s", r.args.OutputFile, err)
	}
	defer w.Close()

	for {
		select {
		case result, ok := <-results:
			if !ok {
				printResultSummary(resultList)
				return nil
			}
			resultList = append(resultList, result)
			if err := r.writeResult(w, result); err != nil {
				return err
			}
		case <-sig:
			stopSent := r.Stop()
			if !stopSent {
				// Exit immediately on second signal.
				return nil
			} else {
				fmt.Println("Shutting down...")
			}
		}
	}
}

func (r *Runner) Stop() bool {
	select {
	case <-r.stopch:
		return false
	default:
		r.stopOnce.Do(func() { close(r.stopch) })
		return true
	}
}

func (r *Runner) StartTest() chan *Result {
	var wg sync.WaitGroup
	lt := &loadTest{began: time.Now()}
	workers := r.args.Workers

	results := make(chan *Result)
	ticks := make(chan struct{})
	for i := uint64(0); i < workers; i++ {
		wg.Add(1)
		go r.runWorker(lt, &wg, ticks, results)
	}

	go func() {
		// The workers will shut down once the ticks channel is closed, so once this loop ends the
		// workers will shut down too
		defer func() {
			close(ticks)
			wg.Wait()
			close(results)
			r.Stop()
		}()

		count := uint64(0)
		for {
			elapsed := time.Since(lt.began)
			if r.args.Duration > 0 && elapsed > r.args.Duration {
				return
			}

			wait, stop := r.pace(elapsed, count)
			if stop {
				return
			}

			time.Sleep(wait)

			if r.args.AutoScale && workers < r.args.MaxWorkers {
				select {
				case ticks <- struct{}{}:
					count++
					continue
				case <-r.stopch:
					return
				default:
					// all workers are blocked. start one more and try again
					workers++
					wg.Add(1)
					go r.runWorker(lt, &wg, ticks, results)
				}
			}

			select {
			case ticks <- struct{}{}:
				count++
			case <-r.stopch:
				return
			}
		}
	}()

	return results
}

func (r *Runner) pace(elapsed time.Duration, requests uint64) (time.Duration, bool) {
	expectedRequests := uint64(r.args.Qps) * uint64(elapsed/time.Second)
	if requests < expectedRequests {
		// Running behind, send next request immediately.
		return 0, false
	}

	interval := uint64(time.Second.Nanoseconds() / int64(r.args.Qps))
	if math.MaxInt64/interval < requests {
		// We would overflow delta if we continued, so stop the run.
		return 0, true
	}

	delta := time.Duration((requests + 1) * interval)

	// Zero or negative durations cause time.Sleep to return immediately.
	return delta - elapsed, false
}

func (r *Runner) runWorker(lt *loadTest, wg *sync.WaitGroup, ticks <-chan struct{}, results chan<- *Result) {
	defer wg.Done()

	for range ticks {
		results <- r.sendRequest(lt)
	}
}

func (r *Runner) sendRequest(lt *loadTest) *Result {
	var result Result
	var err error

	lt.seqmu.Lock()
	result.Timestamp = lt.began.Add(time.Since(lt.began))
	result.Seq = lt.seq
	lt.seq++
	lt.seqmu.Unlock()

	defer func() {
		result.Latency = time.Since(result.Timestamp)
		if err != nil {
			result.Error = err.Error()
		}
	}()

	req, err := http.NewRequest(r.args.Method, r.target, nil)
	if err != nil {
		result.Error = err.Error()
		return &result
	}

	res, err := r.client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return &result
	}
	defer res.Body.Close()

	if result.Code = uint16(res.StatusCode); result.Code < 200 || result.Code >= 400 {
		result.Error = res.Status
	}

	return &result
}

func createWriter(name string) (*os.File, error) {
	switch name {
	case "stdout":
		return os.Stdout, nil
	default:
		return os.Create(name)
	}
}

func (r *Runner) writeResult(w io.Writer, result *Result) error {
	enc := csv.NewWriter(w)
	err := enc.Write([]string{
		strconv.FormatInt(result.Timestamp.UnixNano(), 10),
		strconv.FormatUint(uint64(result.Code), 10),
		strconv.FormatInt(result.Latency.Nanoseconds(), 10),
		result.Error,
		strconv.FormatUint(result.Seq, 10),
	})
	if err != nil {
		return err
	}

	enc.Flush()

	return enc.Error()
}

func printResultSummary(results []*Result) {
	var success, failure int
	var totalLatency time.Duration

	for _, r := range results {
		if r.Code >= 200 && r.Code < 400 {
			success++
		} else {
			failure++
		}
		totalLatency += r.Latency
	}

	fmt.Printf("Successful Requests: %d, Failed Requests: %d\n", success, failure)
	fmt.Printf("Average latency: %s\n", totalLatency/time.Duration(len(results)))
	fmt.Printf("Error rate: %.2f%%\n", float64(failure)/float64(len(results))*100)
}
