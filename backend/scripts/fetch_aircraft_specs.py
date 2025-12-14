#!/usr/bin/env python3
"""
Fetch aircraft performance data from Wikipedia and emit backend/data/aircraft.json.

The source for aircraft type selection is:
https://en.wikipedia.org/wiki/List_of_commercial_jet_airliners

For each target we read the linked article's specification table (or,
when absent, the performance list) to extract range, cruise speed and seating.
Fuel cost and turnaround are derived from gameplay balance rules.
"""

from __future__ import annotations

import json
import math
import os
import re
from dataclasses import dataclass
from io import StringIO
from typing import Any, Callable, Dict, Iterable, List, Optional, Sequence, Tuple

import pandas as pd
import requests
from bs4 import BeautifulSoup

DATA_PATH = os.path.join(os.path.dirname(os.path.dirname(__file__)), "data", "aircraft.json")
HEADERS = {"User-Agent": "AirlineBuilderBot/1.0 (https://github.com/jackweekly/airline-builder)"}

TURNAROUND_BY_CLASS = {
    "regional": 25,
    "narrowbody": 45,
    "widebody": 75,
    "jumbo": 100,
}

FUEL_PROFILES = {
    "regional_turboprop": ("seats", 0.018),
    "regional_jet": ("seats", 0.024),
    "regional_jet_modern": ("seats", 0.022),
    "narrowbody_classic": ("seats", 0.022),
    "narrowbody_modern": ("seats", 0.019),
    "widebody_classic": ("seats", 0.019),
    "widebody_modern": ("seats", 0.017),
    "jumbo_legacy": ("seats", 0.024),
    "jumbo_modern": ("seats", 0.021),
    "cargo_widebody": ("payload", 0.000055),
    "cargo_jumbo": ("payload", 0.00005),
}


@dataclass
class AircraftSpec:
    id: str
    display_name: str
    wiki_page: str
    aircraft_class: str
    fuel_profile: str
    order: int
    spec_type: str  # "table" or "list"
    role: str = "passenger"
    variant_column: Optional[str] = None
    seat_keywords: Sequence[str] = ("seating", "3-class", "2-class", "passenger", "capacity", "max")
    range_keywords: Sequence[str] = ("range",)
    cruise_keywords: Sequence[str] = ("cruise", "cruising", "speed")
    default_seats: Optional[int] = None


@dataclass
class FieldDef:
    key: str
    keywords: Tuple[str, ...]
    parser: Callable[[str], Optional[object]]
    skip: Tuple[str, ...] = ()


PLANES: List[AircraftSpec] = [
    AircraftSpec("ATR72", "ATR 72-600", "ATR_72", "regional", "regional_turboprop", 1, "table", variant_column="ATR 72-600"),
    AircraftSpec("CRJ9", "Bombardier CRJ900", "Bombardier_CRJ700_series", "regional", "regional_jet", 2, "table", variant_column="CRJ900"),
    AircraftSpec("E175", "Embraer E175", "Embraer_175", "regional", "regional_jet", 3, "table", variant_column="E175"),
    AircraftSpec("E190", "Embraer E190", "Embraer_190", "regional", "regional_jet", 4, "table", variant_column="E190"),
    AircraftSpec("E195E2", "Embraer E195-E2", "Embraer_E-Jet_E2_family", "regional", "regional_jet_modern", 5, "table", variant_column="E195-E2"),
    AircraftSpec("B737-700", "Boeing 737-700", "Boeing_737-700", "narrowbody", "narrowbody_classic", 10, "list"),
    AircraftSpec("B737-800", "Boeing 737-800", "Boeing_737-800", "narrowbody", "narrowbody_classic", 11, "list"),
    AircraftSpec("B737MAX8", "Boeing 737 MAX 8", "Boeing_737_MAX", "narrowbody", "narrowbody_modern", 12, "table", variant_column="737 MAX 8"),
    AircraftSpec("A320", "Airbus A320ceo", "Airbus_A320", "narrowbody", "narrowbody_classic", 13, "table", variant_column="A320"),
    AircraftSpec("A320NEO", "Airbus A320neo", "Airbus_A320neo_family", "narrowbody", "narrowbody_modern", 14, "table", variant_column="A320neo"),
    AircraftSpec("A321NEO", "Airbus A321neo", "Airbus_A321neo", "narrowbody", "narrowbody_modern", 15, "table", variant_column="A321neo"),
    AircraftSpec("B767-300ER", "Boeing 767-300ER", "Boeing_767", "widebody", "widebody_classic", 20, "table", variant_column="767-300ER"),
    AircraftSpec("B777-300ER", "Boeing 777-300ER", "Boeing_777", "widebody", "widebody_classic", 21, "table", variant_column="777-300ER"),
    AircraftSpec("B787-9", "Boeing 787-9", "Boeing_787_Dreamliner", "widebody", "widebody_modern", 22, "table", variant_column="787-9"),
    AircraftSpec("A330-900", "Airbus A330-300", "Airbus_A330", "widebody", "widebody_modern", 23, "table", variant_column="A330-300"),
    AircraftSpec("A350-900", "Airbus A350-900", "Airbus_A350", "widebody", "widebody_modern", 24, "list", seat_keywords=("seat", "passenger", "capacity")),
    AircraftSpec("B767-300F", "Boeing 767-300F", "Boeing_767", "widebody", "cargo_widebody", 26, "table", role="cargo", default_seats=0, variant_column="767-300ER/F", seat_keywords=()),
    AircraftSpec("B777F", "Boeing 777F", "Boeing_777F", "widebody", "cargo_widebody", 27, "table", role="cargo", default_seats=0, seat_keywords=(), variant_column="777-200LR/777F"),
    AircraftSpec("A330-200F", "Airbus A330-200F", "Airbus_A330-200F", "widebody", "cargo_widebody", 28, "table", role="cargo", default_seats=0, seat_keywords=()),
    AircraftSpec("B747-8F", "Boeing 747-8F", "Boeing_747-8", "jumbo", "cargo_jumbo", 29, "table", role="cargo", default_seats=0, variant_column="747-8F", seat_keywords=()),
    AircraftSpec("B747-400", "Boeing 747-400", "Boeing_747-400", "jumbo", "jumbo_legacy", 30, "table", variant_column="747-400"),
    AircraftSpec("A380-800", "Airbus A380-800", "Airbus_A380", "jumbo", "jumbo_modern", 31, "table", variant_column="A380-841"),
]


SESSION = requests.Session()
SESSION.headers.update(HEADERS)


def fetch_html(page: str) -> str:
    url = f"https://en.wikipedia.org/wiki/{page}"
    resp = SESSION.get(url, timeout=20)
    resp.raise_for_status()
    return resp.text


def clean_text(value: str) -> str:
    value = re.sub(r"\[[^\]]*\]", "", value)
    value = value.replace("\xa0", " ")
    return value.strip()


def parse_number(text: str) -> Optional[float]:
    text = clean_text(text)
    match = re.search(r"([0-9][0-9,]*)", text)
    if not match:
        return None
    return float(match.group(1).replace(",", ""))


def parse_int_value(text: str) -> Optional[int]:
    num = parse_number(text)
    if num is None:
        return None
    return int(num)


def parse_speed(text: str) -> Optional[float]:
    text = clean_text(text)
    kmh = re.search(r"([0-9][0-9,]*)\s*km/?h", text, re.IGNORECASE)
    if kmh:
        return float(kmh.group(1).replace(",", ""))
    mph = re.search(r"([0-9][0-9,]*)\s*mph", text, re.IGNORECASE)
    if mph:
        return float(mph.group(1).replace(",", "")) * 1.60934
    mach = re.search(r"Mach\s*([0-9.]+)", text)
    if mach:
        return float(mach.group(1)) * 1225.0
    return None


def parse_distance(text: str) -> Optional[float]:
    text = clean_text(text)
    km = re.search(r"([0-9][0-9,]*)\s*km", text)
    if km:
        return float(km.group(1).replace(",", ""))
    nmi = re.search(r"([0-9][0-9,]*)\s*nmi", text, re.IGNORECASE)
    if nmi:
        return float(nmi.group(1).replace(",", "")) * 1.852
    mi = re.search(r"([0-9][0-9,]*)\s*mi", text)
    if mi:
        return float(mi.group(1).replace(",", "")) * 1.60934
    return None


def parse_length(text: str) -> Optional[float]:
    text = clean_text(text)
    m_match = re.search(r"([0-9][0-9,\.]*)\s*m(?![0-9])", text)
    if m_match:
        return float(m_match.group(1).replace(",", ""))
    ft = re.search(r"([0-9][0-9,\.]*)\s*ft", text)
    if ft:
        return float(ft.group(1).replace(",", "")) * 0.3048
    return None


def parse_area(text: str) -> Optional[float]:
    text = clean_text(text)
    m2 = re.search(r"([0-9][0-9,\.]*)\s*m\^?2", text)
    if m2:
        return float(m2.group(1).replace(",", ""))
    sqft = re.search(r"([0-9][0-9,\.]*)\s*sq\s*ft", text, re.IGNORECASE)
    if sqft:
        return float(sqft.group(1).replace(",", "")) * 0.092903
    return None


def parse_mass(text: str) -> Optional[float]:
    text = clean_text(text)
    kg = re.search(r"([0-9][0-9,\.]*)\s*kg", text)
    if kg:
        return float(kg.group(1).replace(",", ""))
    tonne = re.search(r"([0-9][0-9,\.]*)\s*t", text)
    if tonne:
        return float(tonne.group(1).replace(",", "")) * 1000.0
    lb = re.search(r"([0-9][0-9,\.]*)\s*lb", text)
    if lb:
        return float(lb.group(1).replace(",", "")) * 0.453592
    return None


def parse_volume(text: str) -> Optional[float]:
    text = clean_text(text)
    m3 = re.search(r"([0-9][0-9,\.]*)\s*m\^?3", text)
    if m3:
        return float(m3.group(1).replace(",", ""))
    cuft = re.search(r"([0-9][0-9,\.]*)\s*(?:cu|cubic)\s*ft", text, re.IGNORECASE)
    if cuft:
        return float(cuft.group(1).replace(",", "")) * 0.0283168
    return None


def parse_fuel_volume(text: str) -> Optional[float]:
    text = clean_text(text)
    liters = re.search(r"([0-9][0-9,\.]*)\s*L", text)
    if liters:
        return float(liters.group(1).replace(",", ""))
    gal = re.search(r"([0-9][0-9,\.]*)\s*(?:US\s*)?gal", text)
    if gal:
        return float(gal.group(1).replace(",", "")) * 3.78541
    return None


def parse_force(text: str) -> Optional[float]:
    text = clean_text(text)
    kn = re.search(r"([0-9][0-9,\.]*)\s*kN", text)
    if kn:
        return float(kn.group(1).replace(",", ""))
    lbf = re.search(r"([0-9][0-9,\.]*)\s*lbf", text, re.IGNORECASE)
    if lbf:
        return float(lbf.group(1).replace(",", "")) * 0.00444822
    return None


FIELD_DEFS: List[FieldDef] = [
    FieldDef("crew", ("crew",), parse_int_value),
    FieldDef("three_class_seats", ("3-class seats", "three-class"), parse_int_value),
    FieldDef("two_class_seats", ("2-class seats", "two-class"), parse_int_value),
    FieldDef("one_class_max_seats", ("1-class", "one-class", "max seating", "maximum seating"), parse_int_value),
    FieldDef("cargo_volume_m3", ("cargo volume", "cargo capacity", "hold"), parse_volume),
    FieldDef("mtow_kg", ("mtow", "max takeoff weight"), parse_mass),
    FieldDef("max_payload_kg", ("max payload", "payload"), parse_mass),
    FieldDef("oew_kg", ("oew", "operating empty weight"), parse_mass),
    FieldDef("fuel_capacity_l", ("fuel capacity", "max fuel"), parse_fuel_volume),
    FieldDef("length_m", ("length",), parse_length),
    FieldDef("wingspan_m", ("wingspan", "span"), parse_length),
    FieldDef("height_m", ("height",), parse_length, skip=("width",)),
    FieldDef("wing_area_m2", ("wing area",), parse_area),
    FieldDef("engine_type", ("engines", "engine"), clean_text),
    FieldDef("engine_thrust_kn", ("thrust",), parse_force),
    FieldDef("service_ceiling_m", ("ceiling", "service ceiling"), parse_length),
    FieldDef("max_speed_kmh", ("maximum speed", "high speed cruise"), parse_speed),
    FieldDef("takeoff_distance_m", ("takeoff",), parse_length),
    FieldDef("landing_distance_m", ("landing",), parse_length),
    FieldDef("icao_type", ("icao", "icao type"), clean_text),
]


def collect_table_fields(df: pd.DataFrame, column: str) -> Dict[str, Any]:
    extras: Dict[str, Any] = {}
    for field in FIELD_DEFS:
        try:
            raw = find_row_value(df, column, field.keywords, skip_terms=field.skip)
        except RuntimeError:
            continue
        value = field.parser(raw)
        if value is None:
            continue
        if isinstance(value, (int, float)) and value == 0:
            continue
        if isinstance(value, str) and not value.strip():
            continue
        extras[field.key] = value
    return extras


def collect_list_fields(items: Sequence[Tuple[str, str]]) -> Dict[str, Any]:
    extras: Dict[str, Any] = {}
    for field in FIELD_DEFS:
        raw = find_in_list(items, field.keywords, field.skip)
        if raw is None:
            continue
        value = field.parser(raw)
        if value is None:
            continue
        if isinstance(value, (int, float)) and value == 0:
            continue
        if isinstance(value, str) and not value.strip():
            continue
        extras[field.key] = value
    return extras


def find_in_list(items: Sequence[Tuple[str, str]], keywords: Sequence[str], skip_terms: Sequence[str]) -> Optional[str]:
    for key in keywords:
        low_key = key.lower()
        for label, text in items:
            if any(skip in label for skip in skip_terms):
                continue
            if low_key in label:
                return text
    return None


def extract_table_spec(plane: AircraftSpec, html: str) -> Tuple[int, float, float, Dict[str, Any]]:
    tables = pd.read_html(StringIO(html))
    for raw_df in tables:
        if raw_df.shape[1] < 2:
            continue
        df = raw_df.copy()
        df.columns = [clean_text(str(c)) for c in df.columns]
        first_col = df.columns[0]
        first_values = df[first_col].astype(str).map(clean_text)
        df = df.drop(columns=[first_col])
        df.insert(0, first_col, first_values)
        df = df.set_index(first_col)

        if plane.default_seats is None and not has_index_keyword(df.index, plane.seat_keywords):
            continue
        if not has_index_keyword(df.index, plane.range_keywords):
            continue

        column = plane.variant_column
        if column:
            matched = None
            for col in df.columns:
                if column.lower() in col.lower():
                    matched = col
                    break
            if matched is None:
                continue
            column = matched
        else:
            column = df.columns[-1]

        extras = collect_table_fields(df, column)

        seat_val = None
        if plane.seat_keywords:
            try:
                seat_val = find_row_value(df, column, plane.seat_keywords, skip_terms=("cargo", "range"))
            except RuntimeError:
                if plane.default_seats is None:
                    continue
        elif plane.default_seats is None:
            continue
        try:
            range_val = find_row_value(df, column, plane.range_keywords)
            cruise_val = find_row_value(df, column, plane.cruise_keywords)
        except RuntimeError:
            continue

        seats = int(parse_number(seat_val) or 0) if seat_val else 0
        if seats == 0 and plane.default_seats is not None:
            seats = plane.default_seats
        range_km = parse_distance(range_val) or 0.0
        cruise = parse_speed(cruise_val) or 0.0
        if seats == 0 and plane.role != "cargo":
            continue
        if range_km == 0.0 or cruise == 0.0:
            continue
        return seats, range_km, cruise, extras

    raise RuntimeError(f"{plane.id}: specification table not found")


def has_index_keyword(index: pd.Index, keywords: Sequence[str]) -> bool:
    for idx in index:
        if not isinstance(idx, str):
            continue
        lowered = idx.lower()
        for key in keywords:
            if key.lower() in lowered:
                return True
    return False


def find_row_value(df: pd.DataFrame, column: str, keywords: Sequence[str], skip_terms: Sequence[str] | None = None) -> str:
    skip_terms = tuple(skip_terms or ())
    for key in keywords:
        low_key = key.lower()
        for idx in df.index:
            if isinstance(idx, str) and low_key in idx.lower():
                if any(skip in idx.lower() for skip in skip_terms):
                    continue
                value = str(df.at[idx, column])
                if value and value != "nan":
                    return value
    raise RuntimeError(f"Row '{keywords}' not found")


def extract_list_spec(plane: AircraftSpec, soup: BeautifulSoup) -> Tuple[int, float, float, Dict[str, Any]]:
    items = build_list_items(soup)
    seats = extract_li_value(items, plane.seat_keywords, parse_number, skip_terms=("width", "pitch", "cargo", "range"))
    range_km = extract_li_value(items, plane.range_keywords, parse_distance)
    cruise = extract_li_value(items, plane.cruise_keywords, parse_speed)
    if seats is None:
        if plane.default_seats is None:
            raise RuntimeError(f"{plane.id}: missing seats")
        seats = plane.default_seats
    if range_km is None or cruise is None:
        raise RuntimeError(f"{plane.id}: missing data from bullet list")
    extras = collect_list_fields(items)
    return int(seats), range_km, cruise, extras


def build_list_items(soup: BeautifulSoup) -> List[Tuple[str, str]]:
    items: List[Tuple[str, str]] = []
    for li in soup.find_all("li"):
        bold = li.find("b")
        if not bold:
            continue
        label = clean_text(bold.get_text(" ", strip=True)).lower()
        text = li.get_text(" ", strip=True)
        items.append((label, text))
    return items


def extract_li_value(items: Sequence[Tuple[str, str]], keywords: Sequence[str], parser, skip_terms: Sequence[str] | None = None):
    skip_terms = tuple(skip_terms or ())
    for key in keywords:
        low_key = key.lower()
        for label, text in items:
            if any(skip in label for skip in skip_terms):
                continue
            if low_key in label:
                value = parser(text)
                if value:
                    return value
    return None


def compute_fuel_cost(plane: AircraftSpec, data: Dict[str, Any]) -> float:
    profile_type, factor = FUEL_PROFILES[plane.fuel_profile]
    base = 0.0
    if profile_type == "seats":
        base = float(data.get("seats", 0) or 0)
    elif profile_type == "payload":
        payload = data.get("max_payload_kg") or data.get("mtow_kg")
        if payload:
            base = float(payload)
    if base <= 0 and data.get("seats"):
        base = float(data["seats"])
    if base <= 0:
        return 0.0
    return round(base * factor, 2)


def build_entry(plane: AircraftSpec) -> Dict[str, object]:
    html = fetch_html(plane.wiki_page)
    soup = BeautifulSoup(html, "html.parser")
    if plane.spec_type == "table":
        seats, range_km, cruise, extras = extract_table_spec(plane, html)
    else:
        seats, range_km, cruise, extras = extract_list_spec(plane, soup)
    entry = {
        "id": plane.id,
        "name": plane.display_name,
        "range_km": round(range_km, 1),
        "seats": seats,
        "cruise_kmh": round(cruise, 1),
        "role": plane.role,
        "turnaround_min": TURNAROUND_BY_CLASS[plane.aircraft_class],
    }
    for key, value in extras.items():
        entry[key] = value
    if plane.role == "cargo":
        for seat_key in ("three_class_seats", "two_class_seats", "one_class_max_seats"):
            entry.pop(seat_key, None)
        entry["seats"] = 0
    entry["fuel_cost_per_km"] = compute_fuel_cost(plane, entry)
    return entry


def main() -> None:
    entries: List[Dict[str, object]] = []
    for plane in sorted(PLANES, key=lambda p: p.order):
        print(f"Fetching {plane.id} from {plane.wiki_page}")
        entry = build_entry(plane)
        entries.append(entry)

    with open(DATA_PATH, "w", encoding="utf-8") as fh:
        json.dump(entries, fh, indent=2)
        fh.write("\n")
    print(f"Wrote {len(entries)} aircraft to {DATA_PATH}")


if __name__ == "__main__":
    main()
