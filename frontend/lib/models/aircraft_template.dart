class AircraftTemplate {
  AircraftTemplate({
    required this.id,
    required this.name,
    required this.rangeKm,
    required this.seats,
    required this.cruiseKmh,
    required this.fuelCostPerKm,
    required this.turnaroundMin,
  });

  final String id;
  final String name;
  final double rangeKm;
  final int seats;
  final double cruiseKmh;
  final double fuelCostPerKm;
  final int turnaroundMin;

  factory AircraftTemplate.fromJson(Map<String, dynamic> json) {
    return AircraftTemplate(
      id: json['id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      rangeKm: (json['range_km'] ?? 0).toDouble(),
      seats: json['seats'] ?? 0,
      cruiseKmh: (json['cruise_kmh'] ?? 0).toDouble(),
      fuelCostPerKm: (json['fuel_cost_per_km'] ?? 0).toDouble(),
      turnaroundMin: json['turnaround_min'] ?? 0,
    );
  }
}
