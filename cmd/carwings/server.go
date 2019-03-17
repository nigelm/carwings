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

	http.HandleFunc("/battery", func(w http.ResponseWriter, r *http.Request) { handleBattery(w, r, s) })
	http.HandleFunc("/climate", func(w http.ResponseWriter, r *http.Request) { handleClimate(w, r, s) })
	http.HandleFunc("/locate", func(w http.ResponseWriter, r *http.Request) { handleLocate(w, r, s) })
	http.HandleFunc("/daily", func(w http.ResponseWriter, r *http.Request) { handleDaily(w, r, s) })
	http.HandleFunc("/monthly", func(w http.ResponseWriter, r *http.Request) { handleMonthly(w, r, s) })
	http.HandleFunc("/charging/on", func(w http.ResponseWriter, r *http.Request) { handleChargingOn(w, r, s) })
	http.HandleFunc("/climate/on", func(w http.ResponseWriter, r *http.Request) { handleClimateOn(w, r, s) })
	http.HandleFunc("/climate/off", func(w http.ResponseWriter, r *http.Request) { handleClimateOff(w, r, s) })

	srv.Addr = ":8040"
	srv.Handler = nil
	fmt.Printf("Starting HTTP server on %s...\n", srv.Addr)
	return srv.ListenAndServe()
}

func handleBattery(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "GET":
		status, err := s.BatteryStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(status)

	default:
		http.NotFound(w, r)
		return
	}
}

func handleClimate(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "GET":
		status, err := s.ClimateControlStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(status)

	default:
		http.NotFound(w, r)
		return
	}
}

func handleLocate(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "GET":
		status, err := s.LocateVehicle()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(status)

	default:
		http.NotFound(w, r)
		return
	}
}

func handleDaily(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "GET":
		status, err := s.GetDailyStatistics(time.Now().Local())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(status)

	default:
		http.NotFound(w, r)
		return
	}
}

func handleMonthly(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "GET":
		status, err := s.GetMonthlyStatistics(time.Now().Local())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(status)

	default:
		http.NotFound(w, r)
		return
	}
}

func handleChargingOn(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "POST":
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

	default:
		http.NotFound(w, r)
		return
	}
}

func handleClimateOn(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "POST":
		fmt.Println("Climate control on request")

		ch := make(chan error, 1)
		go func() {
			_, err := s.ClimateOnRequest()
			ch <- err
		}()

		select {
		case err := <-ch:
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

		case <-time.After(commandTimeout):
			w.WriteHeader(http.StatusAccepted)
		}

	default:
		http.NotFound(w, r)
		return
	}
}

func handleClimateOff(w http.ResponseWriter, r *http.Request, s *carwings.Session) {
	switch r.Method {
	case "POST":
		fmt.Println("Climate control off request")

		ch := make(chan error, 1)
		go func() {
			_, err := s.ClimateOffRequest()
			ch <- err
		}()

		select {
		case err := <-ch:
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

		case <-time.After(commandTimeout):
			w.WriteHeader(http.StatusAccepted)
		}

	default:
		http.NotFound(w, r)
		return
	}
}
