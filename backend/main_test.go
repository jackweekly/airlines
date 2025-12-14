package main

import (
	"testing"
)

// set up minimal airport data and fleet for tests
func setupTestAirports() {
	store = &AirportStore{Airports: []Airport{
		{ID: "1", Ident: "AAA", Type: "large_airport", Name: "A", Latitude: 0, Longitude: 0, RunwayM: 3200, SlotsPerDay: 200, LandingFee: 1000},
		{ID: "2", Ident: "BBB", Type: "large_airport", Name: "B", Latitude: 0, Longitude: 10, RunwayM: 3200, SlotsPerDay: 200, LandingFee: 1000},
	}}
	airportsByIdent = map[string]Airport{
		"AAA": store.Airports[0],
		"BBB": store.Airports[1],
	}
	aircraftCatalog = []Aircraft{
		{
			ID:            "A320",
			Name:          "Airbus A320",
			Role:          "passenger",
			RangeKm:       6100,
			Seats:         180,
			CruiseKmh:     830,
			FuelCost:      4.2,
			TurnaroundMin: 45,
		},
	}
}

func TestBuildRouteEconomicsIncludesFeesAndProfit(t *testing.T) {
	setupTestAirports()
	state = GameState{Fleet: seedFleet()}

	rt, err := buildRoute("AAA", "BBB", "", "A320", 2, 0)
	if err != nil {
		t.Fatalf("buildRoute returned error: %v", err)
	}
	if rt.LandingFeesPerLeg <= 0 {
		t.Fatalf("expected landing fees >0, got %.2f", rt.LandingFeesPerLeg)
	}
	if rt.CostPerLeg < rt.LandingFeesPerLeg {
		t.Fatalf("cost per leg %.2f should include fees %.2f", rt.CostPerLeg, rt.LandingFeesPerLeg)
	}
	if rt.ProfitPerTick <= 0 {
		t.Fatalf("expected profit per tick to be positive, got %.2f", rt.ProfitPerTick)
	}
}

func TestValidateCapacityAndSlots(t *testing.T) {
	setupTestAirports()
	state = GameState{
		Fleet: []OwnedCraft{
			{TemplateID: "A320", Status: "active"},
		},
		Routes: []Route{
			{AircraftID: "A320", BlockMinutes: 800, FrequencyPerDay: 1, From: "AAA", To: "BBB"},
			{AircraftID: "A320", BlockMinutes: 100, FrequencyPerDay: 1, From: "AAA", To: "BBB"},
		},
	}
	route := Route{AircraftID: "A320", BlockMinutes: 900, FrequencyPerDay: 1, From: "AAA", To: "BBB"}
	if err := validateCapacityLocked(route); err == nil {
		t.Fatalf("expected over-capacity error, got nil")
	}

	// Now check slot limit using tight slots
	store.Airports[0].SlotsPerDay = 2
	store.Airports[1].SlotsPerDay = 2
	airportsByIdent["AAA"] = store.Airports[0]
	airportsByIdent["BBB"] = store.Airports[1]
	state.Routes = []Route{
		{AircraftID: "A320", BlockMinutes: 200, FrequencyPerDay: 2, From: "AAA", To: "BBB"},
	}
	route = Route{AircraftID: "A320", BlockMinutes: 200, FrequencyPerDay: 1, From: "AAA", To: "BBB"}
	if err := validateCapacityLocked(route); err == nil {
		t.Fatalf("expected slot limit error, got nil")
	}
}

func TestMarketUniqueness(t *testing.T) {
	setupTestAirports()
	state = GameState{
		Fleet: []OwnedCraft{{TemplateID: "A320", Status: "active"}},
		Routes: []Route{
			{From: "AAA", To: "BBB", AircraftID: "A320", BlockMinutes: 100, FrequencyPerDay: 1},
		},
	}
	if !marketExistsLocked("AAA", "BBB") {
		t.Fatalf("expected market to exist")
	}
	if !marketExistsLocked("BBB", "AAA") {
		t.Fatalf("expected market to exist for reverse")
	}
}
