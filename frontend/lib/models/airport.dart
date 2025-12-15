class Airport {
  final String id;
  final String ident;
  final String type;
  final String name;
  final double lat;
  final double lon;
  final String country;
  final String region;
  final String city;
  final String iata;
  final String icao;

  Airport({
    required this.id,
    required this.ident,
    required this.type,
    required this.name,
    required this.lat,
    required this.lon,
    required this.country,
    required this.region,
    required this.city,
    required this.iata,
    required this.icao,
  });

  factory Airport.fromJson(Map<String, dynamic> json) {
    return Airport(
      id: json['id']?.toString() ?? '',
      ident: json['ident']?.toString() ?? '',
      type: json['type']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      lat: (json['lat'] as num?)?.toDouble() ?? 0.0,
      lon: (json['lon'] as num?)?.toDouble() ?? 0.0,
      country: json['country']?.toString() ?? '',
      region: json['region']?.toString() ?? '',
      city: json['city']?.toString() ?? '',
      iata: json['iata']?.toString() ?? '',
      icao: json['icao']?.toString() ?? '',
    );
  }
}
