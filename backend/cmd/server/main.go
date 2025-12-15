package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"airline_builder/internal/api"
	"airline_builder/internal/game"
	"airline_builder/internal/models"
)

var (
	aircraftCosts = map[string]float64{
		"ATR72":      26_000_000,
		"CRJ9":       44_000_000,
		"E175":       48_000_000,
		"E190":       52_000_000,
		"E195E2":     60_000_000,
		"B737-700":   82_000_000,
		"B737-800":   96_000_000,
		"B737MAX8":   120_000_000,
		"A320":       98_000_000,
		"A320NEO":    110_000_000,
		"A321NEO":    125_000_000,
		"B767-300ER": 220_000_000,
		"B777-300ER": 375_000_000,
		"B787-9":     292_000_000,
		"A330-900":   296_000_000,
		"A350-900":   317_000_000,
		"B767-300F":  220_000_000,
		"B777F":      352_000_000,
		"A330-200F":  240_000_000,
		"B747-8F":    419_000_000,
		"B747-400":   250_000_000,
		"A380-800":   445_000_000,
	}
	aircraftLeadTicks = map[string]int{
		"ATR72":      5,
		"CRJ9":       6,
		"E175":       6,
		"E190":       6,
		"E195E2":     7,
		"B737-700":   7,
		"B737-800":   8,
		"B737MAX8":   8,
		"A320":       8,
		"A320NEO":    8,
		"A321NEO":    9,
		"B767-300ER": 10,
		"B777-300ER": 11,
		"B787-9":     10,
		"A330-900":   10,
		"A350-900":   11,
		"B767-300F":  9,
		"B777F":      11,
		"A330-200F":  9,
		"B747-8F":    12,
		"B747-400":   12,
		"A380-800":   12,
	}
)

const saveFilePath = "data/savegame.json"

func main() {
	engine := game.NewEngine(aircraftCosts, aircraftLeadTicks)
	engine.SetSavePath(saveFilePath)

	aircraftCatalog, err := loadAircraftDatabase("data/aircraft.json")
	if err != nil {
		log.Fatalf("failed to load aircraft: %v", err)
	}
	engine.SetAircraft(aircraftCatalog)

	if err := engine.LoadAirportsCSV("data/airports.csv"); err != nil {
		log.Fatalf("failed to load airports: %v", err)
	}

	if err := engine.LoadState(saveFilePath); err == nil {
		log.Printf("loaded savegame")
	} else {
		engine.SetState(models.GameState{
			Cash:  500_000_000,
			Speed: 1,
		})
		engine.SeedFleet()
	}
	engine.RecalcUtilization()

	handler := api.New(engine)

	port := getPort()
	log.Printf("Server listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func loadAircraftDatabase(path string) ([]models.Aircraft, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var aircraft []models.Aircraft
	if err := json.Unmarshal(data, &aircraft); err != nil {
		return nil, err
	}
	return aircraft, nil
}

func getPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "4000"
}

// ensure data directory exists when saving
func init() {
	_ = os.MkdirAll(filepath.Dir(saveFilePath), 0o755)
}
