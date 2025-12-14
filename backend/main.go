package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type Airport struct {
	ID          string  `json:"id"`
	Ident       string  `json:"ident"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Latitude    float64 `json:"lat"`
	Longitude   float64 `json:"lon"`
	Country     string  `json:"country"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	IATA        string  `json:"iata"`
	ICAO        string  `json:"icao"`
	RunwayM     int     `json:"runway_m"`
	SlotsPerDay int     `json:"slots_per_day"`
	LandingFee  float64 `json:"landing_fee"`
	Curfew      bool    `json:"curfew"`
	CurfewStart int     `json:"curfew_start_hour"`
	CurfewEnd   int     `json:"curfew_end_hour"`
}

type AirportStore struct {
	Airports []Airport
}

type Aircraft struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	RangeKm       float64 `json:"range_km"`
	Seats         int     `json:"seats"`
	CruiseKmh     float64 `json:"cruise_kmh"`
	FuelCost      float64 `json:"fuel_cost_per_km"`
	TurnaroundMin int     `json:"turnaround_min"`
	Crew          int     `json:"crew,omitempty"`
	ThreeClass    int     `json:"three_class_seats,omitempty"`
	TwoClass      int     `json:"two_class_seats,omitempty"`
	OneClassMax   int     `json:"one_class_max_seats,omitempty"`
	CargoVolumeM3 float64 `json:"cargo_volume_m3,omitempty"`
	MaxPayloadKg  float64 `json:"max_payload_kg,omitempty"`
	MTOWKg        float64 `json:"mtow_kg,omitempty"`
	OEWKg         float64 `json:"oew_kg,omitempty"`
	FuelCapacityL float64 `json:"fuel_capacity_l,omitempty"`
	LengthM       float64 `json:"length_m,omitempty"`
	WingspanM     float64 `json:"wingspan_m,omitempty"`
	HeightM       float64 `json:"height_m,omitempty"`
	WingAreaM2    float64 `json:"wing_area_m2,omitempty"`
	EngineType    string  `json:"engine_type,omitempty"`
	EngineThrust  float64 `json:"engine_thrust_kn,omitempty"`
	ServiceCeilM  float64 `json:"service_ceiling_m,omitempty"`
	MaxSpeedKmh   float64 `json:"max_speed_kmh,omitempty"`
	TakeoffDistM  float64 `json:"takeoff_distance_m,omitempty"`
	LandingDistM  float64 `json:"landing_distance_m,omitempty"`
	IcaoType      string  `json:"icao_type,omitempty"`
}

type Route struct {
	ID                string  `json:"id"`
	From              string  `json:"from"`
	To                string  `json:"to"`
	Via               string  `json:"via,omitempty"`
	AircraftID        string  `json:"aircraft_id"`
	FrequencyPerDay   int     `json:"frequency_per_day"`
	EstimatedDemand   int     `json:"estimated_demand"`
	PricePerSeat      float64 `json:"price_per_seat"`
	EstRevenueTick    float64 `json:"estimated_revenue_tick"`
	EstCostTick       float64 `json:"estimated_cost_tick"`
	LoadFactor        float64 `json:"load_factor"`
	RevenuePerLeg     float64 `json:"revenue_per_leg"`
	CostPerLeg        float64 `json:"cost_per_leg"`
	LandingFeesPerLeg float64 `json:"landing_fees_per_leg"`
	ProfitPerTick     float64 `json:"profit_per_tick"`
	SeatsSoldPerLeg   int     `json:"seats_sold_per_leg"`
	BlockMinutes      float64 `json:"block_minutes"`
	CurfewBlocked     bool    `json:"curfew_blocked"`
	LastTickRevenue   float64 `json:"last_tick_revenue"`
	LastTickLoad      float64 `json:"last_tick_load"`
}

type GameState struct {
	Cash      float64      `json:"cash"`
	Routes    []Route      `json:"routes"`
	Fleet     []OwnedCraft `json:"fleet"`
	Tick      int          `json:"tick"`
	IsRunning bool         `json:"is_running"`
	Speed     int          `json:"speed"`
}

// OwnedCraft represents a specific aircraft in the player's fleet
// (not just the catalog entry).
type OwnedCraft struct {
	ID            string  `json:"id"`
	TemplateID    string  `json:"template_id"`
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	RangeKm       float64 `json:"range_km"`
	Seats         int     `json:"seats"`
	CruiseKmh     float64 `json:"cruise_kmh"`
	FuelCost      float64 `json:"fuel_cost_per_km"`
	TurnaroundMin int     `json:"turnaround_min"`
	Crew          int     `json:"crew,omitempty"`
	CargoVolumeM3 float64 `json:"cargo_volume_m3,omitempty"`
	MaxPayloadKg  float64 `json:"max_payload_kg,omitempty"`
	Status        string  `json:"status"` // active, delivering, maintenance
	AvailableIn   int     `json:"available_in_ticks"`
	Utilization   float64 `json:"utilization_pct"`
	Condition     float64 `json:"condition_pct"`
}

// acquisition configuration
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

var (
	simCtx    context.Context
	simCancel context.CancelFunc
	simTicker *time.Ticker
	rng       = rand.New(rand.NewSource(time.Now().UnixNano()))
)

const saveFilePath = "data/savegame.json"

func seedFleet() []OwnedCraft {
	starterIDs := map[string]bool{
		"A320":     true,
		"B737-800": true,
		"E190":     true,
	}
	out := make([]OwnedCraft, 0, len(starterIDs))
	for _, ac := range aircraftCatalog {
		if !starterIDs[ac.ID] {
			continue
		}
		out = append(out, OwnedCraft{
			ID:            ac.ID + "-1",
			TemplateID:    ac.ID,
			Name:          ac.Name,
			Role:          ac.Role,
			RangeKm:       ac.RangeKm,
			Seats:         ac.Seats,
			CruiseKmh:     ac.CruiseKmh,
			FuelCost:      ac.FuelCost,
			TurnaroundMin: ac.TurnaroundMin,
			Crew:          ac.Crew,
			CargoVolumeM3: ac.CargoVolumeM3,
			MaxPayloadKg:  ac.MaxPayloadKg,
			Status:        "active",
			AvailableIn:   0,
			Utilization:   0,
			Condition:     100,
		})
	}
	return out
}

var (
	store           *AirportStore
	airportsByIdent map[string]Airport
	stateMu         sync.Mutex
	state           GameState
	aircraftCatalog []Aircraft
)

const (
	manualMaintenanceTicks = 3
)

var (
	runwayReqMeters = map[string]int{
		"ATR72":      1300,
		"CRJ9":       1500,
		"E175":       1600,
		"E190":       1600,
		"E195E2":     1700,
		"B737-700":   1800,
		"B737-800":   1800,
		"B737MAX8":   1800,
		"A320":       1800,
		"A320NEO":    1800,
		"A321NEO":    2000,
		"B767-300ER": 2600,
		"B777-300ER": 3000,
		"B787-9":     2800,
		"A330-900":   2900,
		"A350-900":   3000,
		"B767-300F":  2600,
		"B777F":      3000,
		"A330-200F":  2900,
		"B747-8F":    3200,
		"B747-400":   3100,
		"A380-800":   3500,
	}
	curfewAppliesTo = map[string]bool{
		"large_airport":  true,
		"medium_airport": true,
	}
)

func main() {
	var err error
	aircraftCatalog, err = loadAircraftDatabase("data/aircraft.json")
	if err != nil {
		log.Fatalf("failed to load aircraft: %v", err)
	}
	store, err = loadAirports("data/airports.csv")
	if err != nil {
		log.Fatalf("failed to load airports: %v", err)
	}
	airportsByIdent = make(map[string]Airport, len(store.Airports))
	for _, a := range store.Airports {
		airportsByIdent[strings.ToUpper(a.Ident)] = a
	}

	loadedState, err := loadState(saveFilePath)
	if err == nil && (len(loadedState.Fleet) > 0 || len(loadedState.Routes) > 0) {
		state = loadedState
		log.Printf("loaded savegame with %d routes and %d aircraft", len(state.Routes), len(state.Fleet))
	} else {
		state = GameState{
			Cash:  500_000_000, // starting cash
			Fleet: seedFleet(),
			Speed: 1,
		}
	}
	recalcUtilizationLocked()

	r := chi.NewRouter()
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/airports", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tier := r.URL.Query().Get("tier")
		fields := r.URL.Query().Get("fields")
		filtered := filterAirports(store.Airports, tier)
		if strings.EqualFold(fields, "basic") {
			basic := make([]map[string]interface{}, 0, len(filtered))
			for _, a := range filtered {
				basic = append(basic, map[string]interface{}{
					"id": a.ID, "ident": a.Ident, "name": a.Name,
					"lat": a.Latitude, "lon": a.Longitude,
					"type": a.Type, "iata": a.IATA, "icao": a.ICAO,
				})
			}
			if err := json.NewEncoder(w).Encode(basic); err != nil {
				http.Error(w, "failed to encode", http.StatusInternalServerError)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(filtered); err != nil {
			http.Error(w, "failed to encode", http.StatusInternalServerError)
		}
	})

	r.Get("/aircraft/templates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(aircraftCatalog); err != nil {
			http.Error(w, "failed to encode", http.StatusInternalServerError)
		}
	})

	r.Get("/state", func(w http.ResponseWriter, r *http.Request) {
		stateMu.Lock()
		defer stateMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})

	r.Post("/routes", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			From       string `json:"from"`
			To         string `json:"to"`
			Via        string `json:"via,omitempty"`
			AircraftID string `json:"aircraft_id"`
			Frequency  int    `json:"frequency_per_day"`
			OneWay     bool   `json:"one_way"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		stateMu.Lock()
		defer stateMu.Unlock()
		route, err := buildRoute(req.From, req.To, req.Via, req.AircraftID, req.Frequency)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !req.OneWay {
			if marketExistsLocked(route.From, route.To) {
				http.Error(w, "market already served in either direction", http.StatusBadRequest)
				return
			}
		}
		if err := validateCapacityLocked(route); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.Routes = append(state.Routes, route)
		recalcUtilizationLocked()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(route)
	})

	r.Post("/tick", func(w http.ResponseWriter, r *http.Request) {
		stateMu.Lock()
		advanceTickLocked()
		stateMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})

	r.Post("/sim/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Speed int `json:"speed"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Speed == 0 {
			req.Speed = state.Speed
			if req.Speed == 0 {
				req.Speed = 1
			}
		}
		startSim(req.Speed)
		w.Header().Set("Content-Type", "application/json")
		stateMu.Lock()
		defer stateMu.Unlock()
		json.NewEncoder(w).Encode(state)
	})

	r.Post("/sim/pause", func(w http.ResponseWriter, r *http.Request) {
		pauseSim()
		w.Header().Set("Content-Type", "application/json")
		stateMu.Lock()
		defer stateMu.Unlock()
		json.NewEncoder(w).Encode(state)
	})

	r.Post("/sim/speed", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Speed int `json:"speed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Speed <= 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Speed < 1 {
			req.Speed = 1
		}
		if req.Speed > 4 {
			req.Speed = 4
		}
		stateMu.Lock()
		state.Speed = req.Speed
		running := state.IsRunning
		stateMu.Unlock()
		if running {
			startSim(req.Speed)
		}
		w.Header().Set("Content-Type", "application/json")
		stateMu.Lock()
		defer stateMu.Unlock()
		json.NewEncoder(w).Encode(state)
	})

	// Purchase aircraft
	r.Post("/fleet/purchase", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TemplateID string `json:"template_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TemplateID == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ac, err := findAircraft(req.TemplateID)
		if err != nil {
			http.Error(w, "unknown aircraft", http.StatusBadRequest)
			return
		}
		cost := aircraftCosts[ac.ID]
		if cost <= 0 {
			cost = 75_000_000
		}
		lead := aircraftLeadTicks[ac.ID]
		if lead <= 0 {
			lead = 6
		}

		stateMu.Lock()
		defer stateMu.Unlock()
		if state.Cash < cost {
			http.Error(w, "insufficient cash", http.StatusBadRequest)
			return
		}
		state.Cash -= cost
		newCraft := OwnedCraft{
			ID:            ac.ID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			TemplateID:    ac.ID,
			Name:          ac.Name,
			Role:          ac.Role,
			RangeKm:       ac.RangeKm,
			Seats:         ac.Seats,
			CruiseKmh:     ac.CruiseKmh,
			FuelCost:      ac.FuelCost,
			TurnaroundMin: ac.TurnaroundMin,
			Crew:          ac.Crew,
			CargoVolumeM3: ac.CargoVolumeM3,
			MaxPayloadKg:  ac.MaxPayloadKg,
			Status:        "delivering",
			AvailableIn:   lead,
			Utilization:   0,
			Condition:     100,
		}
		state.Fleet = append(state.Fleet, newCraft)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newCraft)
	})

	r.Post("/fleet/maintenance", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OwnedID string `json:"owned_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OwnedID == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		stateMu.Lock()
		defer stateMu.Unlock()
		var craft *OwnedCraft
		for i := range state.Fleet {
			if strings.EqualFold(state.Fleet[i].ID, req.OwnedID) {
				craft = &state.Fleet[i]
				break
			}
		}
		if craft == nil {
			http.Error(w, "unknown aircraft", http.StatusNotFound)
			return
		}
		if craft.Status == "delivering" {
			http.Error(w, "aircraft still delivering", http.StatusBadRequest)
			return
		}
		if state.Cash <= 0 {
			http.Error(w, "insufficient cash", http.StatusBadRequest)
			return
		}
		cost := maintenanceCost(craft.Condition)
		if state.Cash < cost {
			http.Error(w, "insufficient cash", http.StatusBadRequest)
			return
		}
		state.Cash -= cost
		craft.Condition = 100
		beginMaintenanceLocked(craft, manualMaintenanceTicks)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(craft)
	})

	addr := ":" + getPort()
	log.Println("backend listening on", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}
	return port
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS, POST")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loadAircraftDatabase(path string) ([]Aircraft, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var aircraft []Aircraft
	if err := json.Unmarshal(data, &aircraft); err != nil {
		return nil, err
	}
	return aircraft, nil
}

func loadState(path string) (GameState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GameState{}, err
	}
	var st GameState
	if err := json.Unmarshal(data, &st); err != nil {
		return GameState{}, err
	}
	return st, nil
}

func saveState(path string, st *GameState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadAirports(path string) (*AirportStore, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	idx := func(name string) int {
		for i, h := range headers {
			if h == name {
				return i
			}
		}
		return -1
	}

	idIdx := idx("id")
	identIdx := idx("ident")
	typeIdx := idx("type")
	nameIdx := idx("name")
	latIdx := idx("latitude_deg")
	lonIdx := idx("longitude_deg")
	countryIdx := idx("iso_country")
	regionIdx := idx("iso_region")
	cityIdx := idx("municipality")
	iataIdx := idx("iata_code")
	icaoIdx := idx("icao_code")

	var airports []Airport
	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}

		t := rec[typeIdx]
		if t == "closed" || t == "heliport" || t == "seaplane_base" {
			continue
		}

		lat, _ := strconv.ParseFloat(rec[latIdx], 64)
		lon, _ := strconv.ParseFloat(rec[lonIdx], 64)

		airports = append(airports, Airport{
			ID:          rec[idIdx],
			Ident:       rec[identIdx],
			Type:        t,
			Name:        rec[nameIdx],
			Latitude:    lat,
			Longitude:   lon,
			Country:     rec[countryIdx],
			Region:      rec[regionIdx],
			City:        rec[cityIdx],
			IATA:        rec[iataIdx],
			ICAO:        rec[icaoIdx],
			RunwayM:     runwayMetersForType(t),
			SlotsPerDay: slotsForType(t),
			LandingFee:  landingFeeForType(t),
			Curfew:      curfewForType(t),
			CurfewStart: 22,
			CurfewEnd:   6,
		})
	}

	log.Printf("loaded %d airports", len(airports))
	return &AirportStore{Airports: airports}, nil
}

func runwayMetersForType(t string) int {
	switch t {
	case "large_airport":
		return 3200
	case "medium_airport":
		return 2200
	case "small_airport":
		return 1200
	default:
		return 1000
	}
}

func slotsForType(t string) int {
	switch t {
	case "large_airport":
		return 200
	case "medium_airport":
		return 120
	case "small_airport":
		return 40
	default:
		return 20
	}
}

func landingFeeForType(t string) float64 {
	switch t {
	case "large_airport":
		return 3500
	case "medium_airport":
		return 2000
	case "small_airport":
		return 800
	default:
		return 500
	}
}

func curfewForType(t string) bool {
	return curfewAppliesTo[t]
}

func curfewAvailableMinutes(startHour, endHour int) float64 {
	// hours airports are closed from start to end (e.g., 22->6 blocks 8 hours)
	if startHour == endHour {
		return 24 * 60
	}
	blocked := 0
	if startHour < endHour {
		blocked = endHour - startHour
	} else {
		blocked = (24 - startHour) + endHour
	}
	openHours := 24 - blocked
	if openHours < 0 {
		openHours = 0
	}
	return float64(openHours) * 60
}

func filterAirports(all []Airport, tier string) []Airport {
	if tier == "" || tier == "all" {
		return all
	}
	tier = strings.ToLower(tier)
	keep := func(t string) bool {
		switch tier {
		case "large":
			return t == "large_airport"
		case "medium":
			return t == "large_airport" || t == "medium_airport"
		case "small":
			return t == "small_airport"
		default:
			return true
		}
	}
	out := make([]Airport, 0, len(all))
	for _, a := range all {
		if keep(a.Type) {
			out = append(out, a)
		}
	}
	return out
}

func buildRoute(from, to, via, aircraftID string, freq int) (Route, error) {
	if freq <= 0 {
		freq = 1
	}
	fromID := strings.ToUpper(strings.TrimSpace(from))
	toID := strings.ToUpper(strings.TrimSpace(to))
	viaID := strings.ToUpper(strings.TrimSpace(via))

	fromAp, ok := airportsByIdent[fromID]
	if !ok {
		return Route{}, http.ErrMissingFile
	}
	toAp, ok := airportsByIdent[toID]
	if !ok {
		return Route{}, http.ErrMissingFile
	}
	var viaAp Airport
	var hasVia bool
	if viaID != "" {
		v, ok := airportsByIdent[viaID]
		if !ok {
			return Route{}, http.ErrMissingFile
		}
		viaAp = v
		hasVia = true
	}

	ac, err := findAircraft(aircraftID)
	if err != nil {
		return Route{}, err
	}
	reqRunway := runwayReqMeters[ac.ID]
	if reqRunway == 0 {
		reqRunway = 1500
	}

	distMain := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	if distMain > ac.RangeKm {
		return Route{}, http.ErrBodyNotAllowed
	}
	if fromAp.RunwayM < reqRunway || toAp.RunwayM < reqRunway {
		return Route{}, fmt.Errorf("runway too short for %s", ac.ID)
	}

	var distVia1, distVia2 float64
	if hasVia {
		distVia1 = haversine(fromAp.Latitude, fromAp.Longitude, viaAp.Latitude, viaAp.Longitude)
		distVia2 = haversine(viaAp.Latitude, viaAp.Longitude, toAp.Latitude, toAp.Longitude)
		if distVia1 > ac.RangeKm || distVia2 > ac.RangeKm {
			return Route{}, http.ErrBodyNotAllowed
		}
		if viaAp.RunwayM < reqRunway {
			return Route{}, fmt.Errorf("%s runway too short for %s", viaAp.Ident, ac.ID)
		}
	}

	type leg struct {
		dist     float64
		demand   int
		sold     int
		price    float64
		revenue  float64
		cost     float64
		blockMin float64
		fees     float64
	}

	demandLeg := func(a, b Airport, opts demandOptions) int {
		return demandEstimateWithOpts(a, b, ac, freq, opts)
	}
	stopoverPriceFactor := 1.0
	if hasVia {
		totalViaDist := distVia1 + distVia2
		if totalViaDist > 0 && distMain > 0 {
			stopoverPriceFactor = distMain / totalViaDist
			if stopoverPriceFactor > 1 {
				stopoverPriceFactor = 1
			}
		}
	}

	legCost := func(a, b Airport, dist float64) (float64, float64) {
		fees := a.LandingFee + b.LandingFee
		return dist*ac.FuelCost + 800.0 + fees, fees
	}
	legBlock := func(dist float64) float64 {
		return (dist/ac.CruiseKmh)*60 + float64(ac.TurnaroundMin)
	}

	var legs []leg

	if hasVia {
		// Outbound: from->via with local + through demand, then via->to
		stopOpts := demandOptions{Stopover: true, PriceFactor: stopoverPriceFactor}
		d1 := demandLeg(fromAp, viaAp, demandOptions{}) + demandLeg(fromAp, toAp, stopOpts)
		d2 := demandLeg(viaAp, toAp, demandOptions{})
		// Inbound: to->via with local + through demand, then via->from
		d3 := demandLeg(toAp, viaAp, demandOptions{}) + demandLeg(toAp, fromAp, stopOpts)
		d4 := demandLeg(viaAp, fromAp, demandOptions{})

		for _, x := range []struct {
			dist   float64
			demand int
			a      Airport
			b      Airport
		}{
			{distVia1, d1, fromAp, viaAp},
			{distVia2, d2, viaAp, toAp},
			{distVia2, d3, toAp, viaAp},
			{distVia1, d4, viaAp, fromAp},
		} {
			sold := min(x.demand, ac.Seats)
			price := 0.13 * x.dist
			rev := float64(sold) * price
			cost, fees := legCost(x.a, x.b, x.dist)
			legs = append(legs, leg{
				dist:     x.dist,
				demand:   x.demand,
				sold:     sold,
				price:    price,
				revenue:  rev,
				cost:     cost,
				blockMin: legBlock(x.dist),
				fees:     fees,
			})
		}
	} else {
		// Simple round trip
		for _, x := range []struct {
			dist   float64
			demand int
			a      Airport
			b      Airport
		}{
			{distMain, demandLeg(fromAp, toAp, demandOptions{}), fromAp, toAp},
			{distMain, demandLeg(toAp, fromAp, demandOptions{}), toAp, fromAp},
		} {
			sold := min(x.demand, ac.Seats)
			price := 0.13 * x.dist
			rev := float64(sold) * price
			cost, fees := legCost(x.a, x.b, x.dist)
			legs = append(legs, leg{
				dist:     x.dist,
				demand:   x.demand,
				sold:     sold,
				price:    price,
				revenue:  rev,
				cost:     cost,
				blockMin: legBlock(x.dist),
				fees:     fees,
			})
		}
	}

	totalRevenue := 0.0
	totalCost := 0.0
	totalSold := 0
	totalDemand := 0
	totalBlock := 0.0
	totalFees := 0.0
	for _, l := range legs {
		totalRevenue += l.revenue
		totalCost += l.cost
		totalSold += l.sold
		totalDemand += l.demand
		totalBlock += l.blockMin
		totalFees += l.fees
	}

	legsPerTrip := float64(len(legs))
	avgRevenuePerLeg := totalRevenue / legsPerTrip
	avgCostPerLeg := totalCost / legsPerTrip
	avgFeesPerLeg := totalFees / legsPerTrip
	loadFactor := float64(totalSold) / float64(ac.Seats*len(legs))

	profitPerTick := (totalRevenue - totalCost) * float64(freq)
	curfewBlocked := fromAp.Curfew || toAp.Curfew
	if hasVia && viaAp.Curfew {
		curfewBlocked = true
	}

	avgPricePerSeat := 0.0
	if totalSold > 0 {
		avgPricePerSeat = totalRevenue / float64(totalSold)
	}

	route := Route{
		ID:                strconv.FormatInt(time.Now().UnixNano(), 10),
		From:              fromID,
		To:                toID,
		Via:               viaID,
		AircraftID:        ac.ID,
		FrequencyPerDay:   freq,
		EstimatedDemand:   totalDemand,
		PricePerSeat:      avgPricePerSeat,
		EstRevenueTick:    totalRevenue * float64(freq),
		EstCostTick:       totalCost * float64(freq),
		LoadFactor:        loadFactor,
		RevenuePerLeg:     avgRevenuePerLeg,
		CostPerLeg:        avgCostPerLeg,
		LandingFeesPerLeg: avgFeesPerLeg,
		ProfitPerTick:     profitPerTick,
		SeatsSoldPerLeg:   totalSold / len(legs),
		BlockMinutes:      totalBlock,
		CurfewBlocked:     curfewBlocked,
		LastTickRevenue:   totalRevenue * float64(freq),
		LastTickLoad:      loadFactor,
	}
	return route, nil
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func marketKey(a, b string) string {
	a = strings.ToUpper(a)
	b = strings.ToUpper(b)
	if a < b {
		return a + "-" + b
	}
	return b + "-" + a
}

func validateCapacityLocked(route Route) error {
	activeCount := 0
	for _, ac := range state.Fleet {
		if ac.TemplateID == route.AircraftID && ac.Status == "active" {
			activeCount++
		}
	}
	if activeCount == 0 {
		return http.ErrBodyNotAllowed // no available aircraft of that type
	}

	totalMins := route.BlockMinutes * float64(route.FrequencyPerDay)
	for _, rt := range state.Routes {
		if rt.AircraftID == route.AircraftID {
			totalMins += rt.BlockMinutes * float64(rt.FrequencyPerDay)
		}
	}
	// capacity in minutes per day
	if totalMins > float64(activeCount)*960.0 {
		return fmt.Errorf("insufficient aircraft time (over 16h/day for %s fleet)", route.AircraftID)
	}

	addSlotUse := func(ident string, freq int, slotUse map[string]int) {
		if ident == "" || freq == 0 {
			return
		}
		slotUse[strings.ToUpper(ident)] += freq
	}

	// slot constraints per airport
	slotUse := make(map[string]int)
	addSlotUse(route.From, route.FrequencyPerDay, slotUse)
	addSlotUse(route.To, route.FrequencyPerDay, slotUse)
	addSlotUse(route.Via, route.FrequencyPerDay, slotUse)
	for _, rt := range state.Routes {
		addSlotUse(rt.From, rt.FrequencyPerDay, slotUse)
		addSlotUse(rt.To, rt.FrequencyPerDay, slotUse)
		addSlotUse(rt.Via, rt.FrequencyPerDay, slotUse)
	}
	for ident, used := range slotUse {
		if ap, ok := airportsByIdent[ident]; ok && ap.SlotsPerDay > 0 && used > ap.SlotsPerDay {
			return fmt.Errorf("slot limit exceeded at %s (%d/%d)", ident, used, ap.SlotsPerDay)
		}
	}

	// curfew: ensure total block minutes at airport fits within allowed hours
	blockUse := make(map[string]float64)
	addBlockUse := func(ident string, mins float64, freq int, blockUse map[string]float64) {
		if ident == "" || freq == 0 || mins <= 0 {
			return
		}
		blockUse[strings.ToUpper(ident)] += mins * float64(freq)
	}
	// include new route usage
	addBlockUse(route.From, route.BlockMinutes, route.FrequencyPerDay, blockUse)
	addBlockUse(route.To, route.BlockMinutes, route.FrequencyPerDay, blockUse)
	addBlockUse(route.Via, route.BlockMinutes, route.FrequencyPerDay, blockUse)
	for _, rt := range state.Routes {
		addBlockUse(rt.From, rt.BlockMinutes, rt.FrequencyPerDay, blockUse)
		addBlockUse(rt.To, rt.BlockMinutes, rt.FrequencyPerDay, blockUse)
		addBlockUse(rt.Via, rt.BlockMinutes, rt.FrequencyPerDay, blockUse)
	}
	for ident, mins := range blockUse {
		ap, ok := airportsByIdent[ident]
		if !ok || !ap.Curfew {
			continue
		}
		avail := curfewAvailableMinutes(ap.CurfewStart, ap.CurfewEnd)
		if mins > avail {
			return fmt.Errorf("curfew hours limit at %s (%.0f/%.0f mins)", ident, mins, avail)
		}
	}
	return nil
}

func marketExistsLocked(from, to string) bool {
	key := marketKey(from, to)
	for _, rt := range state.Routes {
		if marketKey(rt.From, rt.To) == key {
			return true
		}
	}
	return false
}

func demandEstimate(fromAp, toAp Airport, ac Aircraft, freq int) int {
	return demandEstimateWithOpts(fromAp, toAp, ac, freq, demandOptions{})
}

type demandOptions struct {
	Stopover    bool
	PriceFactor float64
}

func demandEstimateWithOpts(fromAp, toAp Airport, ac Aircraft, freq int, opts demandOptions) int {
	dist := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	base := 60 + int(dist/45)
	if base < 35 {
		base = 35
	}
	if base > ac.Seats*3 {
		base = ac.Seats * 3
	}
	priceFactor := 1.0
	if opts.PriceFactor > 0 {
		priceFactor = opts.PriceFactor
	}
	price := 0.13 * dist * priceFactor
	priceElasticity := math.Exp(-price / 8000.0)
	freqBoost := 1.0 + (float64(freq-1) * 0.08)
	d := int(float64(base) * priceElasticity * freqBoost)
	if opts.Stopover {
		penalty := 0.8
		if priceFactor <= 0.85 {
			penalty = 1.0
		}
		d = int(float64(d) * penalty)
	}
	if d < 20 {
		d = 20
	}
	return d
}

func findAircraft(id string) (Aircraft, error) {
	for _, a := range aircraftCatalog {
		if strings.EqualFold(a.ID, id) {
			return a, nil
		}
	}
	return Aircraft{}, http.ErrMissingFile
}

func blockTimeMinutes(distanceKm, cruiseKmh float64, turnaroundMin int) float64 {
	if cruiseKmh <= 0 {
		return 0
	}
	flightHours := distanceKm / cruiseKmh
	return (flightHours * 60.0 * 2) + float64(turnaroundMin) // out and back plus turnaround
}

func maintenanceCost(condition float64) float64 {
	deficit := 100 - condition
	if deficit < 5 {
		deficit = 5
	}
	return deficit * 75_000
}

func maxTemplateUtilization(templateID string) float64 {
	maxUtil := 0.0
	for _, ac := range state.Fleet {
		if ac.TemplateID == templateID && ac.Status == "active" {
			if ac.Utilization > maxUtil {
				maxUtil = ac.Utilization
			}
		}
	}
	return maxUtil
}

func advanceFleetTimersLocked() {
	for i := range state.Fleet {
		ac := &state.Fleet[i]
		if ac.AvailableIn > 0 {
			ac.AvailableIn--
			if ac.AvailableIn <= 0 {
				ac.AvailableIn = 0
				if ac.Status == "delivering" || ac.Status == "maintenance" {
					ac.Status = "active"
				}
			}
		}
	}
}

func applyMaintenanceWearLocked() {
	for i := range state.Fleet {
		ac := &state.Fleet[i]
		if ac.Status != "active" || ac.Condition <= 0 {
			continue
		}
		wear := 0.05 + (ac.Utilization/100.0)*0.4
		ac.Condition -= wear
		if ac.Condition < 0 {
			ac.Condition = 0
		}
		if ac.Condition < 50 {
			chance := ((50 - ac.Condition) / 50.0) * 0.25
			if chance > 0 && rng.Float64() < chance {
				beginMaintenanceLocked(ac, 3+rng.Intn(3))
			}
		}
	}
}

func beginMaintenanceLocked(ac *OwnedCraft, ticks int) {
	if ticks < 1 {
		ticks = 1
	}
	ac.Status = "maintenance"
	ac.AvailableIn = ticks
}

// recalcUtilizationLocked recomputes utilization for each owned aircraft based on assigned routes.
func recalcUtilizationLocked() {
	// map templateID -> total minutes scheduled
	scheduled := make(map[string]float64)
	for _, rt := range state.Routes {
		mins := rt.BlockMinutes * float64(rt.FrequencyPerDay)
		scheduled[rt.AircraftID] += mins
	}
	countByTemplate := make(map[string]int)
	for _, ac := range state.Fleet {
		if ac.Status == "active" {
			countByTemplate[ac.TemplateID]++
		}
	}
	for i := range state.Fleet {
		ac := &state.Fleet[i]
		// assume a 16-hour operating day (960 minutes)
		util := 0.0
		if totalMins, ok := scheduled[ac.TemplateID]; ok {
			activeCount := countByTemplate[ac.TemplateID]
			if activeCount > 0 {
				util = (totalMins / (960.0 * float64(activeCount))) * 100.0
				if util > 100 {
					util = 100
				}
			}
		}
		ac.Utilization = util
	}
}

func advanceTickLocked() {
	totalRevenue := 0.0
	totalCost := 0.0
	for i := range state.Routes {
		rt := &state.Routes[i]
		ac, err := findAircraft(rt.AircraftID)
		if err != nil {
			continue
		}
		routeCost := rt.EstCostTick
		totalCost += routeCost

		randomFactor := 0.85 + rng.Float64()*0.3
		capacity := float64(ac.Seats * maxInt(1, rt.FrequencyPerDay))
		if capacity <= 0 {
			capacity = float64(ac.Seats)
		}
		demand := float64(rt.EstimatedDemand) * randomFactor
		actualSold := math.Min(capacity, demand)
		actualRevenue := 0.0
		actualLoad := 0.0
		if capacity > 0 {
			actualLoad = actualSold / capacity
		}

		if maxTemplateUtilization(rt.AircraftID) > 100 && rng.Float64() < 0.2 {
			actualRevenue = 0
			actualLoad = 0
		} else {
			actualRevenue = actualSold * rt.PricePerSeat
		}

		rt.LastTickRevenue = actualRevenue
		rt.LastTickLoad = actualLoad
		rt.LoadFactor = actualLoad
		rt.EstRevenueTick = actualRevenue
		rt.ProfitPerTick = actualRevenue - routeCost

		totalRevenue += actualRevenue
	}
	state.Cash += totalRevenue - totalCost
	advanceFleetTimersLocked()
	applyMaintenanceWearLocked()
	state.Tick++
	if state.Tick%6 == 0 { // periodically decay utilization so it re-calculates with routes
		recalcUtilizationLocked()
	}
	if err := saveState(saveFilePath, &state); err != nil {
		log.Printf("failed to save state: %v", err)
	}
}

func intervalForSpeed(speed int) time.Duration {
	switch speed {
	case 1:
		return 2 * time.Second
	case 2:
		return 1 * time.Second
	case 3:
		return 500 * time.Millisecond
	case 4:
		return 250 * time.Millisecond
	default:
		return 2 * time.Second
	}
}

func startSim(speed int) {
	if speed < 1 {
		speed = 1
	}
	if speed > 4 {
		speed = 4
	}
	interval := intervalForSpeed(speed)

	stateMu.Lock()
	state.Speed = speed
	state.IsRunning = true
	stateMu.Unlock()

	if simTicker == nil {
		simTicker = time.NewTicker(interval)
	} else {
		simTicker.Reset(interval)
	}
	if simCancel != nil {
		simCancel()
	}
	simCtx, simCancel = context.WithCancel(context.Background())

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-simTicker.C:
				stateMu.Lock()
				advanceTickLocked()
				stateMu.Unlock()
			}
		}
	}(simCtx)
}

func pauseSim() {
	stateMu.Lock()
	state.IsRunning = false
	stateMu.Unlock()
	if simCancel != nil {
		simCancel()
		simCancel = nil
	}
	if simTicker != nil {
		simTicker.Stop()
		simTicker = nil
	}
}
