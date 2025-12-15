package models

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
	UserPrice         float64 `json:"user_price"`
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

type AircraftState string

const (
	AircraftIdle       AircraftState = "idle"
	AircraftFlying     AircraftState = "flying"
	AircraftTurnaround AircraftState = "turnaround"
)

type FlightPlan struct {
	Origin     string `json:"origin"`
	Dest       string `json:"dest"`
	Passengers int    `json:"passengers"`
}

type OwnedCraft struct {
	ID            string        `json:"id"`
	TemplateID    string        `json:"template_id"`
	Name          string        `json:"name"`
	Role          string        `json:"role"`
	RangeKm       float64       `json:"range_km"`
	Seats         int           `json:"seats"`
	CruiseKmh     float64       `json:"cruise_kmh"`
	FuelCost      float64       `json:"fuel_cost_per_km"`
	TurnaroundMin int           `json:"turnaround_min"`
	Crew          int           `json:"crew,omitempty"`
	CargoVolumeM3 float64       `json:"cargo_volume_m3,omitempty"`
	MaxPayloadKg  float64       `json:"max_payload_kg,omitempty"`
	Status        string        `json:"status"`
	AvailableIn   int           `json:"available_in_ticks"`
	Utilization   float64       `json:"utilization_pct"`
	Condition     float64       `json:"condition_pct"`
	OwnershipType string        `json:"ownership_type,omitempty"`
	MonthlyCost   float64       `json:"monthly_cost,omitempty"`
	State         AircraftState `json:"state"`
	Location      string        `json:"location"`
	TimerMin      int           `json:"timer_min"`
	FlightPlan    *FlightPlan   `json:"flight_plan,omitempty"`
	RouteLegIndex int           `json:"route_leg_index,omitempty"`
}
