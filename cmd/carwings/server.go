package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joeshaw/carwings"
)

const commandTimeout = 5 * time.Second

var ServerRefreshTime time.Duration = 10 * time.Minute

func updateLoop(ctx context.Context, s *carwings.Session) {
	_, err := s.UpdateStatus()
	if err != nil {
		fmt.Printf("Error updating status: %s\n", err)
	}

	t := time.NewTicker(ServerRefreshTime)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			_, err := s.UpdateStatus()
			if err != nil {
				fmt.Printf("Error updating status: %s\n", err)
			}
		}
	}
}

func runServer(s *carwings.Session, cfg config, args []string) error {
	var srv http.Server

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		cancel()
		srv.Shutdown(context.Background())
	}()

	go updateLoop(ctx, s)

	rt := mux.NewRouter()
	rt.Use(commonMiddleware) // sets content-type
	rt.HandleFunc("/battery", func(w http.ResponseWriter, r *http.Request) { handleBattery(w, r, s) }).Methods("GET")
	rt.HandleFunc("/climate", func(w http.ResponseWriter, r *http.Request) { handleClimate(w, r, s) }).Methods("GET")
	rt.HandleFunc("/locate", func(w http.ResponseWriter, r *http.Request) { handleLocate(w, r, s) }).Methods("GET")
	rt.HandleFunc("/daily", func(w http.ResponseWriter, r *http.Request) { handleDaily(w, r, s) }).Methods("GET")
	rt.HandleFunc("/monthly", func(w http.ResponseWriter, r *http.Request) { handleMonthly(w, r, s) }).Methods("GET")
	rt.HandleFunc("/charging/on", func(w http.ResponseWriter, r *http.Request) { handleChargingOn(w, r, s) }).Methods("PUT")
	rt.HandleFunc("/climate/{which:(?:off|on)}", func(w http.ResponseWriter, r *http.Request) { handleClimateChange(w, r, s) }).Methods("PUT")
	http.Handle("/", rt)

	srv.Addr = ":8040"
	srv.Handler = nil
	fmt.Printf("Starting HTTP server on %s...\n", srv.Addr)
	return srv.ListenAndServe()
}

// commonMiddleware sets the content type for all returns
func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func handleBattery(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	status, err := s.BatteryStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func handleClimate(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	status, err := s.ClimateControlStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func handleLocate(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	status, err := s.LocateVehicle()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func handleDaily(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	status, err := s.GetDailyStatistics(time.Now().Local())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func handleMonthly(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	status, err := s.GetMonthlyStatistics(time.Now().Local())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func handleChargingOn(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	fmt.Println("Charging request")

	ch := make(chan error, 1)
	go func() {
		ch <- s.ChargingRequest()
	}()

	select {
	case err := <-ch:
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case <-time.After(commandTimeout):
		w.WriteHeader(http.StatusAccepted)
	}
}

func handleClimateChange(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	vars := mux.Vars(r)
	change := vars["change"]
	fmt.Println("Climate control", change, "request")

	ch := make(chan error, 1)
	go func() {
		if change == "on" {
			_, err := s.ClimateOnRequest()
			ch <- err
		} else {
			_, err := s.ClimateOffRequest()
			ch <- err
		}
	}()

	select {
	case err := <-ch:
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case <-time.After(commandTimeout):
		w.WriteHeader(http.StatusAccepted)
	}
}
