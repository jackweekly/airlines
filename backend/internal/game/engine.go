package game

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

	"airline_builder/internal/models"
)

const (
	savePath               = "data/savegame.json"
	manualMaintenanceTicks = 3
)

// Engine owns simulation state and logic.
type Engine struct {
	mu            sync.Mutex
	state         models.GameState
	aircraft      []models.Aircraft
	airports      []models.Airport
	byIdent       map[string]models.Airport
	rng           *rand.Rand
	aircraftCosts map[string]float64
	aircraftLead  map[string]int
	ctx           context.Context
	cancel        context.CancelFunc
	ticker        *time.Ticker
	savePath      string
}

func NewEngine(costs map[string]float64, leads map[string]int) *Engine {
	return &Engine{
		byIdent:       make(map[string]models.Airport),
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		aircraftCosts: costs,
		aircraftLead:  leads,
		savePath:      savePath,
	}
}

// SetSavePath configures where the save file is written.
func (e *Engine) SetSavePath(path string) {
	e.savePath = path
}

func (e *Engine) SetAircraft(list []models.Aircraft) {
	e.aircraft = list
}

func (e *Engine) Aircraft() []models.Aircraft {
	return e.aircraft
}

func (e *Engine) AircraftCosts() map[string]float64 {
	return e.aircraftCosts
}

func (e *Engine) AircraftLeadTimes() map[string]int {
	return e.aircraftLead
}

// SetState replaces the current game state.
func (e *Engine) SetState(st models.GameState) {
	e.mu.Lock()
	e.state = st
	e.mu.Unlock()
}

func (e *Engine) SetAirports(list []models.Airport) {
	e.airports = list
	e.byIdent = make(map[string]models.Airport, len(list))
	for _, a := range list {
		e.byIdent[strings.ToUpper(a.Ident)] = a
	}
}

func (e *Engine) Airports() []models.Airport {
	return e.airports
}

// AirportByIdent returns an airport by ident (IATA/ICAO).
func (e *Engine) AirportByIdent(ident string) (models.Airport, bool) {
	ident = strings.ToUpper(strings.TrimSpace(ident))
	ap, ok := e.byIdent[ident]
	return ap, ok
}

func (e *Engine) State() models.GameState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// LoadAirportsCSV parses an airports CSV and populates the engine.
func (e *Engine) LoadAirportsCSV(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return err
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

	var airports []models.Airport
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

		airports = append(airports, models.Airport{
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

	e.SetAirports(airports)
	log.Printf("loaded %d airports", len(airports))
	return nil
}

// SeedFleet creates the starter fleet into the engine state.
func (e *Engine) SeedFleet() {
	e.mu.Lock()
	defer e.mu.Unlock()
	starter := map[string]bool{"A320": true, "B737-800": true, "E190": true}
	fleet := make([]models.OwnedCraft, 0, len(starter))
	for _, ac := range e.aircraft {
		if !starter[ac.ID] {
			continue
		}
		fleet = append(fleet, models.OwnedCraft{
			ID:            ac.ID + "-1",
			TemplateID:    ac.ID,
			Name:          ac.Name,
			Role:          ac.Role,
			RangeKm:       ac.RangeKm,
			Seats:         ac.Seats,
			CruiseKmh:     ac.CruiseKmh,
			FuelCost:      ac.FuelCost,
			TurnaroundMin: ac.TurnaroundMin,
			Status:        "active",
			AvailableIn:   0,
			Utilization:   0,
			Condition:     100,
			OwnershipType: "owned",
			MonthlyCost:   0,
			State:         models.AircraftIdle,
		})
	}
	e.state.Fleet = fleet
}

// SaveState persists the current state to disk.
func (e *Engine) SaveState(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if path == "" {
		path = e.savePath
	}
	data, err := json.MarshalIndent(&e.state, "", "  ")
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

// LoadState loads state from disk if present.
func (e *Engine) LoadState(path string) error {
	if path == "" {
		path = e.savePath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var st models.GameState
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	for i := range st.Fleet {
		if st.Fleet[i].OwnershipType == "" {
			st.Fleet[i].OwnershipType = "owned"
		}
		if st.Fleet[i].State == "" {
			st.Fleet[i].State = models.AircraftIdle
		}
		if st.Fleet[i].TurnaroundMin == 0 {
			if tpl, err := e.findAircraft(st.Fleet[i].TemplateID); err == nil && tpl.TurnaroundMin > 0 {
				st.Fleet[i].TurnaroundMin = tpl.TurnaroundMin
			} else {
				st.Fleet[i].TurnaroundMin = 30
			}
		}
	}
	if st.DemandVariability <= 0 {
		st.DemandVariability = 0.08
	}
	if st.RecentEvents == nil {
		st.RecentEvents = []string{}
	}
	e.mu.Lock()
	e.state = st
	e.mu.Unlock()
	return nil
}

// AddRoute appends a new route and refreshes utilization.
func (e *Engine) AddRoute(route models.Route) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Routes = append(e.state.Routes, route)
	e.addEventLocked(fmt.Sprintf("Route %s-%s created", strings.ToUpper(route.From), strings.ToUpper(route.To)))
	e.recalcUtilizationLocked()
}

// PurchaseAircraft adds a new aircraft order or purchase.
func (e *Engine) PurchaseAircraft(templateID, mode string) (models.OwnedCraft, error) {
	ac, err := e.findAircraft(templateID)
	if err != nil {
		return models.OwnedCraft{}, err
	}

	cost := e.aircraftCosts[ac.ID]
	if cost <= 0 {
		cost = 75_000_000
	}
	lead := e.aircraftLead[ac.ID]
	if lead <= 0 {
		lead = 6
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "buy"
	}
	var upfront float64
	var monthly float64
	ownershipType := "owned"
	switch mode {
	case "lease":
		ownershipType = "leased"
		upfront = cost * 0.02
		monthly = cost * 0.01
	default:
		upfront = cost
	}
	if upfront <= 0 {
		upfront = cost
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state.Cash < upfront {
		return models.OwnedCraft{}, fmt.Errorf("insufficient cash")
	}
	e.state.Cash -= upfront
	newCraft := models.OwnedCraft{
		ID:            ac.ID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		TemplateID:    ac.ID,
		Name:          ac.Name,
		Role:          ac.Role,
		RangeKm:       ac.RangeKm,
		Seats:         ac.Seats,
		CruiseKmh:     ac.CruiseKmh,
		FuelCost:      ac.FuelCost,
		TurnaroundMin: ac.TurnaroundMin,
		Status:        "delivering",
		AvailableIn:   lead,
		Utilization:   0,
		Condition:     100,
		OwnershipType: ownershipType,
		MonthlyCost:   monthly,
		State:         models.AircraftIdle,
	}
	e.state.Fleet = append(e.state.Fleet, newCraft)
	e.addEventLocked(fmt.Sprintf("Ordered %s (%s)", newCraft.Name, newCraft.ID))
	return newCraft, nil
}

// Maintain performs manual maintenance on an aircraft.
func (e *Engine) Maintain(ownedID string, manualTicks int) (*models.OwnedCraft, error) {
	if manualTicks < 1 {
		manualTicks = 3
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var craft *models.OwnedCraft
	for i := range e.state.Fleet {
		if strings.EqualFold(e.state.Fleet[i].ID, ownedID) {
			craft = &e.state.Fleet[i]
			break
		}
	}
	if craft == nil {
		return nil, fmt.Errorf("unknown aircraft")
	}
	if craft.Status == "delivering" {
		return nil, fmt.Errorf("aircraft still delivering")
	}
	if e.state.Cash <= 0 {
		return nil, fmt.Errorf("insufficient cash")
	}
	cost := maintenanceCost(craft.Condition)
	if e.state.Cash < cost {
		return nil, fmt.Errorf("insufficient cash")
	}
	e.state.Cash -= cost
	craft.Condition = 100
	craft.State = models.AircraftIdle
	craft.Status = "active"
	e.beginMaintenanceLocked(craft, manualTicks)
	e.addEventLocked(fmt.Sprintf("%s sent to maintenance", craft.Name))
	return craft, nil
}

// SetSpeed updates the simulation speed without starting it.
func (e *Engine) SetSpeed(speed int) {
	if speed < 1 {
		speed = 1
	}
	if speed > 4 {
		speed = 4
	}
	e.mu.Lock()
	e.state.Speed = speed
	e.mu.Unlock()
	if e.state.IsRunning {
		e.startSim(speed)
	}
}

// StartSim starts the ticker loop.
func (e *Engine) StartSim(speed int) {
	if speed <= 0 {
		speed = e.state.Speed
		if speed <= 0 {
			speed = 1
		}
	}
	e.startSim(speed)
}

func (e *Engine) startSim(speed int) {
	if speed < 1 {
		speed = 1
	}
	if speed > 4 {
		speed = 4
	}
	interval := intervalForSpeed(speed)

	e.mu.Lock()
	e.state.Speed = speed
	e.state.IsRunning = true
	e.mu.Unlock()

	if e.ticker == nil {
		e.ticker = time.NewTicker(interval)
	} else {
		e.ticker.Reset(interval)
	}
	if e.cancel != nil {
		e.cancel()
	}
	e.ctx, e.cancel = context.WithCancel(context.Background())

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.ticker.C:
				e.AdvanceTick()
			}
		}
	}(e.ctx)
}

// PauseSim stops the ticker loop.
func (e *Engine) PauseSim() {
	e.mu.Lock()
	e.state.IsRunning = false
	e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	if e.ticker != nil {
		e.ticker.Stop()
		e.ticker = nil
	}
}

// BuildRoute calculates a new route using current aircraft and airports.
func (e *Engine) BuildRoute(from, to, via, aircraftID string, freq int, userPrice float64) (models.Route, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if freq <= 0 {
		freq = 1
	}
	fromID := strings.ToUpper(strings.TrimSpace(from))
	toID := strings.ToUpper(strings.TrimSpace(to))
	viaID := strings.ToUpper(strings.TrimSpace(via))

	fromAp, ok := e.byIdent[fromID]
	if !ok {
		return models.Route{}, fmt.Errorf("airport not found")
	}
	toAp, ok := e.byIdent[toID]
	if !ok {
		return models.Route{}, fmt.Errorf("airport not found")
	}
	var viaAp models.Airport
	var hasVia bool
	if viaID != "" {
		v, ok := e.byIdent[viaID]
		if !ok {
			return models.Route{}, fmt.Errorf("airport not found")
		}
		viaAp = v
		hasVia = true
	}

	ac, err := e.findAircraft(aircraftID)
	if err != nil {
		return models.Route{}, err
	}
	reqRunway := runwayReqMeters[ac.ID]
	if reqRunway == 0 {
		reqRunway = 1500
	}

	distMain := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	if distMain > ac.RangeKm {
		return models.Route{}, fmt.Errorf("route distance exceeds aircraft range of %.0f km", ac.RangeKm)
	}
	if fromAp.RunwayM < reqRunway || toAp.RunwayM < reqRunway {
		return models.Route{}, fmt.Errorf("runway too short for %s", ac.ID)
	}

	var distVia1, distVia2 float64
	if hasVia {
		distVia1 = haversine(fromAp.Latitude, fromAp.Longitude, viaAp.Latitude, viaAp.Longitude)
		distVia2 = haversine(viaAp.Latitude, viaAp.Longitude, toAp.Latitude, toAp.Longitude)
		if distVia1 > ac.RangeKm || distVia2 > ac.RangeKm {
			return models.Route{}, fmt.Errorf("route distance exceeds aircraft range of %.0f km", ac.RangeKm)
		}
		if viaAp.RunwayM < reqRunway {
			return models.Route{}, fmt.Errorf("%s runway too short for %s", viaAp.Ident, ac.ID)
		}
	}

	basePrice := 0.13 * distMain
	if basePrice <= 0 {
		totalVia := distVia1 + distVia2
		basePrice = 0.13 * totalVia
	}
	if basePrice <= 0 {
		basePrice = 150
	}
	if userPrice <= 0 {
		userPrice = basePrice
	}
	baseDistance := distMain
	if baseDistance <= 0 {
		baseDistance = distVia1 + distVia2
		if baseDistance <= 0 {
			baseDistance = 1
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

	demandLeg := func(a, b models.Airport, opts demandOptions) int {
		return e.demandEstimateWithOpts(a, b, ac, freq, opts)
	}
	priceForLeg := func(dist float64) float64 {
		if dist <= 0 || baseDistance <= 0 {
			return userPrice
		}
		p := userPrice * (dist / baseDistance)
		if p <= 0 {
			return userPrice
		}
		return p
	}

	legCost := func(a, b models.Airport, dist float64) (float64, float64) {
		fees := a.LandingFee + b.LandingFee
		return dist*ac.FuelCost + 800.0 + fees, fees
	}
	legBlock := func(dist float64) float64 {
		return (dist/ac.CruiseKmh)*60 + float64(ac.TurnaroundMin)
	}

	var legs []leg

	if hasVia {
		priceLeg1 := priceForLeg(distVia1)
		d1 := demandLeg(fromAp, viaAp, demandOptions{Price: priceLeg1}) + demandLeg(fromAp, toAp, demandOptions{Stopover: true, Price: userPrice})
		d2 := demandLeg(viaAp, toAp, demandOptions{Price: priceForLeg(distVia2)})
		d3 := demandLeg(toAp, viaAp, demandOptions{Price: priceForLeg(distVia2)}) + demandLeg(toAp, fromAp, demandOptions{Stopover: true, Price: userPrice})
		d4 := demandLeg(viaAp, fromAp, demandOptions{Price: priceLeg1})

		for _, x := range []struct {
			dist   float64
			demand int
			a      models.Airport
			b      models.Airport
		}{
			{distVia1, d1, fromAp, viaAp},
			{distVia2, d2, viaAp, toAp},
			{distVia2, d3, toAp, viaAp},
			{distVia1, d4, viaAp, fromAp},
		} {
			sold := min(x.demand, ac.Seats)
			price := priceForLeg(x.dist)
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
		for _, x := range []struct {
			dist   float64
			demand int
			a      models.Airport
			b      models.Airport
		}{
			{distMain, demandLeg(fromAp, toAp, demandOptions{Price: userPrice}), fromAp, toAp},
			{distMain, demandLeg(toAp, fromAp, demandOptions{Price: userPrice}), toAp, fromAp},
		} {
			sold := min(x.demand, ac.Seats)
			price := userPrice
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

	avgPricePerSeat := userPrice

	route := models.Route{
		ID:                strconv.FormatInt(time.Now().UnixNano(), 10),
		From:              fromID,
		To:                toID,
		Via:               viaID,
		AircraftID:        ac.ID,
		FrequencyPerDay:   freq,
		EstimatedDemand:   totalDemand,
		PricePerSeat:      avgPricePerSeat,
		UserPrice:         userPrice,
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

// AdvanceTick runs the simulation tick.
func (e *Engine) AdvanceTick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	totalRevenue := 0.0
	totalCost := 0.0

	findRouteForAc := func(acID string, tplID string) *models.Route {
		for i := range e.state.Routes {
			if strings.EqualFold(e.state.Routes[i].AircraftID, acID) {
				return &e.state.Routes[i]
			}
			if strings.EqualFold(e.state.Routes[i].AircraftID, tplID) {
				return &e.state.Routes[i]
			}
		}
		return nil
	}

	for i := range e.state.Fleet {
		ac := &e.state.Fleet[i]
		if ac.Status != "active" {
			continue
		}

		if ac.State == "" {
			ac.State = models.AircraftIdle
		}

		switch ac.State {
		case models.AircraftFlying:
			if ac.TimerMin > 0 {
				ac.TimerMin--
			}
			if ac.TimerMin <= 0 {
				if ac.FlightPlan != nil {
					ac.Location = strings.ToUpper(ac.FlightPlan.Dest)
				}
				// enforce a full turnaround before next leg
				ac.State = models.AircraftTurnaround
				ac.TimerMin = maxInt(1, ac.TurnaroundMin)
				ac.FlightPlan = nil
			}
		case models.AircraftTurnaround, models.AircraftIdle:
			if ac.State == models.AircraftTurnaround && ac.TimerMin > 0 {
				ac.TimerMin--
			} else {
				ac.TimerMin = 0
			}
			if ac.TimerMin > 0 {
				continue
			}
			rt := findRouteForAc(ac.ID, ac.TemplateID)
			if rt == nil {
				ac.State = models.AircraftIdle
				continue
			}
			origin, dest := e.nextLegForAircraft(ac, rt)
			if origin == "" || dest == "" {
				ac.State = models.AircraftIdle
				continue
			}
			durationMin, plan, revenue, cost := e.planFlightLeg(ac, rt, origin, dest)
			if plan == nil {
				ac.State = models.AircraftIdle
				continue
			}
			ac.State = models.AircraftFlying
			ac.FlightPlan = plan
			ac.Location = strings.ToUpper(origin)
			ac.TimerMin = maxInt(1, int(math.Ceil(durationMin)))

			totalRevenue += revenue
			totalCost += cost

			load := 0.0
			if ac.Seats > 0 {
				load = float64(plan.Passengers) / float64(ac.Seats)
				if load > 1 {
					load = 1
				}
			}
			rt.LastTickRevenue = revenue
			rt.LastTickLoad = load
			rt.LoadFactor = load
			rt.EstRevenueTick = revenue
			rt.EstCostTick = cost
			rt.ProfitPerTick = revenue - cost
		}
	}

	leaseCost := 0.0
	for _, ac := range e.state.Fleet {
		if strings.EqualFold(ac.OwnershipType, "leased") && ac.MonthlyCost > 0 {
			leaseCost += ac.MonthlyCost
		}
	}
	cashDelta := totalRevenue - totalCost - leaseCost
	e.state.Cash += cashDelta
	e.state.LastCashDelta = cashDelta

	e.advanceFleetTimersLocked()
	e.applyMaintenanceWearLocked()
	e.state.Tick++
	if e.state.Tick%6 == 0 {
		e.recalcUtilizationLocked()
	}
	_ = e.SaveState(e.savePath)
}

// ======================
// helper functions below

type demandOptions struct {
	Stopover bool
	Price    float64
}

func (e *Engine) findAircraft(id string) (models.Aircraft, error) {
	for _, a := range e.aircraft {
		if strings.EqualFold(a.ID, id) {
			return a, nil
		}
	}
	return models.Aircraft{}, fmt.Errorf("aircraft type not found")
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

func (e *Engine) demandEstimateWithOpts(fromAp, toAp models.Airport, ac models.Aircraft, freq int, opts demandOptions) int {
	airportWeight := func(t string) float64 {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "large_airport":
			return 1.6
		case "medium_airport":
			return 1.25
		case "small_airport":
			return 0.85
		default:
			return 0.95
		}
	}

	variability := e.state.DemandVariability
	if variability <= 0 {
		variability = 0.08
	}

	dist := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	base := 60 + int(dist/45)
	if base < 35 {
		base = 35
	}
	// Airports with more traffic generate more base demand.
	hubWeight := (airportWeight(fromAp.Type) + airportWeight(toAp.Type)) / 2
	base = int(float64(base) * hubWeight)
	if base > ac.Seats*3 {
		base = ac.Seats * 3
	}
	// Higher base fare makes cash flow easier to notice in UI.
	basePrice := 0.16 * dist
	price := basePrice
	if opts.Price > 0 {
		price = opts.Price
	}
	ratio := 1.0
	if basePrice > 0 {
		ratio = price / basePrice
	}
	priceElasticity := math.Exp(-3.0 * (ratio - 1.0))
	if priceElasticity < 0.1 {
		priceElasticity = 0.1
	}
	if priceElasticity > 2.5 {
		priceElasticity = 2.5
	}
	freqBoost := 1.0 + (float64(freq-1) * 0.08)
	d := int(float64(base) * priceElasticity * freqBoost)
	// Small regional equipment feels less attractive on long hauls.
	if dist > 2500 && (strings.Contains(strings.ToLower(ac.Role), "regional") || (ac.Seats > 0 && ac.Seats < 120)) {
		longPenalty := math.Min(0.35, (dist-2500)/8000) // cap penalty so it doesn't zero out demand
		d = int(float64(d) * (1 - longPenalty))
	}
	if opts.Stopover {
		d = int(float64(d) * 0.8)
	}
	// Add a little noise so demand is not static tick-to-tick.
	noise := 1 + ((e.rng.Float64()*2 - 1) * variability)
	if noise < 0.5 {
		noise = 0.5
	}
	d = int(float64(d) * noise)
	if d < 20 {
		d = 20
	}
	return d
}

func (e *Engine) nextLegForAircraft(ac *models.OwnedCraft, rt *models.Route) (string, string) {
	legs := routeLegs(rt)
	if len(legs) == 0 {
		return "", ""
	}
	if ac.RouteLegIndex < 0 {
		ac.RouteLegIndex = 0
	}
	location := strings.ToUpper(strings.TrimSpace(ac.Location))

	// First, try to find a leg that departs from the aircraft's current location.
	if location != "" {
		for idx, leg := range legs {
			if sameAirport(location, leg.Origin) {
				ac.RouteLegIndex = (idx + 1) % len(legs)
				return leg.Origin, leg.Dest
			}
		}
	}

	// Otherwise continue from the saved index (wrap around the legs list).
	start := ac.RouteLegIndex % len(legs)
	leg := legs[start]
	ac.RouteLegIndex = (start + 1) % len(legs)
	if location == "" {
		ac.Location = leg.Origin
	}
	return leg.Origin, leg.Dest
}

type routeLeg struct {
	Origin string
	Dest   string
}

func routeLegs(rt *models.Route) []routeLeg {
	fromID := strings.ToUpper(strings.TrimSpace(rt.From))
	toID := strings.ToUpper(strings.TrimSpace(rt.To))
	viaID := strings.ToUpper(strings.TrimSpace(rt.Via))
	if fromID == "" || toID == "" {
		return nil
	}
	if viaID == "" {
		return []routeLeg{
			{Origin: fromID, Dest: toID},
			{Origin: toID, Dest: fromID},
		}
	}
	return []routeLeg{
		{Origin: fromID, Dest: viaID},
		{Origin: viaID, Dest: toID},
		{Origin: toID, Dest: viaID},
		{Origin: viaID, Dest: fromID},
	}
}

func sameAirport(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (e *Engine) addEventLocked(msg string) {
	if msg == "" {
		return
	}
	e.state.RecentEvents = append(e.state.RecentEvents, msg)
	const maxEvents = 20
	if len(e.state.RecentEvents) > maxEvents {
		e.state.RecentEvents = e.state.RecentEvents[len(e.state.RecentEvents)-maxEvents:]
	}
}

func (e *Engine) planFlightLeg(ac *models.OwnedCraft, rt *models.Route, origin, dest string) (float64, *models.FlightPlan, float64, float64) {
	originID := strings.ToUpper(strings.TrimSpace(origin))
	destID := strings.ToUpper(strings.TrimSpace(dest))
	if originID == "" || destID == "" {
		return 0, nil, 0, 0
	}
	fromAp, ok := e.byIdent[originID]
	if !ok {
		return 0, nil, 0, 0
	}
	toAp, ok := e.byIdent[destID]
	if !ok {
		return 0, nil, 0, 0
	}
	dist := haversine(fromAp.Latitude, fromAp.Longitude, toAp.Latitude, toAp.Longitude)
	if dist <= 0 || (ac.RangeKm > 0 && dist > ac.RangeKm) {
		return 0, nil, 0, 0
	}
	if ac.CruiseKmh <= 0 {
		return 0, nil, 0, 0
	}
	price := rt.UserPrice
	if price <= 0 {
		price = rt.PricePerSeat
	}
	if price <= 0 {
		price = math.Max(220, 0.18*dist)
	}
	demand := e.demandEstimateWithOpts(fromAp, toAp, e.aircraftFromOwned(ac), rt.FrequencyPerDay, demandOptions{Price: price})
	if demand < 0 {
		demand = 0
	}
	sold := min(demand, ac.Seats)
	revenue := float64(sold) * price
	fees := fromAp.LandingFee + toAp.LandingFee
	// Nudge fuel/ops costs up a bit so net cash swings are visible but balanced.
	cost := dist*ac.FuelCost*1.05 + 900 + fees
	duration := (dist / ac.CruiseKmh) * 60.0
	if duration < 1 {
		duration = 1
	}
	plan := &models.FlightPlan{
		Origin:     originID,
		Dest:       destID,
		Passengers: sold,
	}
	return duration, plan, revenue, cost
}

func (e *Engine) aircraftFromOwned(ac *models.OwnedCraft) models.Aircraft {
	return models.Aircraft{
		ID:            ac.TemplateID,
		Name:          ac.Name,
		Role:          ac.Role,
		RangeKm:       ac.RangeKm,
		Seats:         ac.Seats,
		CruiseKmh:     ac.CruiseKmh,
		FuelCost:      ac.FuelCost,
		TurnaroundMin: ac.TurnaroundMin,
	}
}

// RecalcUtilization recomputes utilization for each owned aircraft based on assigned routes.
func (e *Engine) RecalcUtilization() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recalcUtilizationLocked()
}

func (e *Engine) recalcUtilizationLocked() {
	scheduled := make(map[string]float64)
	for _, rt := range e.state.Routes {
		mins := rt.BlockMinutes * float64(rt.FrequencyPerDay)
		scheduled[rt.AircraftID] += mins
	}
	countByTemplate := make(map[string]int)
	for _, ac := range e.state.Fleet {
		if ac.Status == "active" {
			countByTemplate[ac.TemplateID]++
		}
	}
	for i := range e.state.Fleet {
		ac := &e.state.Fleet[i]
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

func (e *Engine) advanceFleetTimersLocked() {
	for i := range e.state.Fleet {
		ac := &e.state.Fleet[i]
		if ac.AvailableIn > 0 {
			ac.AvailableIn--
			if ac.AvailableIn <= 0 {
				ac.AvailableIn = 0
				if ac.Status == "delivering" || ac.Status == "maintenance" {
					if ac.Status == "delivering" {
						e.addEventLocked(fmt.Sprintf("%s delivered", ac.Name))
					} else {
						e.addEventLocked(fmt.Sprintf("%s maintenance complete", ac.Name))
					}
					ac.Status = "active"
					if ac.State == models.AircraftGrounded {
						ac.State = models.AircraftIdle
					}
				}
			}
		}
	}
}

func (e *Engine) applyMaintenanceWearLocked() {
	for i := range e.state.Fleet {
		ac := &e.state.Fleet[i]
		if ac.Status != "active" || ac.Condition <= 0 {
			continue
		}
		wear := 0.05 + (ac.Utilization/100.0)*0.4
		ac.Condition -= wear
		if ac.Condition < 0 {
			ac.Condition = 0
		}
		if ac.Condition <= 0 {
			ac.State = models.AircraftGrounded
			ac.Status = "grounded"
			ac.FlightPlan = nil
			e.addEventLocked(fmt.Sprintf("%s grounded — requires maintenance", ac.Name))
		} else if ac.Condition < 50 {
			chance := ((50 - ac.Condition) / 50.0) * 0.25
			if chance > 0 && e.rng.Float64() < chance {
				e.beginMaintenanceLocked(ac, 3+e.rng.Intn(3))
				e.addEventLocked(fmt.Sprintf("%s pulled for maintenance", ac.Name))
			}
		}
	}
}

func (e *Engine) beginMaintenanceLocked(ac *models.OwnedCraft, ticks int) {
	if ticks < 1 {
		ticks = 1
	}
	ac.Status = "maintenance"
	ac.AvailableIn = ticks
}

func maintenanceCost(condition float64) float64 {
	deficit := 100 - condition
	if deficit < 5 {
		deficit = 5
	}
	return deficit * 75_000
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

// utility functions copied from legacy
var runwayReqMeters = map[string]int{
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
var curfewAppliesTo = map[string]bool{
	"large_airport":  true,
	"medium_airport": true,
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

// DemandEstimate is a public wrapper.
func (e *Engine) DemandEstimate(fromAp, toAp models.Airport, ac models.Aircraft, freq int) int {
	return e.demandEstimateWithOpts(fromAp, toAp, ac, freq, demandOptions{})
}

// ValidateCapacity checks slot, curfew, and block-time constraints for a proposed route.
func (e *Engine) ValidateCapacity(route models.Route) error {
	activeCount := 0
	for _, ac := range e.state.Fleet {
		if ac.TemplateID == route.AircraftID && ac.Status == "active" {
			activeCount++
		}
	}
	if activeCount == 0 {
		return http.ErrBodyNotAllowed // no available aircraft of that type
	}

	totalMins := route.BlockMinutes * float64(route.FrequencyPerDay)
	for _, rt := range e.state.Routes {
		if rt.AircraftID == route.AircraftID {
			totalMins += rt.BlockMinutes * float64(rt.FrequencyPerDay)
		}
	}
	if totalMins > float64(activeCount)*960.0 {
		return fmt.Errorf("insufficient aircraft time (over 16h/day for %s fleet)", route.AircraftID)
	}

	addSlotUse := func(ident string, freq int, slotUse map[string]int) {
		if ident == "" || freq == 0 {
			return
		}
		slotUse[strings.ToUpper(ident)] += freq
	}
	slotUse := make(map[string]int)
	addSlotUse(route.From, route.FrequencyPerDay, slotUse)
	addSlotUse(route.To, route.FrequencyPerDay, slotUse)
	addSlotUse(route.Via, route.FrequencyPerDay, slotUse)
	for _, rt := range e.state.Routes {
		addSlotUse(rt.From, rt.FrequencyPerDay, slotUse)
		addSlotUse(rt.To, rt.FrequencyPerDay, slotUse)
		addSlotUse(rt.Via, rt.FrequencyPerDay, slotUse)
	}
	for ident, used := range slotUse {
		if ap, ok := e.byIdent[ident]; ok && ap.SlotsPerDay > 0 && used > ap.SlotsPerDay {
			return fmt.Errorf("slot limit exceeded at %s (%d/%d)", ident, used, ap.SlotsPerDay)
		}
	}

	blockUse := make(map[string]float64)
	addBlockUse := func(ident string, mins float64, freq int, blockUse map[string]float64) {
		if ident == "" || freq == 0 || mins <= 0 {
			return
		}
		blockUse[strings.ToUpper(ident)] += mins * float64(freq)
	}
	addBlockUse(route.From, route.BlockMinutes, route.FrequencyPerDay, blockUse)
	addBlockUse(route.To, route.BlockMinutes, route.FrequencyPerDay, blockUse)
	addBlockUse(route.Via, route.BlockMinutes, route.FrequencyPerDay, blockUse)
	for _, rt := range e.state.Routes {
		addBlockUse(rt.From, rt.BlockMinutes, rt.FrequencyPerDay, blockUse)
		addBlockUse(rt.To, rt.BlockMinutes, rt.FrequencyPerDay, blockUse)
		addBlockUse(rt.Via, rt.BlockMinutes, rt.FrequencyPerDay, blockUse)
	}
	for ident, mins := range blockUse {
		ap, ok := e.byIdent[ident]
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

// MarketExists reports if a route already exists in either direction.
func (e *Engine) MarketExists(from, to string) bool {
	key := marketKey(from, to)
	for _, rt := range e.state.Routes {
		if marketKey(rt.From, rt.To) == key {
			return true
		}
	}
	return false
}

func marketKey(a, b string) string {
	a = strings.ToUpper(a)
	b = strings.ToUpper(b)
	if a < b {
		return a + "-" + b
	}
	return b + "-" + a
}
