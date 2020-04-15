package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"taxi/models"
	"time"
)

type CommandType int

const (
	OfferCount  = 50
	StopCommand = iota
	GetCommand
	SetCommand
	IncCommand
)

type Command struct {
	cmdType   CommandType
	key       int
	offer     models.Offer
	replyChan chan models.Offer
}

type Service struct {
	commandChan chan Command
	ticker      *time.Ticker
	terminated chan struct{}
}

func NewService() *Service {
	return &Service{
		make(chan Command),
		time.NewTicker(time.Millisecond * 200),
		make(chan struct{}),
	}
}

func (s *Service) Run(ctx context.Context, port int) {
	s.startOfferManager()
	s.startTicker()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.routers(),
	}

	go func() {
		err := server.ListenAndServe()
		log.Printf("[WARN] http server terminated, %s", err)
	}()
	log.Print("[INFO] server started")

	<-ctx.Done()
	s.ticker.Stop()
	log.Print("[INFO] ticker stopped")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	log.Print("[INFO] shutdown initiated")
	if err := server.Shutdown(ctxShutDown); err != nil {
		log.Printf("HTTP server Shutdown: %v", err)
	}
	log.Print("[INFO] shutdown completed")

	s.commandChan <- Command{cmdType: StopCommand}

	close(s.terminated)
}

// Wait for service completion (termination)
func (s *Service) Wait() {
	<-s.terminated
}

func (s *Service) startTicker() {
	go func() {
		for range s.ticker.C {
			key := rand.Intn(OfferCount)
			offer := models.Offer{Body: randomString(2), Views: 0}
			s.commandChan <- Command{cmdType: SetCommand, key: key, offer: offer}
		}
	}()
	log.Print("[INFO] ticker started")
}

func (s *Service) routers() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/request", http.HandlerFunc(s.getRandOfferHandler))
	mux.Handle("/admin/requests", http.HandlerFunc(s.getOffersHandler))

	return mux
}

func (s *Service) getRandOfferHandler(w http.ResponseWriter, r *http.Request) {
	replyChan := make(chan models.Offer)
	s.commandChan <- Command{cmdType: IncCommand, key: rand.Intn(OfferCount), replyChan: replyChan}
	reply := <-replyChan

	if _, err := w.Write([]byte(reply.Body)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Service) getOffersHandler(w http.ResponseWriter, r *http.Request) {
	var offers []models.Offer
	replyChan := make(chan models.Offer)
	s.commandChan <- Command{cmdType: GetCommand, replyChan: replyChan}
	for o := range replyChan {
		offers = append(offers, o)
	}

	var buf []byte
	for _, o := range offers {
		if o.Views > 0 {
			buf = bytes.Join([][]byte{buf, []byte("\n")}, []byte(o.Body+": "+strconv.Itoa(o.Views)))
		}
	}

	if _, err := w.Write(buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Service) startOfferManager() {
	offers := make([]models.Offer, 0, OfferCount)
	for i := 0; i < OfferCount; i++ {
		o := models.Offer{Body: randomString(2), Views: 0}
		offers = append(offers, o)
	}
	var inActiveOffers []models.Offer

	go func() {
		for cmd := range s.commandChan {
			switch cmd.cmdType {
			case StopCommand:
				log.Print("[INFO] offer manager stopped")
				return
			case GetCommand:
				for _, o := range append(offers, inActiveOffers...) {
					cmd.replyChan <- o
				}
				close(cmd.replyChan)
			case SetCommand:
				inActive := offers[cmd.key]
				if inActive.Views > 0 {
					inActiveOffers = append(inActiveOffers, inActive)
				}
				offers[cmd.key] = cmd.offer
			case IncCommand:
				offers[cmd.key].Views++
				cmd.replyChan <- offers[cmd.key]
			default:
				log.Print("unknown command type", cmd.cmdType)
			}
		}
	}()
	log.Print("[INFO] offer manager started")
}

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

func randomString(len int) string {
	randomBytes := make([]byte, len)
	for i := 0; i < len; i++ {
		randomBytes[i] = byte(randomInt(97, 122))
	}
	return string(randomBytes)
}
