// Copyright (C) ENEO Tecnologia SL - 2016
//
// Authors: Diego Fernández Barrera <dfernandez@redborder.com> <bigomby@gmail.com>
// 					Eugenio Pérez Martín <eugenio@redborder.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/lgpl-3.0.txt>.

package rbforwarder

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/davidredborder/rbforwarder/components/batch"
	"github.com/davidredborder/rbforwarder/components/httpsender"
	"github.com/davidredborder/rbforwarder/utils"
)

type Null struct{}

// Workers returns the number of workers
func (null *Null) Workers() int {
	return 1
}

func (null *Null) Spawn(id int) utils.Composer {
	n := *null
	return &n
}

func (null *Null) OnMessage(m *utils.Message, done utils.Done) {
	done(m, 0, "")
}

func BenchmarkQueue1(b *testing.B)      { benchmarkQueue(1, b) }
func BenchmarkQueue10(b *testing.B)     { benchmarkQueue(10, b) }
func BenchmarkQueue1000(b *testing.B)   { benchmarkQueue(100, b) }
func BenchmarkQueue10000(b *testing.B)  { benchmarkQueue(1000, b) }
func BenchmarkQueue100000(b *testing.B) { benchmarkQueue(10000, b) }

func BenchmarkHTTP(b *testing.B) { benchmarkHTTP(b) }

func BenchmarkBatch1(b *testing.B)      { benchmarkBatch(1, b) }
func BenchmarkBatch10(b *testing.B)     { benchmarkBatch(10, b) }
func BenchmarkBatch1000(b *testing.B)   { benchmarkBatch(100, b) }
func BenchmarkBatch10000(b *testing.B)  { benchmarkBatch(1000, b) }
func BenchmarkBatch100000(b *testing.B) { benchmarkBatch(10000, b) }

func BenchmarkHTTPBatch1(b *testing.B)      { benchmarkHTTPBatch(1, b) }
func BenchmarkHTTPBatch10(b *testing.B)     { benchmarkHTTPBatch(10, b) }
func BenchmarkHTTPBatch1000(b *testing.B)   { benchmarkHTTPBatch(100, b) }
func BenchmarkHTTPBatch10000(b *testing.B)  { benchmarkHTTPBatch(1000, b) }
func BenchmarkHTTPBatch100000(b *testing.B) { benchmarkHTTPBatch(10000, b) }

func benchmarkQueue(queue int, b *testing.B) {
	var wg sync.WaitGroup
	var components []interface{}

	f := NewRBForwarder(Config{
		Retries:   3,
		Backoff:   5,
		QueueSize: queue,
	})

	null := &Null{}
	components = append(components, null)

	f.PushComponents(components)
	f.Run()

	opts := map[string]interface{}{}

	wg.Add(1)
	b.ResetTimer()
	go func() {
		for report := range f.GetReports() {
			r := report.(Report)
			if r.Code > 0 {
				b.FailNow()
			}
			if r.Opaque.(int) >= b.N-1 {
				break
			}
		}

		wg.Done()
	}()

	for i := 0; i < b.N; i++ {
		data := fmt.Sprintf("{\"message\": %d}", i)
		f.Produce([]byte(data), opts, i)
	}

	wg.Wait()
}

func benchmarkBatch(batchSize int, b *testing.B) {
	var wg sync.WaitGroup
	var components []interface{}

	f := NewRBForwarder(Config{
		Retries:   3,
		Backoff:   5,
		QueueSize: 10000,
	})

	batch := &batcher.Batcher{
		Config: batcher.Config{
			TimeoutMillis: 1000,
			Limit:         uint64(batchSize),
		},
	}
	components = append(components, batch)

	f.PushComponents(components)
	f.Run()

	opts := map[string]interface{}{
		"batch_group": "test",
	}

	wg.Add(1)
	b.ResetTimer()
	go func() {
		for report := range f.GetReports() {
			r := report.(Report)
			if r.Code > 0 {
				b.FailNow()
			}
			if r.Opaque.(int) >= b.N-1 {
				break
			}
		}

		wg.Done()
	}()

	for i := 0; i < b.N; i++ {
		data := fmt.Sprintf("{\"message\": %d}", i)
		f.Produce([]byte(data), opts, i)
	}
	b.StopTimer()

	wg.Wait()
}

func benchmarkHTTPBatch(batchSize int, b *testing.B) {
	var wg sync.WaitGroup
	var components []interface{}

	f := NewRBForwarder(Config{
		Retries:   3,
		Backoff:   5,
		QueueSize: 10000,
	})

	batch := &batcher.Batcher{
		Config: batcher.Config{
			TimeoutMillis: 1000,
			Limit:         uint64(batchSize),
		},
	}
	components = append(components, batch)

	sender := &httpsender.HTTPSender{
		Config: httpsender.Config{
			URL: "http://localhost:8888",
		},
	}
	components = append(components, sender)

	f.PushComponents(components)
	sender.Client = NewTestClient(200, func(r *http.Request) {})

	f.Run()

	opts := map[string]interface{}{
		"http_endpoint": "test",
		"batch_group":   "test",
	}

	wg.Add(1)
	b.ResetTimer()
	go func() {
		for report := range f.GetReports() {
			r := report.(Report)
			if r.Code > 0 {
				b.FailNow()
			}
			if r.Opaque.(int) >= b.N-1 {
				break
			}
		}

		wg.Done()
	}()

	for i := 0; i < b.N; i++ {
		data := fmt.Sprintf("{\"message\": %d}", i)
		f.Produce([]byte(data), opts, i)
	}

	wg.Wait()
}

func benchmarkHTTP(b *testing.B) {
	var wg sync.WaitGroup
	var components []interface{}

	f := NewRBForwarder(Config{
		Retries:   3,
		Backoff:   5,
		QueueSize: 10000,
	})

	sender := &httpsender.HTTPSender{
		Config: httpsender.Config{
			URL: "http://localhost:8888",
		},
	}
	components = append(components, sender)

	f.PushComponents(components)
	sender.Client = NewTestClient(200, func(r *http.Request) {})

	f.Run()

	opts := map[string]interface{}{
		"http_endpoint": "test",
	}

	wg.Add(1)
	b.ResetTimer()
	go func() {
		for report := range f.GetReports() {
			r := report.(Report)
			if r.Code > 0 {
				b.FailNow()
			}
			if r.Opaque.(int) >= b.N-1 {
				break
			}
		}

		wg.Done()
	}()

	for i := 0; i < b.N; i++ {
		data := fmt.Sprintf("{\"message\": %d}", i)
		f.Produce([]byte(data), opts, i)
	}

	wg.Wait()
}

////////////////////////////////////////////////////////////////////////////////
/// Aux functions
////////////////////////////////////////////////////////////////////////////////

func NewTestClient(code int, cb func(*http.Request)) *http.Client {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			cb(r)
		}))

	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	return &http.Client{Transport: transport}
}
