class RouteInfo {
  RouteInfo({
    required this.id,
    required this.from,
    required this.to,
    required this.via,
    required this.aircraftId,
    required this.freq,
    required this.price,
    required this.userPrice,
    required this.rev,
    required this.cost,
    required this.load,
    required this.lastRev,
    required this.lastLoad,
    required this.profit,
    required this.blockMins,
    required this.landingFees,
    required this.curfewBlocked,
  });

  final String id;
  final String from;
  final String to;
  final String via;
  final String aircraftId;
  final int freq;
  final double price;
  final double userPrice;
  final double rev;
  final double cost;
  final double load;
  final double lastRev;
  final double lastLoad;
  final double profit;
  final double blockMins;
  final double landingFees;
  final bool curfewBlocked;

  factory RouteInfo.fromJson(Map<String, dynamic> json) {
    return RouteInfo(
      id: json['id'] ?? '',
      from: json['from'] ?? '',
      to: json['to'] ?? '',
      via: json['via']?.toString() ?? '',
      aircraftId: json['aircraft_id'] ?? '',
      freq: json['frequency_per_day'] ?? 0,
      price: (json['price_per_seat'] ?? 0).toDouble(),
      userPrice: (json['user_price'] ?? 0).toDouble(),
      rev: (json['estimated_revenue_tick'] ?? 0).toDouble(),
      cost: (json['estimated_cost_tick'] ?? 0).toDouble(),
      load: (json['load_factor'] ?? 0).toDouble(),
      lastRev: (json['last_tick_revenue'] ?? 0).toDouble(),
      lastLoad: (json['last_tick_load'] ?? 0).toDouble(),
      profit: (json['profit_per_tick'] ?? 0).toDouble(),
      blockMins: (json['block_minutes'] ?? 0).toDouble(),
      landingFees: (json['landing_fees_per_leg'] ?? 0).toDouble(),
      curfewBlocked: json['curfew_blocked'] ?? false,
    );
  }
}
