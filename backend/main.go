package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
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
	RangeKm       float64 `json:"range_km"`
	Seats         int     `json:"seats"`
	CruiseKmh     float64 `json:"cruise_kmh"`
	FuelCost      float64 `json:"fuel_cost_per_km"`
	TurnaroundMin int     `json:"turnaround_min"`
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
	RangeKm       float64 `json:"range_km"`
	Seats         int     `json:"seats"`
	CruiseKmh     float64 `json:"cruise_kmh"`
	FuelCost      float64 `json:"fuel_cost_per_km"`
	TurnaroundMin int     `json:"turnaround_min"`
	Status        string  `json:"status"` // active, delivering, maintenance
	AvailableIn   int     `json:"available_in_ticks"`
	Utilization   float64 `json:"utilization_pct"`
}

// acquisition configuration
var (
	aircraftCosts = map[string]float64{
		"A320": 98_000_000,
		"B738": 96_000_000,
		"E190": 52_000_000,
	}
	aircraftLeadTicks = map[string]int{
		"A320": 8,
		"B738": 8,
		"E190": 6,
	}
)

var (
	simCtx    context.Context
	simCancel context.CancelFunc
	simTicker *time.Ticker
)

func seedFleet() []OwnedCraft {
	out := make([]OwnedCraft, 0, len(aircraftCatalog))
	for _, ac := range aircraftCatalog {
		out = append(out, OwnedCraft{
			ID:            ac.ID + "-1",
			TemplateID:    ac.ID,
			Name:          ac.Name,
			RangeKm:       ac.RangeKm,
			Seats:         ac.Seats,
			CruiseKmh:     ac.CruiseKmh,
			FuelCost:      ac.FuelCost,
			TurnaroundMin: ac.TurnaroundMin,
			Status:        "active",
			AvailableIn:   0,
			Utilization:   0,
		})
	}
	return out
}

var (
	store           *AirportStore
	airportsByIdent map[string]Airport
	stateMu         sync.Mutex
	state           GameState
	aircraftCatalog = []Aircraft{
		{ID: "A320", Name: "Airbus A320", RangeKm: 6100, Seats: 180, CruiseKmh: 830, FuelCost: 4.2, TurnaroundMin: 45},
		{ID: "B738", Name: "Boeing 737-800", RangeKm: 5765, Seats: 175, CruiseKmh: 840, FuelCost: 4.0, TurnaroundMin: 45},
		{ID: "E190", Name: "Embraer E190", RangeKm: 4500, Seats: 100, CruiseKmh: 820, FuelCost: 2.8, TurnaroundMin: 35},
	}
	runwayReqMeters = map[string]int{
		"A320": 1800,
		"B738": 1800,
		"E190": 1400,
	}
	curfewAppliesTo = map[string]bool{
		"large_airport":  true,
		"medium_airport": true,
	}
)

func main() {
	var err error
	store, err = loadAirports("data/airports.csv")
	if err != nil {
		log.Fatalf("failed to load airports: %v", err)
	}
	airportsByIdent = make(map[string]Airport, len(store.Airports))
	for _, a := range store.Airports {
		airportsByIdent[strings.ToUpper(a.Ident)] = a
	}

	state = GameState{
		Cash:  500_000_000, // starting cash
		Fleet: seedFleet(),
		Speed: 1,
	}

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
			RangeKm:       ac.RangeKm,
			Seats:         ac.Seats,
			CruiseKmh:     ac.CruiseKmh,
			FuelCost:      ac.FuelCost,
			TurnaroundMin: ac.TurnaroundMin,
			Status:        "delivering",
			AvailableIn:   lead,
			Utilization:   0,
		}
		state.Fleet = append(state.Fleet, newCraft)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newCraft)
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

	demandLeg := func(a, b Airport) int {
		return demandEstimate(a, b, ac, freq)
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
		d1 := demandLeg(fromAp, viaAp) + demandLeg(fromAp, toAp)
		d2 := demandLeg(viaAp, toAp)
		// Inbound: to->via with local + through demand, then via->from
		d3 := demandLeg(toAp, viaAp) + demandLeg(toAp, fromAp)
		d4 := demandLeg(viaAp, fromAp)

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
			{distMain, demandLeg(fromAp, toAp), fromAp, toAp},
			{distMain, demandLeg(toAp, fromAp), toAp, fromAp},
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

	// slot constraints per airport
	slotUse := make(map[string]int)
	slotUse[strings.ToUpper(route.From)] += route.FrequencyPerDay
	slotUse[strings.ToUpper(route.To)] += route.FrequencyPerDay
	for _, rt := range state.Routes {
		slotUse[strings.ToUpper(rt.From)] += rt.FrequencyPerDay
		slotUse[strings.ToUpper(rt.To)] += rt.FrequencyPerDay
	}
	for ident, used := range slotUse {
		if ap, ok := airportsByIdent[ident]; ok && ap.SlotsPerDay > 0 && used > ap.SlotsPerDay {
			return fmt.Errorf("slot limit exceeded at %s (%d/%d)", ident, used, ap.SlotsPerDay)
		}
	}

	// curfew: ensure total block minutes at airport fits within allowed hours
	blockUse := make(map[string]float64)
	// include new route usage
	blockUse[strings.ToUpper(route.From)] += route.BlockMinutes * float64(route.FrequencyPerDay)
	blockUse[strings.ToUpper(route.To)] += route.BlockMinutes * float64(route.FrequencyPerDay)
	for _, rt := range state.Routes {
		blockUse[strings.ToUpper(rt.From)] += rt.BlockMinutes * float64(rt.FrequencyPerDay)
		blockUse[strings.ToUpper(rt.To)] += rt.BlockMinutes * float64(rt.FrequencyPerDay)
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
	dist := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	base := 60 + int(dist/45)
	if base < 35 {
		base = 35
	}
	if base > ac.Seats*3 {
		base = ac.Seats * 3
	}
	price := 0.13 * dist
	priceElasticity := math.Exp(-price / 8000.0)
	freqBoost := 1.0 + (float64(freq-1) * 0.08)
	d := int(float64(base) * priceElasticity * freqBoost)
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
		if ac.AvailableIn > 0 {
			ac.AvailableIn--
			if ac.AvailableIn == 0 {
				ac.Status = "active"
			}
		}
	}
}

func advanceTickLocked() {
	revenue := 0.0
	cost := 0.0
	for _, rt := range state.Routes {
		revenue += rt.EstRevenueTick
		cost += rt.EstCostTick
	}
	state.Cash += revenue - cost
	state.Tick++
	if state.Tick%6 == 0 { // periodically decay utilization so it re-calculates with routes
		recalcUtilizationLocked()
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
