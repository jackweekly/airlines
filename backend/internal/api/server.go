package api

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"

	"airline_builder/internal/game"
	"airline_builder/internal/models"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	engine *game.Engine
}

// New constructs the HTTP router wired to the game engine.
func New(engine *game.Engine) http.Handler {
	s := &Server{engine: engine}
	r := chi.NewRouter()
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/airports", s.handleAirports)
	r.Get("/aircraft/templates", s.handleAircraftTemplates)
	r.Get("/state", s.handleState)
	r.Post("/routes", s.handleCreateRoute)
	r.Post("/tick", s.handleTick)
	r.Post("/sim/start", s.handleSimStart)
	r.Post("/sim/pause", s.handleSimPause)
	r.Post("/sim/speed", s.handleSimSpeed)
	r.Post("/fleet/purchase", s.handlePurchase)
	r.Post("/fleet/maintenance", s.handleMaintenance)
	r.Post("/analysis/route", s.handleRouteAnalysis)

	return r
}

func (s *Server) handleAirports(w http.ResponseWriter, r *http.Request) {
	tier := r.URL.Query().Get("tier")
	fields := r.URL.Query().Get("fields")
	filtered := filterAirports(s.engine.Airports(), tier)
	w.Header().Set("Content-Type", "application/json")
	if strings.EqualFold(fields, "basic") {
		basic := make([]map[string]interface{}, 0, len(filtered))
		for _, a := range filtered {
			basic = append(basic, map[string]interface{}{
				"id": a.ID, "ident": a.Ident, "name": a.Name,
				"lat": a.Latitude, "lon": a.Longitude,
				"type": a.Type, "iata": a.IATA, "icao": a.ICAO,
			})
		}
		_ = json.NewEncoder(w).Encode(basic)
		return
	}
	_ = json.NewEncoder(w).Encode(filtered)
}

func (s *Server) handleAircraftTemplates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.Aircraft())
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.State())
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From       string  `json:"from"`
		To         string  `json:"to"`
		Via        string  `json:"via,omitempty"`
		AircraftID string  `json:"aircraft_id"`
		Frequency  int     `json:"frequency_per_day"`
		OneWay     bool    `json:"one_way"`
		UserPrice  float64 `json:"user_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad request")
		return
	}

	route, err := s.engine.BuildRoute(req.From, req.To, req.Via, req.AircraftID, req.Frequency, req.UserPrice)
	if err != nil {
		msg := err.Error()
		if err == http.ErrBodyNotAllowed {
			msg = "route not feasible (range/runway/legs)"
		}
		writeJSONError(w, http.StatusBadRequest, msg)
		return
	}
	if !req.OneWay && s.engine.MarketExists(route.From, route.To) {
		writeJSONError(w, http.StatusBadRequest, "market already served in either direction")
		return
	}
	if err := s.engine.ValidateCapacity(route); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.engine.AddRoute(route)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(route)
}

func (s *Server) handleTick(w http.ResponseWriter, r *http.Request) {
	s.engine.AdvanceTick()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.State())
}

func (s *Server) handleSimStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Speed int `json:"speed"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.engine.StartSim(req.Speed)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.State())
}

func (s *Server) handleSimPause(w http.ResponseWriter, r *http.Request) {
	s.engine.PauseSim()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.State())
}

func (s *Server) handleSimSpeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Speed int `json:"speed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Speed <= 0 {
		writeJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.engine.SetSpeed(req.Speed)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.State())
}

func (s *Server) handlePurchase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TemplateID string `json:"template_id"`
		Mode       string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TemplateID == "" {
		writeJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	craft, err := s.engine.PurchaseAircraft(req.TemplateID, req.Mode)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(craft)
}

func (s *Server) handleMaintenance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OwnedID string `json:"owned_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OwnedID == "" {
		writeJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	craft, err := s.engine.Maintain(req.OwnedID, 3)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(craft)
}

type RouteAnalysisRequest struct {
	Origin        string   `json:"origin"`
	Dest          string   `json:"dest"`
	Via           string   `json:"via"`
	AircraftTypes []string `json:"aircraft_types"`
}

type RouteAnalysisResult struct {
	AircraftType string  `json:"aircraft_type"`
	Frequency    float64 `json:"frequency"`
	LoadFactor   float64 `json:"load_factor"`
	DailyProfit  float64 `json:"daily_profit"`
	RoiScore     float64 `json:"roi_score"`
	Valid        bool    `json:"valid"`
	Error        string  `json:"error,omitempty"`
}

func (s *Server) handleRouteAnalysis(w http.ResponseWriter, r *http.Request) {
	var req RouteAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	fromAp, ok1 := s.engine.AirportByIdent(req.Origin)
	toAp, ok2 := s.engine.AirportByIdent(req.Dest)
	if !ok1 || !ok2 {
		writeJSONError(w, http.StatusBadRequest, "invalid airports")
		return
	}

	var viaAp models.Airport
	hasVia := strings.TrimSpace(req.Via) != ""
	if hasVia {
		viaAp, ok1 = s.engine.AirportByIdent(req.Via)
		if !ok1 {
			writeJSONError(w, http.StatusBadRequest, "invalid via airport")
			return
		}
	}

	distDirect := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	if distDirect <= 0 {
		writeJSONError(w, http.StatusBadRequest, "origin and destination must differ")
		return
	}

	distLeg1 := distDirect
	distLeg2 := 0.0
	if hasVia {
		distLeg1 = haversine(fromAp.Latitude, fromAp.Longitude, viaAp.Latitude, viaAp.Longitude)
		distLeg2 = haversine(viaAp.Latitude, viaAp.Longitude, toAp.Latitude, toAp.Longitude)
	}
	totalDist := distLeg1 + distLeg2

	results := []RouteAnalysisResult{}

	benchmarkSpeed := 850.0
	directTimeHours := distDirect / benchmarkSpeed

	for _, typeID := range req.AircraftTypes {
		ac, err := findAircraft(typeID, s.engine.Aircraft())
		if err != nil {
			results = append(results, RouteAnalysisResult{AircraftType: typeID, Valid: false, Error: "Unknown Type"})
			continue
		}

		if distLeg1 > ac.RangeKm || distLeg2 > ac.RangeKm {
			results = append(results, RouteAnalysisResult{AircraftType: typeID, Valid: false, Error: "Range Exceeded"})
			continue
		}

		blockSpeed := ac.CruiseKmh * 0.9
		if blockSpeed <= 0 {
			blockSpeed = 100
		}

		flightTimeHours := totalDist / blockSpeed

		totalTravelTime := flightTimeHours
		if hasVia {
			totalTravelTime += float64(ac.TurnaroundMin) / 60.0
		}

		extraHours := totalTravelTime - directTimeHours
		if extraHours < 0 {
			extraHours = 0
		}
		penalty := extraHours * 0.10
		demandFactor := 1.0 - penalty
		if demandFactor < 0.1 {
			demandFactor = 0.1
		}

		baseDemand := float64(s.engine.DemandEstimate(fromAp, toAp, ac, 1))
		adjustedDemand := baseDemand * demandFactor

		oneWayBlock := totalTravelTime*60 + float64(ac.TurnaroundMin)
		roundTripTimeMins := oneWayBlock * 2

		maxFreq := (24 * 60) / roundTripTimeMins
		if maxFreq < 1 {
			maxFreq = 1
		}
		freq := math.Floor(maxFreq)
		if freq < 1 {
			freq = 1
		}

		load := adjustedDemand
		if load > float64(ac.Seats) {
			load = float64(ac.Seats)
		}
		loadFactor := load / float64(ac.Seats)

		price := 0.13 * totalDist
		if price < 50 {
			price = 50
		}

		revenuePerFlight := load * price
		fuelCost := totalDist * ac.FuelCost
		landingFees := toAp.LandingFee
		if hasVia {
			landingFees += viaAp.LandingFee
		}

		crewCost := 1000.0
		costPerFlight := fuelCost + landingFees + crewCost

		profitPerFlight := revenuePerFlight - costPerFlight
		dailyProfit := profitPerFlight * freq

		costToBuy := s.engine.AircraftCosts()[ac.ID]
		if costToBuy == 0 {
			costToBuy = 50_000_000
		}

		roi := 0.0
		if costToBuy > 0 && math.IsInf(dailyProfit, 0) == false {
			roi = (dailyProfit * 365) / costToBuy * 100
		}

		results = append(results, RouteAnalysisResult{
			AircraftType: ac.ID,
			Frequency:    freq,
			LoadFactor:   loadFactor * 100,
			DailyProfit:  dailyProfit,
			RoiScore:     roi,
			Valid:        true,
		})
	}

	// limit to top 5 by profit
	if len(results) > 1 {
		sort.Slice(results, func(i, j int) bool {
			return results[i].DailyProfit > results[j].DailyProfit
		})
		if len(results) > 5 {
			results = results[:5]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

// ===== helpers =====

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	if msg == "" {
		msg = http.StatusText(status)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func filterAirports(all []models.Airport, tier string) []models.Airport {
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
	out := make([]models.Airport, 0, len(all))
	for _, a := range all {
		if keep(a.Type) {
			out = append(out, a)
		}
	}
	return out
}

func findAircraft(id string, list []models.Aircraft) (models.Aircraft, error) {
	for _, a := range list {
		if strings.EqualFold(a.ID, id) {
			return a, nil
		}
	}
	return models.Aircraft{}, os.ErrNotExist
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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
