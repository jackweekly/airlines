class RouteAnalysisResult {
  final String aircraftType;
  final double frequency;
  final double loadFactor;
  final double dailyProfit;
  final double roiScore;
  final bool valid;
  final String? error;

  RouteAnalysisResult({
    required this.aircraftType,
    required this.frequency,
    required this.loadFactor,
    required this.dailyProfit,
    required this.roiScore,
    required this.valid,
    this.error,
  });

  factory RouteAnalysisResult.fromJson(Map<String, dynamic> json) {
    return RouteAnalysisResult(
      aircraftType: json['aircraft_type'] ?? '',
      frequency: (json['frequency'] ?? 0).toDouble(),
      loadFactor: (json['load_factor'] ?? 0).toDouble(),
      dailyProfit: (json['daily_profit'] ?? 0).toDouble(),
      roiScore: (json['roi_score'] ?? 0).toDouble(),
      valid: json['valid'] ?? false,
      error: json['error']?.toString(),
    );
  }
}
