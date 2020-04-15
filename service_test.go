package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"net"
	"net/http"
	"taxi/models"
	"testing"
	"time"
)

func TestService(t *testing.T) {
	port := chooseRandomUnusedPort()
	s := NewService()

	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx, port)
	waitForHTTPServerStart(port)

	callsCount := 100
	ch := make(chan *http.Response)
	for i := 0; i < callsCount; i++ {
		go func() {
			r, err := http.Get(fmt.Sprintf("http://localhost:%d/request", port))
			require.NoError(t, err)
			ch <- r
		}()
	}

	var results []http.Response
	for {
		result := <-ch
		results = append(results, *result)

		if len(results) == callsCount {
			break
		}
	}

	var offers []models.Offer
	replyChan := make(chan models.Offer)
	s.commandChan <- Command{cmdType: GetCommand, replyChan: replyChan}
	for o := range replyChan {
		offers = append(offers, o)
	}
	var count int
	for _, o := range offers {
		count += o.Views
	}
	assert.Equal(t, callsCount, count)
	cancel()
	s.Wait()
}

func waitForHTTPServerStart(port int) {
	// wait for up to 3 seconds for server to start before returning it
	client := http.Client{Timeout: time.Second}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Millisecond * 10)
		if resp, err := client.Get(fmt.Sprintf("http://localhost:%d", port)); err == nil {
			_ = resp.Body.Close()
			return
		}
	}
}

func chooseRandomUnusedPort() (port int) {
	for i := 0; i < 10; i++ {
		port = 40000 + int(rand.Int31n(10000))
		if ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port)); err == nil {
			_ = ln.Close()
			break
		}
	}
	return port
}
