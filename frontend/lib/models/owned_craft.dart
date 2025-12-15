class OwnedCraft {
  OwnedCraft({
    required this.id,
    required this.templateId,
    required this.name,
    required this.rangeKm,
    required this.seats,
    required this.cruiseKmh,
    required this.fuelCost,
    required this.turnaround,
    required this.status,
    required this.availableIn,
    required this.util,
    required this.condition,
    required this.ownershipType,
    required this.monthlyCost,
  });

  final String id;
  final String templateId;
  final String name;
  final double rangeKm;
  final int seats;
  final double cruiseKmh;
  final double fuelCost;
  final int turnaround;
  final String status;
  final int availableIn;
  final double util;
  final double condition;
  final String ownershipType;
  final double monthlyCost;

  factory OwnedCraft.fromJson(Map<String, dynamic> json) {
    return OwnedCraft(
      id: json['id']?.toString() ?? '',
      templateId: json['template_id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      rangeKm: (json['range_km'] ?? 0).toDouble(),
      seats: json['seats'] ?? 0,
      cruiseKmh: (json['cruise_kmh'] ?? 0).toDouble(),
      fuelCost: (json['fuel_cost_per_km'] ?? 0).toDouble(),
      turnaround: json['turnaround_min'] ?? 0,
      status: json['status']?.toString() ?? '',
      availableIn: json['available_in_ticks'] ?? 0,
      util: (json['utilization_pct'] ?? 0).toDouble(),
      condition: (json['condition_pct'] ?? 0).toDouble(),
      ownershipType: json['ownership_type']?.toString() ?? 'owned',
      monthlyCost: (json['monthly_cost'] ?? 0).toDouble(),
    );
  }
}
