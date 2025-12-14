package main

import (
    "encoding/csv"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"

    "github.com/go-chi/chi/v5"
)

type Airport struct {
    ID        string  `json:"id"`
    Ident     string  `json:"ident"`
    Type      string  `json:"type"`
    Name      string  `json:"name"`
    Latitude  float64 `json:"lat"`
    Longitude float64 `json:"lon"`
    Country   string  `json:"country"`
    Region    string  `json:"region"`
    City      string  `json:"city"`
    IATA      string  `json:"iata"`
    ICAO      string  `json:"icao"`
}

type AirportStore struct {
    Airports []Airport
}

func main() {
    store, err := loadAirports("backend/data/airports.csv")
    if err != nil {
        log.Fatalf("failed to load airports: %v", err)
    }

    r := chi.NewRouter()

    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })

    r.Get("/airports", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        tier := r.URL.Query().Get("tier")
        filtered := filterAirports(store.Airports, tier)
        if err := json.NewEncoder(w).Encode(filtered); err != nil {
            http.Error(w, "failed to encode", http.StatusInternalServerError)
        }
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
            ID:        rec[idIdx],
            Ident:     rec[identIdx],
            Type:      t,
            Name:      rec[nameIdx],
            Latitude:  lat,
            Longitude: lon,
            Country:   rec[countryIdx],
            Region:    rec[regionIdx],
            City:      rec[cityIdx],
            IATA:      rec[iataIdx],
            ICAO:      rec[icaoIdx],
        })
    }

    log.Printf("loaded %d airports", len(airports))
    return &AirportStore{Airports: airports}, nil
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
